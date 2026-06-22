// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &RunRuleLogsDataSource{}

// NewRunRuleLogsDataSource returns a new RunRuleLogsDataSource.
func NewRunRuleLogsDataSource() datasource.DataSource {
	return &RunRuleLogsDataSource{}
}

// RunRuleLogsDataSource surfaces the execution history of a LangSmith
// automation (run) rule: the most recently applied log entry and the full list
// of recorded executions.
type RunRuleLogsDataSource struct {
	client *client.Client
}

// RunRuleLogsDataSourceModel is the Terraform state for the data source.
type RunRuleLogsDataSourceModel struct {
	RuleID      types.String `tfsdk:"rule_id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	LastApplied types.String `tfsdk:"last_applied"`
	Logs        types.List   `tfsdk:"logs"`
}

// runRuleLogsAPIEntry mirrors the RuleLogSchema component returned by both the
// last_applied and logs endpoints. The action sub-objects
// (add_to_annotation_queue, add_to_dataset, evaluators, alerts, webhooks,
// extend_only) are polymorphic, so they are preserved as raw JSON in raw_json
// rather than flattened into typed attributes.
type runRuleLogsAPIEntry struct {
	RuleID          string  `json:"rule_id"`
	RunID           string  `json:"run_id"`
	RunName         *string `json:"run_name"`
	RunType         *string `json:"run_type"`
	RunSessionID    *string `json:"run_session_id"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	ApplicationTime *string `json:"application_time"`
	ThreadID        *string `json:"thread_id"`
}

// runRuleLogEntryObjectType is the Terraform object type for a single log entry.
var runRuleLogEntryObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"rule_id":          types.StringType,
	"run_id":           types.StringType,
	"run_name":         types.StringType,
	"run_type":         types.StringType,
	"run_session_id":   types.StringType,
	"start_time":       types.StringType,
	"end_time":         types.StringType,
	"application_time": types.StringType,
	"thread_id":        types.StringType,
	"raw_json":         types.StringType,
}}

func (d *RunRuleLogsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run_rule_logs"
}

func (d *RunRuleLogsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the execution history of a LangSmith automation (run) rule: `last_applied` is the most recently applied log entry (JSON-encoded, may be null) and `logs` is the list of recorded executions.",
		Attributes: map[string]schema.Attribute{
			"rule_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the run rule whose logs to read.",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"last_applied": schema.StringAttribute{
				MarkdownDescription: "The most recently applied log entry for the rule, JSON-encoded. Null when the rule has never been applied.",
				Computed:            true,
			},
			"logs": schema.ListNestedAttribute{
				MarkdownDescription: "The list of recorded rule executions, most recent first.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"rule_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "The UUID of the rule this log entry belongs to."},
						"run_id":           schema.StringAttribute{Computed: true, MarkdownDescription: "The UUID of the run the rule was applied to."},
						"run_name":         schema.StringAttribute{Computed: true, MarkdownDescription: "The name of the run, if available."},
						"run_type":         schema.StringAttribute{Computed: true, MarkdownDescription: "The type of the run, if available."},
						"run_session_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "The UUID of the run's session/project, if available."},
						"start_time":       schema.StringAttribute{Computed: true, MarkdownDescription: "The start time of the run."},
						"end_time":         schema.StringAttribute{Computed: true, MarkdownDescription: "The end time of the run."},
						"application_time": schema.StringAttribute{Computed: true, MarkdownDescription: "The time the rule was applied to the run, if available."},
						"thread_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "The thread ID associated with the run, if available."},
						"raw_json":         schema.StringAttribute{Computed: true, MarkdownDescription: "The complete log entry as returned by the API, JSON-encoded. Includes the polymorphic per-action outcomes (add_to_annotation_queue, add_to_dataset, evaluators, alerts, webhooks, extend_only)."},
					},
				},
			},
		},
	}
}

func (d *RunRuleLogsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *RunRuleLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RunRuleLogsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	effClient := effectiveClient(d.client, data.WorkspaceID)
	ruleID := data.RuleID.ValueString()

	// last_applied: a single RuleLogSchema record, or empty when never applied.
	var lastRaw json.RawMessage
	if err := effClient.Get(ctx, fmt.Sprintf("/api/v1/runs/rules/%s/last_applied", ruleID), nil, &lastRaw); err != nil {
		// A rule that has never been applied returns 404 here; treat that as
		// "no last-applied entry" rather than an error.
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error reading run rule last applied log", err.Error())
			return
		}
		data.LastApplied = types.StringNull()
	} else if len(lastRaw) == 0 || string(lastRaw) == "null" {
		data.LastApplied = types.StringNull()
	} else {
		data.LastApplied = jsonStringValue(lastRaw)
	}

	// logs: an array of RuleLogSchema records.
	var logsRaw []json.RawMessage
	if err := effClient.Get(ctx, fmt.Sprintf("/api/v1/runs/rules/%s/logs", ruleID), nil, &logsRaw); err != nil {
		// A rule with no recorded executions may 404; treat as an empty list.
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error reading run rule logs", err.Error())
			return
		}
		logsRaw = nil
	}

	stringOrNull := func(s *string) types.String {
		if s != nil {
			return types.StringValue(*s)
		}
		return types.StringNull()
	}

	elems := make([]attr.Value, 0, len(logsRaw))
	for _, raw := range logsRaw {
		var entry runRuleLogsAPIEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			resp.Diagnostics.AddError("Error decoding run rule log entry", err.Error())
			return
		}
		obj, diags := types.ObjectValue(runRuleLogEntryObjectType.AttrTypes, map[string]attr.Value{
			"rule_id":          types.StringValue(entry.RuleID),
			"run_id":           types.StringValue(entry.RunID),
			"run_name":         stringOrNull(entry.RunName),
			"run_type":         stringOrNull(entry.RunType),
			"run_session_id":   stringOrNull(entry.RunSessionID),
			"start_time":       types.StringValue(entry.StartTime),
			"end_time":         types.StringValue(entry.EndTime),
			"application_time": stringOrNull(entry.ApplicationTime),
			"thread_id":        stringOrNull(entry.ThreadID),
			"raw_json":         jsonStringValue(raw),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(runRuleLogEntryObjectType, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Logs = list

	// The logs endpoints do not echo a workspace/tenant id; fall back to the
	// client's workspace so workspace_id is never left unknown.
	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read run rule logs data source", map[string]interface{}{"rule_id": ruleID, "count": len(logsRaw)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

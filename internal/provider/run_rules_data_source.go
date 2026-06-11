// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &RunRulesDataSource{}

func NewRunRulesDataSource() datasource.DataSource {
	return &RunRulesDataSource{}
}

type RunRulesDataSource struct {
	client *client.Client
}

type RunRulesDataSourceModel struct {
	SessionID    types.String `tfsdk:"session_id"`
	DatasetID    types.String `tfsdk:"dataset_id"`
	Type         types.String `tfsdk:"type"`
	NameContains types.String `tfsdk:"name_contains"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Rules        types.List   `tfsdk:"rules"`
}

var runRuleObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":            types.StringType,
	"display_name":  types.StringType,
	"session_id":    types.StringType,
	"dataset_id":    types.StringType,
	"is_enabled":    types.BoolType,
	"sampling_rate": types.Float64Type,
	"filter":        types.StringType,
	"trace_filter":  types.StringType,
	"tree_filter":   types.StringType,
	"backfill_from": types.StringType,
	"evaluators":    types.StringType,
	"alerts":        types.StringType,
	"webhooks":      types.StringType,
	"created_at":    types.StringType,
	"updated_at":    types.StringType,
}}

// runRulesListItemAPI mirrors a single rule in GET /api/v1/runs/rules
// responses (component schema RunRulesSchema), reusing the field shapes of the
// singular run rule data source plus dataset_id.
type runRulesListItemAPI struct {
	runRuleDataSourceAPIResponse
	DatasetID *string `json:"dataset_id"`
}

func (d *RunRulesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run_rules"
}

func (d *RunRulesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith automation (run) rules, with optional filters.",
		Attributes: map[string]schema.Attribute{
			"session_id": schema.StringAttribute{
				MarkdownDescription: "Filter to rules attached to this project/session ID.",
				Optional:            true,
			},
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "Filter to rules attached to this dataset ID.",
				Optional:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Filter by rule type. Valid values: `session`, `dataset`.",
				Optional:            true,
			},
			"name_contains": schema.StringAttribute{
				MarkdownDescription: "Filter to rules whose display name contains this substring.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"rules": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Computed: true},
						"display_name":  schema.StringAttribute{Computed: true},
						"session_id":    schema.StringAttribute{Computed: true},
						"dataset_id":    schema.StringAttribute{Computed: true},
						"is_enabled":    schema.BoolAttribute{Computed: true},
						"sampling_rate": schema.Float64Attribute{Computed: true},
						"filter":        schema.StringAttribute{Computed: true},
						"trace_filter":  schema.StringAttribute{Computed: true},
						"tree_filter":   schema.StringAttribute{Computed: true},
						"backfill_from": schema.StringAttribute{Computed: true},
						"evaluators":    schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded evaluators."},
						"alerts":        schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded alerts."},
						"webhooks":      schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded webhooks."},
						"created_at":    schema.StringAttribute{Computed: true},
						"updated_at":    schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *RunRulesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RunRulesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RunRulesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if !data.SessionID.IsNull() && !data.SessionID.IsUnknown() {
		query.Set("session_id", data.SessionID.ValueString())
	}
	if !data.DatasetID.IsNull() && !data.DatasetID.IsUnknown() {
		query.Set("dataset_id", data.DatasetID.ValueString())
	}
	if !data.Type.IsNull() && !data.Type.IsUnknown() {
		query.Set("type", data.Type.ValueString())
	}
	if !data.NameContains.IsNull() && !data.NameContains.IsUnknown() {
		query.Set("name_contains", data.NameContains.ValueString())
	}

	var rules []runRulesListItemAPI
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/runs/rules", query, &rules); err != nil {
		resp.Diagnostics.AddError("Error listing run rules", err.Error())
		return
	}

	stringOrNull := func(s *string) types.String {
		if s != nil {
			return types.StringValue(*s)
		}
		return types.StringNull()
	}

	elems := make([]attr.Value, 0, len(rules))
	for _, rule := range rules {
		obj, diags := types.ObjectValue(runRuleObjectType.AttrTypes, map[string]attr.Value{
			"id":            types.StringValue(rule.ID),
			"display_name":  types.StringValue(rule.DisplayName),
			"session_id":    stringOrNull(rule.SessionID),
			"dataset_id":    stringOrNull(rule.DatasetID),
			"is_enabled":    types.BoolValue(rule.IsEnabled),
			"sampling_rate": types.Float64Value(rule.SamplingRate),
			"filter":        stringOrNull(rule.Filter),
			"trace_filter":  stringOrNull(rule.TraceFilter),
			"tree_filter":   stringOrNull(rule.TreeFilter),
			"backfill_from": stringOrNull(rule.BackfillFrom),
			"evaluators":    jsonEmptyArrayIsNull(rule.Evaluators),
			"alerts":        jsonEmptyArrayIsNull(rule.Alerts),
			"webhooks":      jsonEmptyArrayIsNull(rule.Webhooks),
			"created_at":    types.StringValue(rule.CreatedAt),
			"updated_at":    types.StringValue(rule.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(runRuleObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Rules = list

	tflog.Trace(ctx, "read run rules data source", map[string]interface{}{"count": len(rules)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

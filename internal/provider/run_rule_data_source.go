// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &RunRuleDataSource{}

func NewRunRuleDataSource() datasource.DataSource {
	return &RunRuleDataSource{}
}

type RunRuleDataSource struct {
	client *client.Client
}

type RunRuleDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	SessionID   types.String `tfsdk:"session_id"`
	IsEnabled   types.Bool   `tfsdk:"is_enabled"`
	SamplingRate types.Float64 `tfsdk:"sampling_rate"`
	Filter       types.String  `tfsdk:"filter"`
	TraceFilter  types.String  `tfsdk:"trace_filter"`
	TreeFilter   types.String  `tfsdk:"tree_filter"`
	BackfillFrom types.String  `tfsdk:"backfill_from"`
	Evaluators   types.String  `tfsdk:"evaluators"`
	Alerts       types.String  `tfsdk:"alerts"`
	Webhooks     types.String  `tfsdk:"webhooks"`
	CreatedAt    types.String  `tfsdk:"created_at"`
	UpdatedAt    types.String  `tfsdk:"updated_at"`
}

type runRuleDataSourceAPIResponse struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"display_name"`
	SessionID    *string         `json:"session_id"`
	IsEnabled    bool            `json:"is_enabled"`
	SamplingRate float64         `json:"sampling_rate"`
	Filter       *string         `json:"filter"`
	TraceFilter  *string         `json:"trace_filter"`
	TreeFilter   *string         `json:"tree_filter"`
	BackfillFrom *string         `json:"backfill_from"`
	Evaluators   json.RawMessage `json:"evaluators"`
	Alerts       json.RawMessage `json:"alerts"`
	Webhooks     json.RawMessage `json:"webhooks"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

func (d *RunRuleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run_rule"
}

func (d *RunRuleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith automation rule by ID or display name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier. Either `id` or `display_name` must be specified.",
				Optional: true, Computed: true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name. Either `id` or `display_name` must be specified.",
				Optional: true, Computed: true,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project/session ID.", Computed: true,
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is enabled.", Computed: true,
			},
			"sampling_rate": schema.Float64Attribute{
				MarkdownDescription: "The sampling rate.", Computed: true,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "Run filter expression.", Computed: true,
			},
			"trace_filter": schema.StringAttribute{
				MarkdownDescription: "Trace filter expression.", Computed: true,
			},
			"tree_filter": schema.StringAttribute{
				MarkdownDescription: "Tree filter expression.", Computed: true,
			},
			"backfill_from": schema.StringAttribute{
				MarkdownDescription: "Backfill start timestamp.", Computed: true,
			},
			"evaluators": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded evaluators.", Computed: true,
			},
			"alerts": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded alerts.", Computed: true,
			},
			"webhooks": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded webhooks.", Computed: true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.", Computed: true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.", Computed: true,
			},
		},
	}
}

func (d *RunRuleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RunRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RunRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.DisplayName.IsNull() && !data.DisplayName.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"display_name\" must be specified.")
		return
	}

	var rules []runRuleDataSourceAPIResponse
	err := d.client.Get(ctx, "/api/v1/runs/rules", nil, &rules)
	if err != nil {
		resp.Diagnostics.AddError("Error reading run rules", err.Error())
		return
	}

	var found *runRuleDataSourceAPIResponse
	for i := range rules {
		if idSet && rules[i].ID == data.ID.ValueString() {
			found = &rules[i]
			break
		}
		if nameSet && rules[i].DisplayName == data.DisplayName.ValueString() {
			found = &rules[i]
			break
		}
	}

	if found == nil {
		if idSet {
			resp.Diagnostics.AddError("Run Rule Not Found", fmt.Sprintf("No rule found with ID %q.", data.ID.ValueString()))
		} else {
			resp.Diagnostics.AddError("Run Rule Not Found", fmt.Sprintf("No rule found with display name %q.", data.DisplayName.ValueString()))
		}
		return
	}

	data.ID = types.StringValue(found.ID)
	data.DisplayName = types.StringValue(found.DisplayName)
	data.IsEnabled = types.BoolValue(found.IsEnabled)
	data.SamplingRate = types.Float64Value(found.SamplingRate)
	data.CreatedAt = types.StringValue(found.CreatedAt)
	data.UpdatedAt = types.StringValue(found.UpdatedAt)
	data.Evaluators = jsonEmptyArrayIsNull(found.Evaluators)
	data.Alerts = jsonEmptyArrayIsNull(found.Alerts)
	data.Webhooks = jsonEmptyArrayIsNull(found.Webhooks)

	if found.SessionID != nil {
		data.SessionID = types.StringValue(*found.SessionID)
	} else {
		data.SessionID = types.StringNull()
	}
	if found.Filter != nil {
		data.Filter = types.StringValue(*found.Filter)
	} else {
		data.Filter = types.StringNull()
	}
	if found.TraceFilter != nil {
		data.TraceFilter = types.StringValue(*found.TraceFilter)
	} else {
		data.TraceFilter = types.StringNull()
	}
	if found.TreeFilter != nil {
		data.TreeFilter = types.StringValue(*found.TreeFilter)
	} else {
		data.TreeFilter = types.StringNull()
	}
	if found.BackfillFrom != nil {
		data.BackfillFrom = types.StringValue(*found.BackfillFrom)
	} else {
		data.BackfillFrom = types.StringNull()
	}

	tflog.Trace(ctx, "read run rule data source", map[string]interface{}{"id": found.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

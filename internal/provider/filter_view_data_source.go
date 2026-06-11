// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &FilterViewDataSource{}

// NewFilterViewDataSource returns a new FilterViewDataSource for looking up
// an existing saved filter view within a tracing project.
func NewFilterViewDataSource() datasource.DataSource {
	return &FilterViewDataSource{}
}

// FilterViewDataSource reads a LangSmith filter view by session ID and view ID.
type FilterViewDataSource struct {
	client *client.Client
}

// FilterViewDataSourceModel holds the read-only attributes for a filter view lookup.
type FilterViewDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	SessionID         types.String `tfsdk:"session_id"`
	DisplayName       types.String `tfsdk:"display_name"`
	Description       types.String `tfsdk:"description"`
	FilterString      types.String `tfsdk:"filter_string"`
	TraceFilterString types.String `tfsdk:"trace_filter_string"`
	TreeFilterString  types.String `tfsdk:"tree_filter_string"`
	Type              types.String `tfsdk:"type"`
	StartTime         types.String `tfsdk:"start_time"`
	EndTime           types.String `tfsdk:"end_time"`
	Duration          types.String `tfsdk:"duration"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	WorkspaceID       types.String `tfsdk:"workspace_id"`
}

func (d *FilterViewDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filter_view"
}

func (d *FilterViewDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith filter view (saved filter) by project/session ID and view ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the filter view.",
				Required:            true,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project/session ID the filter view belongs to.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the filter view.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the filter view.",
				Computed:            true,
			},
			"filter_string": schema.StringAttribute{
				MarkdownDescription: "The run filter expression.",
				Computed:            true,
			},
			"trace_filter_string": schema.StringAttribute{
				MarkdownDescription: "The trace filter expression.",
				Computed:            true,
			},
			"tree_filter_string": schema.StringAttribute{
				MarkdownDescription: "The tree filter expression.",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of filter view (`runs` or `threads`).",
				Computed:            true,
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The start time filter (ISO 8601).",
				Computed:            true,
			},
			"end_time": schema.StringAttribute{
				MarkdownDescription: "The end time filter (ISO 8601).",
				Computed:            true,
			},
			"duration": schema.StringAttribute{
				MarkdownDescription: "The duration filter.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *FilterViewDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *FilterViewDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FilterViewDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result filterViewAPIResponse
	apiPath := fmt.Sprintf("/api/v1/sessions/%s/views/%s", data.SessionID.ValueString(), data.ID.ValueString())
	err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, apiPath, nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"Filter View Not Found",
				fmt.Sprintf("No filter view found with ID %q in session %q.", data.ID.ValueString(), data.SessionID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading filter view", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.Type = types.StringValue(result.Type)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	if result.SessionID != nil {
		data.SessionID = types.StringValue(*result.SessionID)
	}
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.FilterString, result.FilterString)
	setStateOptionalString(&data.TraceFilterString, result.TraceFilterString)
	setStateOptionalString(&data.TreeFilterString, result.TreeFilterString)
	setStateOptionalString(&data.StartTime, result.StartTime)
	setStateOptionalString(&data.EndTime, result.EndTime)
	setStateOptionalString(&data.Duration, result.Duration)

	tflog.Trace(ctx, "read filter view data source", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

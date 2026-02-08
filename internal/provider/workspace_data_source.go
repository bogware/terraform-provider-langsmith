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

var _ datasource.DataSource = &WorkspaceDataSource{}

// NewWorkspaceDataSource returns a new WorkspaceDataSource for looking up which
// corner of the territory you are working in.
func NewWorkspaceDataSource() datasource.DataSource {
	return &WorkspaceDataSource{}
}

// WorkspaceDataSource reads a LangSmith workspace by ID or display name.
// Fetches the full list from the API and finds the matching one -- no shortcut
// endpoint exists, so we ride the long trail.
type WorkspaceDataSource struct {
	client *client.Client
}

// WorkspaceDataSourceModel holds the read-only attributes for a workspace:
// display name, tenant handle, creation timestamp, and which outfit it belongs to.
type WorkspaceDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	DisplayName    types.String `tfsdk:"display_name"`
	TenantHandle   types.String `tfsdk:"tenant_handle"`
	OrganizationID types.String `tfsdk:"organization_id"`
	IsPersonal     types.Bool   `tfsdk:"is_personal"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

// workspaceDataSourceAPIResponse is the API response for a workspace lookup.
type workspaceDataSourceAPIResponse struct {
	ID             string  `json:"id"`
	DisplayName    string  `json:"display_name"`
	TenantHandle   string  `json:"tenant_handle"`
	OrganizationID *string `json:"organization_id"`
	IsPersonal     *bool   `json:"is_personal"`
	CreatedAt      string  `json:"created_at"`
}

func (d *WorkspaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *WorkspaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith workspace by ID or display name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the workspace. Either `id` or `display_name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the workspace. Either `id` or `display_name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"tenant_handle": schema.StringAttribute{
				MarkdownDescription: "The tenant handle of the workspace.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns this workspace.",
				Computed:            true,
			},
			"is_personal": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a personal workspace.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the workspace.",
				Computed:            true,
			},
		},
	}
}

func (d *WorkspaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.DisplayName.IsNull() && !data.DisplayName.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either \"id\" or \"display_name\" must be specified to look up a workspace.",
		)
		return
	}

	var results []workspaceDataSourceAPIResponse
	err := d.client.Get(ctx, "/api/v1/workspaces", nil, &results)
	if err != nil {
		resp.Diagnostics.AddError("Error reading workspaces", err.Error())
		return
	}

	var found *workspaceDataSourceAPIResponse
	for i := range results {
		if idSet {
			if results[i].ID == data.ID.ValueString() {
				found = &results[i]
				break
			}
		} else if nameSet {
			if results[i].DisplayName == data.DisplayName.ValueString() {
				found = &results[i]
				break
			}
		}
	}

	if found == nil {
		if idSet {
			resp.Diagnostics.AddError(
				"Workspace Not Found",
				fmt.Sprintf("No workspace found with ID %q.", data.ID.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Workspace Not Found",
				fmt.Sprintf("No workspace found with display name %q.", data.DisplayName.ValueString()),
			)
		}
		return
	}

	data.ID = types.StringValue(found.ID)
	data.DisplayName = types.StringValue(found.DisplayName)
	data.TenantHandle = types.StringValue(found.TenantHandle)

	if found.OrganizationID != nil {
		data.OrganizationID = types.StringValue(*found.OrganizationID)
	} else {
		data.OrganizationID = types.StringNull()
	}

	if found.IsPersonal != nil {
		data.IsPersonal = types.BoolValue(*found.IsPersonal)
	} else {
		data.IsPersonal = types.BoolNull()
	}

	data.CreatedAt = types.StringValue(found.CreatedAt)

	tflog.Trace(ctx, "read workspace data source", map[string]interface{}{"id": found.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

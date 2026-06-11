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

var _ datasource.DataSource = &WorkspacesDataSource{}

// NewWorkspacesDataSource returns a new WorkspacesDataSource that lists every
// workspace visible to the caller's API key.
func NewWorkspacesDataSource() datasource.DataSource {
	return &WorkspacesDataSource{}
}

// WorkspacesDataSource lists all workspaces (tenants) in the current
// organization. It is the natural companion to resource-level workspace_id:
// iterate the result with for_each to stamp out per-workspace resources.
type WorkspacesDataSource struct {
	client *client.Client
}

// WorkspacesDataSourceModel holds the inputs and the resulting workspace list.
type WorkspacesDataSourceModel struct {
	IncludeDeleted types.Bool `tfsdk:"include_deleted"`
	Workspaces     types.List `tfsdk:"workspaces"`
}

// workspaceListAPI mirrors the TenantForUser schema returned by
// GET /api/v1/workspaces.
type workspaceListAPI struct {
	ID             string  `json:"id"`
	DisplayName    string  `json:"display_name"`
	TenantHandle   *string `json:"tenant_handle"`
	OrganizationID *string `json:"organization_id"`
	IsPersonal     bool    `json:"is_personal"`
	IsDeleted      bool    `json:"is_deleted"`
	RoleID         *string `json:"role_id"`
	RoleName       *string `json:"role_name"`
	CreatedAt      string  `json:"created_at"`
}

var workspaceObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":              types.StringType,
	"display_name":    types.StringType,
	"tenant_handle":   types.StringType,
	"organization_id": types.StringType,
	"is_personal":     types.BoolType,
	"is_deleted":      types.BoolType,
	"role_id":         types.StringType,
	"role_name":       types.StringType,
	"created_at":      types.StringType,
}}

func (d *WorkspacesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspaces"
}

func (d *WorkspacesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all LangSmith workspaces visible to the configured API key. " +
			"Combine with `for_each` and resource-level `workspace_id` to manage the same resource across every workspace in the organization.",
		Attributes: map[string]schema.Attribute{
			"include_deleted": schema.BoolAttribute{
				MarkdownDescription: "Whether to include deleted workspaces in the results. Defaults to `false`.",
				Optional:            true,
			},
			"workspaces": schema.ListNestedAttribute{
				MarkdownDescription: "All workspaces visible to the caller.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the workspace. Use this as `workspace_id` on workspace-scoped resources.",
							Computed:            true,
						},
						"display_name": schema.StringAttribute{
							MarkdownDescription: "The display name of the workspace.",
							Computed:            true,
						},
						"tenant_handle": schema.StringAttribute{
							MarkdownDescription: "The URL handle of the workspace, if one has been set.",
							Computed:            true,
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the organization that owns the workspace.",
							Computed:            true,
						},
						"is_personal": schema.BoolAttribute{
							MarkdownDescription: "Whether this is a personal workspace.",
							Computed:            true,
						},
						"is_deleted": schema.BoolAttribute{
							MarkdownDescription: "Whether the workspace has been deleted.",
							Computed:            true,
						},
						"role_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the caller's role in this workspace, if any.",
							Computed:            true,
						},
						"role_name": schema.StringAttribute{
							MarkdownDescription: "The name of the caller's role in this workspace, if any.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the workspace.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *WorkspacesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if !data.IncludeDeleted.IsNull() && !data.IncludeDeleted.IsUnknown() && data.IncludeDeleted.ValueBool() {
		query.Set("include_deleted", "true")
	}

	var results []workspaceListAPI
	if err := d.client.Get(ctx, "/api/v1/workspaces", query, &results); err != nil {
		resp.Diagnostics.AddError("Error listing workspaces", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for _, w := range results {
		tenantHandle := types.StringNull()
		if w.TenantHandle != nil {
			tenantHandle = types.StringValue(*w.TenantHandle)
		}
		orgID := types.StringNull()
		if w.OrganizationID != nil {
			orgID = types.StringValue(*w.OrganizationID)
		}
		roleID := types.StringNull()
		if w.RoleID != nil {
			roleID = types.StringValue(*w.RoleID)
		}
		roleName := types.StringNull()
		if w.RoleName != nil {
			roleName = types.StringValue(*w.RoleName)
		}
		obj, diags := types.ObjectValue(workspaceObjectType.AttrTypes, map[string]attr.Value{
			"id":              types.StringValue(w.ID),
			"display_name":    types.StringValue(w.DisplayName),
			"tenant_handle":   tenantHandle,
			"organization_id": orgID,
			"is_personal":     types.BoolValue(w.IsPersonal),
			"is_deleted":      types.BoolValue(w.IsDeleted),
			"role_id":         roleID,
			"role_name":       roleName,
			"created_at":      types.StringValue(w.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(workspaceObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Workspaces = list

	tflog.Trace(ctx, "read workspaces data source", map[string]interface{}{"count": len(results)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

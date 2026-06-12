// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &PermissionsDataSource{}

// NewPermissionsDataSource returns a new PermissionsDataSource listing the
// permission descriptors available when authoring custom organization roles.
func NewPermissionsDataSource() datasource.DataSource {
	return &PermissionsDataSource{}
}

// PermissionsDataSource lists every permission known to the LangSmith API,
// along with its description and access scope. Useful when building the
// permissions list for a langsmith_org_role.
type PermissionsDataSource struct {
	client *client.Client
}

// PermissionsDataSourceModel holds the resulting permissions list.
type PermissionsDataSourceModel struct {
	Permissions types.List `tfsdk:"permissions"`
}

// permissionAPI mirrors the PermissionResponse schema returned by
// GET /api/v1/orgs/permissions.
type permissionAPI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AccessScope string `json:"access_scope"`
}

var permissionObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":         types.StringType,
	"description":  types.StringType,
	"access_scope": types.StringType,
}}

func (d *PermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_permissions"
}

func (d *PermissionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all permissions available in the organization, " +
			"for example when assembling the permission set of a `langsmith_org_role`.",
		Attributes: map[string]schema.Attribute{
			"permissions": schema.ListNestedAttribute{
				MarkdownDescription: "All available permission descriptors.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The permission identifier (e.g. `datasets:read`).",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "A human-readable description of the permission.",
							Computed:            true,
						},
						"access_scope": schema.StringAttribute{
							MarkdownDescription: "The scope at which the permission applies (`organization` or `workspace`).",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *PermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PermissionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []permissionAPI
	if err := d.client.Get(ctx, "/api/v1/orgs/permissions", nil, &results); err != nil {
		resp.Diagnostics.AddError("Error listing permissions", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for _, p := range results {
		obj, diags := types.ObjectValue(permissionObjectType.AttrTypes, map[string]attr.Value{
			"name":         types.StringValue(p.Name),
			"description":  types.StringValue(p.Description),
			"access_scope": types.StringValue(p.AccessScope),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(permissionObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Permissions = list

	tflog.Trace(ctx, "read permissions data source", map[string]interface{}{"count": len(results)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &OrganizationPermissionsDataSource{}

// permissionResponseRowAttrTypes matches OpenAPI `PermissionResponse`.
var permissionResponseRowAttrTypes = map[string]attr.Type{
	"name":         types.StringType,
	"description":  types.StringType,
	"access_scope": types.StringType,
}

// NewOrganizationPermissionsDataSource returns a data source backed by GET /api/v1/orgs/permissions.
func NewOrganizationPermissionsDataSource() datasource.DataSource {
	return &OrganizationPermissionsDataSource{}
}

// OrganizationPermissionsDataSource lists organization/workspace permission catalog entries.
type OrganizationPermissionsDataSource struct {
	client *client.Client
}

// OrganizationPermissionsDataSourceModel is Terraform state for GET /api/v1/orgs/permissions.
type OrganizationPermissionsDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Permissions types.List   `tfsdk:"permissions"`
}

type permissionResponseAPI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AccessScope string `json:"access_scope"`
}

func (d *OrganizationPermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_permissions"
}

func (d *OrganizationPermissionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith **permission definitions** (name, description, access scope) via GET [`/api/v1/orgs/permissions`](https://api.smith.langchain.com/openapi.json). " +
			"Use this data source to discover valid permission identifiers when configuring roles or reviewing access. " +
			"OpenAPI type: `PermissionResponse` with `access_scope` enum `organization` or `workspace`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable placeholder (`organization_permissions`).",
				Computed:            true,
			},
			"permissions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Permission identifier.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable description.",
						},
						"access_scope": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "`organization` or `workspace` (OpenAPI `AccessScope`).",
						},
					},
				},
				MarkdownDescription: "Permission rows returned by the API, sorted by `name` for stable Terraform state.",
			},
		},
	}
}

func (d *OrganizationPermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationPermissionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rows []permissionResponseAPI
	if err := d.client.Get(ctx, "/api/v1/orgs/permissions", nil, &rows); err != nil {
		resp.Diagnostics.AddError("Error reading organization permissions", err.Error())
		return
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	elems := make([]attr.Value, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		obj := types.ObjectValueMust(permissionResponseRowAttrTypes, map[string]attr.Value{
			"name":         types.StringValue(row.Name),
			"description":  types.StringValue(row.Description),
			"access_scope": types.StringValue(row.AccessScope),
		})
		elems = append(elems, obj)
	}

	permList, diags := types.ListValue(types.ObjectType{AttrTypes: permissionResponseRowAttrTypes}, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("organization_permissions")
	data.Permissions = permList
	tflog.Trace(ctx, "read organization_permissions data source", map[string]any{"count": len(rows)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

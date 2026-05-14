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

var _ datasource.DataSource = &TenantsDataSource{}

// tenantObjectAttrTypes matches the computed attributes on each element of
// `tenants` (LangSmith OpenAPI component TenantForUser).
var tenantObjectAttrTypes = map[string]attr.Type{
	"id":              types.StringType,
	"organization_id": types.StringType,
	"created_at":      types.StringType,
	"display_name":    types.StringType,
	"is_personal":     types.BoolType,
	"is_deleted":      types.BoolType,
	"tenant_handle":   types.StringType,
	"data_plane_url":  types.StringType,
	"read_only":       types.BoolType,
	"role_id":         types.StringType,
	"role_name":       types.StringType,
	"permissions":     types.ListType{ElemType: types.StringType},
}

// NewTenantsDataSource returns a data source backed by GET /api/v1/tenants.
func NewTenantsDataSource() datasource.DataSource {
	return &TenantsDataSource{}
}

// TenantsDataSource lists LangSmith tenants (workspaces) visible to the caller.
type TenantsDataSource struct {
	client *client.Client
}

// TenantsDataSourceModel is Terraform state for the tenants listing.
type TenantsDataSourceModel struct {
	SkipCreate     types.Bool `tfsdk:"skip_create"`
	IncludeDeleted types.Bool `tfsdk:"include_deleted"`
	Tenants        types.List `tfsdk:"tenants"`
}

type tenantForUserAPIResponse struct {
	ID             string   `json:"id"`
	OrganizationID *string  `json:"organization_id"`
	CreatedAt      string   `json:"created_at"`
	DisplayName    string   `json:"display_name"`
	IsPersonal     *bool    `json:"is_personal"`
	IsDeleted      *bool    `json:"is_deleted"`
	TenantHandle   *string  `json:"tenant_handle"`
	DataPlaneURL   *string  `json:"data_plane_url"`
	ReadOnly       *bool    `json:"read_only"`
	RoleID         *string  `json:"role_id"`
	RoleName       *string  `json:"role_name"`
	Permissions    []string `json:"permissions"`
}

func (d *TenantsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenants"
}

func (d *TenantsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith tenants (workspaces) visible to the authenticated user via GET `/api/v1/tenants`. " +
			"Each element matches the OpenAPI `TenantForUser` schema—the same object shape returned by GET `/api/v1/workspaces`, " +
			"but this endpoint is not limited to the current organization and supports the `skip_create` query flag. " +
			"To look up a single workspace within the current org by id or display name, use the `langsmith_workspace` data source.",
		Attributes: map[string]schema.Attribute{
			"skip_create": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When `true`, sent to the API as `skip_create=true` (see LangSmith OpenAPI). When unset, the query parameter is omitted and the API default applies.",
			},
			"include_deleted": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When `true`, sent to the API as `include_deleted=true` so deleted tenants may appear in the list. When unset, the query parameter is omitted and the API default applies.",
			},
			"tenants": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Workspace (tenant) UUID.",
						},
						"organization_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Owning organization UUID, if any.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Creation timestamp (RFC3339 from the API).",
						},
						"display_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Display name of the workspace.",
						},
						"is_personal": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is a personal workspace.",
						},
						"is_deleted": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this tenant is marked deleted.",
						},
						"tenant_handle": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Tenant handle, if set.",
						},
						"data_plane_url": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Data plane URL for the tenant, if exposed.",
						},
						"read_only": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Deprecated flag from the API; `false` when absent.",
						},
						"role_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Role UUID for the current user on this tenant, if any.",
						},
						"role_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Role name for the current user on this tenant, if any.",
						},
						"permissions": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Permission strings for the current user on this tenant.",
						},
					},
				},
				MarkdownDescription: "Tenants returned by the API (each is a `TenantForUser` in OpenAPI).",
			},
		},
	}
}

func (d *TenantsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TenantsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TenantsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if !data.SkipCreate.IsNull() && !data.SkipCreate.IsUnknown() && data.SkipCreate.ValueBool() {
		query.Set("skip_create", "true")
	}
	if !data.IncludeDeleted.IsNull() && !data.IncludeDeleted.IsUnknown() && data.IncludeDeleted.ValueBool() {
		query.Set("include_deleted", "true")
	}

	var results []tenantForUserAPIResponse
	err := d.client.Get(ctx, "/api/v1/tenants", query, &results)
	if err != nil {
		resp.Diagnostics.AddError("Error reading tenants", err.Error())
		return
	}

	tenantElems := make([]attr.Value, 0, len(results))
	for i := range results {
		t := &results[i]
		permList := types.ListNull(types.StringType)
		if t.Permissions != nil {
			permElems := make([]attr.Value, 0, len(t.Permissions))
			for _, p := range t.Permissions {
				permElems = append(permElems, types.StringValue(p))
			}
			permList = types.ListValueMust(types.StringType, permElems)
		}

		readOnly := types.BoolValue(false)
		if t.ReadOnly != nil {
			readOnly = types.BoolValue(*t.ReadOnly)
		}

		obj := types.ObjectValueMust(tenantObjectAttrTypes, map[string]attr.Value{
			"id":              types.StringValue(t.ID),
			"organization_id": types.StringPointerValue(t.OrganizationID),
			"created_at":      types.StringValue(t.CreatedAt),
			"display_name":    types.StringValue(t.DisplayName),
			"is_personal":     boolPointerAttrValue(t.IsPersonal),
			"is_deleted":      boolPointerAttrValue(t.IsDeleted),
			"tenant_handle":   types.StringPointerValue(t.TenantHandle),
			"data_plane_url":  types.StringPointerValue(t.DataPlaneURL),
			"read_only":       readOnly,
			"role_id":         types.StringPointerValue(t.RoleID),
			"role_name":       types.StringPointerValue(t.RoleName),
			"permissions":     permList,
		})
		tenantElems = append(tenantElems, obj)
	}

	tenantList, listDiags := types.ListValue(types.ObjectType{AttrTypes: tenantObjectAttrTypes}, tenantElems)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Tenants = tenantList

	tflog.Trace(ctx, "read tenants data source", map[string]interface{}{"count": len(results)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func boolPointerAttrValue(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*b)
}

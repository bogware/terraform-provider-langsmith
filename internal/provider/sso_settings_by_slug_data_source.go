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

var (
	_ datasource.DataSource              = &SSOSettingsBySlugDataSource{}
	_ datasource.DataSourceWithConfigure = &SSOSettingsBySlugDataSource{}
)

// providerSlimAttrTypes matches OpenAPI components/schemas/SSOProviderSlim.
var providerSlimAttrTypes = map[string]attr.Type{
	"provider_id":               types.StringType,
	"organization_id":           types.StringType,
	"organization_display_name": types.StringType,
}

var providerSlimObjectType = types.ObjectType{AttrTypes: providerSlimAttrTypes}

// NewSSOSettingsBySlugDataSource returns a data source for GET
// /api/v1/sso/settings/{sso_login_slug} (public slug lookup of SSO providers).
func NewSSOSettingsBySlugDataSource() datasource.DataSource {
	return &SSOSettingsBySlugDataSource{}
}

// SSOSettingsBySlugDataSource reads SSO provider summaries for a login slug.
type SSOSettingsBySlugDataSource struct {
	client *client.Client
}

// SSOSettingsBySlugDataSourceModel is Terraform state for the slug lookup.
type SSOSettingsBySlugDataSourceModel struct {
	SSOLoginSlug types.String `tfsdk:"sso_login_slug"`
	Providers    types.List   `tfsdk:"providers"`
}

type ssoProviderSlimAPIResponse struct {
	ProviderID              string `json:"provider_id"`
	OrganizationID          string `json:"organization_id"`
	OrganizationDisplayName string `json:"organization_display_name"`
}

func (d *SSOSettingsBySlugDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_settings_by_slug"
}

func (d *SSOSettingsBySlugDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read SSO provider summaries for a given **SSO login slug** via `GET /api/v1/sso/settings/{sso_login_slug}` (OpenAPI `SSOProviderSlim`). " +
			"This slug-scoped endpoint returns only provider and organization identifiers; it does **not** return full SAML metadata or org provisioning defaults. " +
			"For org-level SAML configuration managed in Terraform, use the `langsmith_sso_settings` resource (`/api/v1/orgs/current/sso-settings`). " +
			"Email lookup and email-verification flows under `/api/v1/sso/...` are interactive-only and are not exposed as Terraform data sources.",
		Attributes: map[string]schema.Attribute{
			"sso_login_slug": schema.StringAttribute{
				MarkdownDescription: "SSO login slug path parameter (`sso_login_slug` in the LangSmith OpenAPI).",
				Required:            true,
			},
			"providers": schema.ListNestedAttribute{
				MarkdownDescription: "Organizations and provider IDs associated with this slug (OpenAPI array of `SSOProviderSlim`). May be empty.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider_id": schema.StringAttribute{
							MarkdownDescription: "SSO provider UUID.",
							Computed:            true,
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "Organization UUID.",
							Computed:            true,
						},
						"organization_display_name": schema.StringAttribute{
							MarkdownDescription: "Human-readable organization name.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *SSOSettingsBySlugDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SSOSettingsBySlugDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SSOSettingsBySlugDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	slug := data.SSOLoginSlug.ValueString()
	apiPath := "/api/v1/sso/settings/" + url.PathEscape(slug)

	var apiRows []ssoProviderSlimAPIResponse
	if err := d.client.Get(ctx, apiPath, nil, &apiRows); err != nil {
		resp.Diagnostics.AddError("Error reading SSO settings by slug", err.Error())
		return
	}

	objs := make([]attr.Value, len(apiRows))
	for i, row := range apiRows {
		obj, diags := types.ObjectValue(providerSlimAttrTypes, map[string]attr.Value{
			"provider_id":               types.StringValue(row.ProviderID),
			"organization_id":           types.StringValue(row.OrganizationID),
			"organization_display_name": types.StringValue(row.OrganizationDisplayName),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		objs[i] = obj
	}

	listVal, diags := types.ListValue(providerSlimObjectType, objs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Providers = listVal
	tflog.Trace(ctx, "read SSO settings by slug data source", map[string]interface{}{"sso_login_slug": slug, "count": len(apiRows)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

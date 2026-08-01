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

var _ datasource.DataSource = &OAuthAuthorizedAppsDataSource{}

// NewOAuthAuthorizedAppsDataSource returns a data source listing the OAuth
// applications the caller has granted access to.
func NewOAuthAuthorizedAppsDataSource() datasource.DataSource {
	return &OAuthAuthorizedAppsDataSource{}
}

// OAuthAuthorizedAppsDataSource reads
// GET /api/v1/platform/oauth/authorized-apps.
type OAuthAuthorizedAppsDataSource struct {
	client *client.Client
}

// OAuthAuthorizedAppsDataSourceModel maps the Terraform schema.
type OAuthAuthorizedAppsDataSourceModel struct {
	Apps []oauthAuthorizedAppModel `tfsdk:"apps"`
}

type oauthAuthorizedAppModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientName   types.String `tfsdk:"client_name"`
	ClientURI    types.String `tfsdk:"client_uri"`
	LogoURI      types.String `tfsdk:"logo_uri"`
	AuthorizedAt types.String `tfsdk:"authorized_at"`
	Scopes       types.List   `tfsdk:"scopes"`
}

type oauthAuthorizedAppAPI struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	ClientURI    string   `json:"client_uri"`
	LogoURI      string   `json:"logo_uri"`
	AuthorizedAt string   `json:"authorized_at"`
	Scopes       []string `json:"scopes"`
}

func (d *OAuthAuthorizedAppsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth_authorized_apps"
}

func (d *OAuthAuthorizedAppsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the OAuth applications the authenticated identity has granted access to, and the scopes each was granted. " +
			"This reads the caller's own authorizations — it is an audit view of what has been consented to, not an organization-wide report.",
		Attributes: map[string]schema.Attribute{
			"apps": schema.ListNestedAttribute{
				MarkdownDescription: "The authorized applications.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"client_id": schema.StringAttribute{
							MarkdownDescription: "Client identifier of the authorized application.",
							Computed:            true,
						},
						"client_name": schema.StringAttribute{
							MarkdownDescription: "Display name of the application.",
							Computed:            true,
						},
						"client_uri": schema.StringAttribute{
							MarkdownDescription: "Home page of the application.",
							Computed:            true,
						},
						"logo_uri": schema.StringAttribute{
							MarkdownDescription: "Logo of the application.",
							Computed:            true,
						},
						"authorized_at": schema.StringAttribute{
							MarkdownDescription: "When access was granted.",
							Computed:            true,
						},
						"scopes": schema.ListAttribute{
							MarkdownDescription: "Scopes granted to the application.",
							ElementType:         types.StringType,
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *OAuthAuthorizedAppsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *OAuthAuthorizedAppsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OAuthAuthorizedAppsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var listResp []oauthAuthorizedAppAPI
	if err := d.client.Get(ctx, "/api/v1/platform/oauth/authorized-apps", nil, &listResp); err != nil {
		resp.Diagnostics.AddError("Error listing authorized OAuth applications", err.Error())
		return
	}

	data.Apps = make([]oauthAuthorizedAppModel, 0, len(listResp))
	for _, a := range listResp {
		scopes, diags := types.ListValueFrom(ctx, types.StringType, a.Scopes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Apps = append(data.Apps, oauthAuthorizedAppModel{
			ClientID:     types.StringValue(a.ClientID),
			ClientName:   types.StringValue(a.ClientName),
			ClientURI:    types.StringValue(a.ClientURI),
			LogoURI:      types.StringValue(a.LogoURI),
			AuthorizedAt: types.StringValue(a.AuthorizedAt),
			Scopes:       scopes,
		})
	}

	tflog.Trace(ctx, "read OAuth authorized apps", map[string]interface{}{"count": len(data.Apps)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

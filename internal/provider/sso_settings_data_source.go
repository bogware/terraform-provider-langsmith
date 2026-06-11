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

var _ datasource.DataSource = &SSOSettingsDataSource{}

// NewSSOSettingsDataSource returns a new SSOSettingsDataSource for reading
// the current organization's SSO settings.
func NewSSOSettingsDataSource() datasource.DataSource {
	return &SSOSettingsDataSource{}
}

// SSOSettingsDataSource reads the current organization's SSO settings. This
// is org-scoped: it does not take a workspace_id.
type SSOSettingsDataSource struct {
	client *client.Client
}

// SSOSettingsDataSourceModel holds the read-only attributes for an SSO
// settings lookup.
type SSOSettingsDataSourceModel struct {
	ID                     types.String `tfsdk:"id"`
	DefaultWorkspaceRoleID types.String `tfsdk:"default_workspace_role_id"`
	DefaultWorkspaceIDs    types.String `tfsdk:"default_workspace_ids"`
	MetadataURL            types.String `tfsdk:"metadata_url"`
	MetadataXML            types.String `tfsdk:"metadata_xml"`
	ProviderID             types.String `tfsdk:"provider_id"`
	OrganizationID         types.String `tfsdk:"organization_id"`
}

func (d *SSOSettingsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_settings"
}

func (d *SSOSettingsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to read the current organization's LangSmith SSO settings. If the organization has multiple SSO configurations, specify `id` to select one.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the SSO settings. Optional when the organization has exactly one SSO configuration.",
				Optional:            true,
				Computed:            true,
			},
			"default_workspace_role_id": schema.StringAttribute{
				MarkdownDescription: "Default role ID for SSO-provisioned users.",
				Computed:            true,
			},
			"default_workspace_ids": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of default workspace IDs for SSO-provisioned users.",
				Computed:            true,
			},
			"metadata_url": schema.StringAttribute{
				MarkdownDescription: "The SAML metadata URL.",
				Computed:            true,
			},
			"metadata_xml": schema.StringAttribute{
				MarkdownDescription: "The SAML metadata XML.",
				Computed:            true,
				Sensitive:           true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "The SSO provider ID.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns these SSO settings.",
				Computed:            true,
			},
		},
	}
}

func (d *SSOSettingsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SSOSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SSOSettingsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API only offers a list endpoint for the current organization.
	var listResult ssoSettingsListAPIResponse
	err := d.client.Get(ctx, "/api/v1/orgs/current/sso-settings", nil, &listResult)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"SSO Settings Not Found",
				"The current organization has no SSO settings.",
			)
			return
		}
		resp.Diagnostics.AddError("Error reading SSO settings", err.Error())
		return
	}

	if len(listResult) == 0 {
		resp.Diagnostics.AddError(
			"SSO Settings Not Found",
			"The current organization has no SSO settings configured.",
		)
		return
	}

	var found *ssoSettingsAPIResponse
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		id := data.ID.ValueString()
		for i := range listResult {
			if listResult[i].ID == id {
				found = &listResult[i]
				break
			}
		}
		if found == nil {
			resp.Diagnostics.AddError(
				"SSO Settings Not Found",
				fmt.Sprintf("No SSO settings found with ID %q in the current organization.", id),
			)
			return
		}
	} else {
		if len(listResult) > 1 {
			resp.Diagnostics.AddError(
				"Multiple SSO Settings Found",
				fmt.Sprintf("The current organization has %d SSO configurations. Specify \"id\" to select one.", len(listResult)),
			)
			return
		}
		found = &listResult[0]
	}

	data.ID = types.StringValue(found.ID)
	data.OrganizationID = types.StringValue(found.OrganizationID)
	data.ProviderID = types.StringValue(found.ProviderID)

	if found.DefaultWorkspaceRoleID != "" {
		data.DefaultWorkspaceRoleID = types.StringValue(found.DefaultWorkspaceRoleID)
	} else {
		data.DefaultWorkspaceRoleID = types.StringNull()
	}

	data.DefaultWorkspaceIDs = jsonStringValue(found.DefaultWorkspaceIDs)

	if found.MetadataURL != "" {
		data.MetadataURL = types.StringValue(found.MetadataURL)
	} else {
		data.MetadataURL = types.StringNull()
	}

	if found.MetadataXML != "" {
		data.MetadataXML = types.StringValue(found.MetadataXML)
	} else {
		data.MetadataXML = types.StringNull()
	}

	tflog.Trace(ctx, "read SSO settings data source", map[string]interface{}{"id": found.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

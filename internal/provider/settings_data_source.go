// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &SettingsDataSource{}

// NewSettingsDataSource returns a data source for GET /api/v1/settings.
func NewSettingsDataSource() datasource.DataSource {
	return &SettingsDataSource{}
}

// SettingsDataSource reads the current workspace settings (OpenAPI Tenant).
type SettingsDataSource struct {
	client *client.Client
}

// SettingsDataSourceModel holds read-only workspace settings.
type SettingsDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	DisplayName  types.String `tfsdk:"display_name"`
	TenantHandle types.String `tfsdk:"tenant_handle"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (d *SettingsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings"
}

func (d *SettingsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the **current workspace** from `GET /api/v1/settings` (OpenAPI `Tenant`: id, display name, handle, created time). " +
			"The workspace is whichever tenant the provider targets (`tenant_id` / `LANGSMITH_TENANT_ID`). " +
			"To change the workspace handle, use the `langsmith_settings` resource (`POST /api/v1/settings/handle`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The workspace (tenant) UUID.",
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The workspace display name.",
				Computed:            true,
			},
			"tenant_handle": schema.StringAttribute{
				MarkdownDescription: "The workspace handle (slug), if set.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Workspace creation timestamp (RFC3339).",
				Computed:            true,
			},
		},
	}
}

func (d *SettingsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SettingsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api settingsTenantAPIResponse
	if err := d.client.Get(ctx, "/api/v1/settings", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error reading workspace settings", err.Error())
		return
	}

	data.ID = types.StringValue(api.ID)
	data.DisplayName = types.StringValue(api.DisplayName)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	if api.TenantHandle != nil {
		if trimmed := strings.TrimSpace(*api.TenantHandle); trimmed != "" {
			data.TenantHandle = types.StringValue(trimmed)
		} else {
			data.TenantHandle = types.StringNull()
		}
	} else {
		data.TenantHandle = types.StringNull()
	}

	tflog.Trace(ctx, "read settings data source", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

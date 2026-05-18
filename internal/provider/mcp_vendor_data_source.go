// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &MCPVendorDataSource{}

func NewMCPVendorDataSource() datasource.DataSource {
	return &MCPVendorDataSource{}
}

type MCPVendorDataSource struct {
	client *client.Client
}

type MCPVendorDataSourceModel struct {
	VendorSlug   types.String `tfsdk:"vendor_slug"`
	VendorID     types.String `tfsdk:"vendor_id"`
	ProviderID   types.String `tfsdk:"provider_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Icon         types.String `tfsdk:"icon"`
	Status       types.String `tfsdk:"status"`
	SettingsJSON types.String `tfsdk:"settings_json"`
}

type mcpVendorAPI struct {
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	Name        string      `json:"name"`
	ProviderID  string      `json:"provider_id"`
	Settings    interface{} `json:"settings"`
	Status      string      `json:"status"`
	VendorID    string      `json:"vendor_id"`
}

func (d *MCPVendorDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_vendor"
}

func (d *MCPVendorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an MCP vendor by `vendor_slug`. MCP vendors are read-only platform-level registrations (no resource counterpart).",
		Attributes: map[string]schema.Attribute{
			"vendor_slug":   schema.StringAttribute{Required: true},
			"vendor_id":     schema.StringAttribute{Computed: true},
			"provider_id":   schema.StringAttribute{Computed: true},
			"name":          schema.StringAttribute{Computed: true},
			"description":   schema.StringAttribute{Computed: true},
			"icon":          schema.StringAttribute{Computed: true},
			"status":        schema.StringAttribute{Computed: true, MarkdownDescription: "`enabled` or `disabled`."},
			"settings_json": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded vendor-specific settings."},
		},
	}
}

func (d *MCPVendorDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *MCPVendorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MCPVendorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api mcpVendorAPI
	if err := d.client.Get(ctx, "/v1/platform/mcp-vendors/"+data.VendorSlug.ValueString(), nil, &api); err != nil {
		resp.Diagnostics.AddError("Error reading MCP vendor", err.Error())
		return
	}
	data.VendorID = types.StringValue(api.VendorID)
	data.ProviderID = types.StringValue(api.ProviderID)
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	data.Icon = types.StringValue(api.Icon)
	data.Status = types.StringValue(api.Status)
	if api.Settings != nil {
		b, _ := json.Marshal(api.Settings)
		data.SettingsJSON = jsonStringValue(b)
	} else {
		data.SettingsJSON = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

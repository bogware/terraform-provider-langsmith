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
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &MCPVendorDetailsDataSource{}

// NewMCPVendorDetailsDataSource returns a data source exposing the three
// per-vendor discovery endpoints: the linked account, the vendor's MCP servers,
// and the tools it publishes.
//
// They are combined into one data source rather than three because they answer a
// single question — "what does this vendor currently expose to us?" — and each
// is fetched only when asked for, so nothing is paid for unless it is used.
func NewMCPVendorDetailsDataSource() datasource.DataSource {
	return &MCPVendorDetailsDataSource{}
}

// MCPVendorDetailsDataSource reads the account, mcp-servers and tools endpoints
// under /api/v1/platform/mcp-vendors/{vendor_slug}.
type MCPVendorDetailsDataSource struct {
	client *client.Client
}

// MCPVendorDetailsDataSourceModel maps the Terraform schema.
type MCPVendorDetailsDataSourceModel struct {
	VendorSlug     types.String `tfsdk:"vendor_slug"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	IncludeAccount types.Bool   `tfsdk:"include_account"`
	IncludeServers types.Bool   `tfsdk:"include_servers"`
	IncludeTools   types.Bool   `tfsdk:"include_tools"`
	Account        types.String `tfsdk:"account"`
	MCPServers     types.String `tfsdk:"mcp_servers"`
	Tools          types.String `tfsdk:"tools"`
}

func (d *MCPVendorDetailsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_vendor_details"
}

func (d *MCPVendorDetailsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads what an MCP vendor currently exposes to the workspace: the linked account, the vendor's MCP servers, and the tools it publishes.\n\n" +
			"Each section is fetched only when its `include_*` flag is set, because a vendor with no linked account answers 404 for `account` while still serving the rest. " +
			"The payloads are vendor-specific and not pinned down by the API, so they are returned as JSON for `jsondecode()`.",
		Attributes: map[string]schema.Attribute{
			"vendor_slug": schema.StringAttribute{
				MarkdownDescription: "Slug of the MCP vendor (for example `slack`).",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"include_account": schema.BoolAttribute{
				MarkdownDescription: "Fetch the linked vendor account. Defaults to `false`.",
				Optional:            true,
			},
			"include_servers": schema.BoolAttribute{
				MarkdownDescription: "Fetch the vendor's MCP servers. Defaults to `false`.",
				Optional:            true,
			},
			"include_tools": schema.BoolAttribute{
				MarkdownDescription: "Fetch the tools the vendor publishes. Defaults to `false`.",
				Optional:            true,
			},
			"account": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded account information, when `include_account` is set. Null otherwise.",
				Computed:            true,
			},
			"mcp_servers": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded list of MCP servers, when `include_servers` is set. Null otherwise.",
				Computed:            true,
			},
			"tools": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded list of tools, when `include_tools` is set. Null otherwise.",
				Computed:            true,
			},
		},
	}
}

func (d *MCPVendorDetailsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *MCPVendorDetailsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MCPVendorDetailsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)
	base := "/api/v1/platform/mcp-vendors/" + data.VendorSlug.ValueString()

	fetch := func(enabled types.Bool, suffix, label string, dst *types.String) bool {
		if !enabled.ValueBool() {
			*dst = types.StringNull()
			return true
		}
		var raw json.RawMessage
		if err := c.Get(ctx, base+suffix, nil, &raw); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error reading MCP vendor %s", label), err.Error())
			return false
		}
		*dst = jsonStringValue(raw)
		return true
	}

	if !fetch(data.IncludeAccount, "/account", "account", &data.Account) ||
		!fetch(data.IncludeServers, "/mcp-servers", "servers", &data.MCPServers) ||
		!fetch(data.IncludeTools, "/tools", "tools", &data.Tools) {
		return
	}

	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read mcp vendor details", map[string]interface{}{"vendor_slug": data.VendorSlug.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

var _ datasource.DataSource = &MCPVendorsDataSource{}

func NewMCPVendorsDataSource() datasource.DataSource {
	return &MCPVendorsDataSource{}
}

type MCPVendorsDataSource struct {
	client *client.Client
}

type MCPVendorsDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Vendors     types.List   `tfsdk:"vendors"`
}

// mcpVendorsItem mirrors a single entry in the GET /v1/platform/mcp-vendors
// list response. The list endpoint returns only this subset of fields.
type mcpVendorsItem struct {
	VendorID    string `json:"vendor_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Icon        string `json:"icon"`
}

type mcpVendorsAPIResponse struct {
	MCPVendors []mcpVendorsItem `json:"mcp_vendors"`
}

var mcpVendorsObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"vendor_id":   types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"status":      types.StringType,
	"icon":        types.StringType,
}}

func (d *MCPVendorsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_vendors"
}

func (d *MCPVendorsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the MCP vendors available to the workspace. MCP vendors are read-only platform-level registrations (no resource counterpart).",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"vendors": schema.ListNestedAttribute{
				MarkdownDescription: "The list of MCP vendors.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"vendor_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Stable identifier for the vendor (e.g. `arcade`)."},
						"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable vendor name."},
						"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Vendor description."},
						"status":      schema.StringAttribute{Computed: true, MarkdownDescription: "`enabled` or `disabled`."},
						"icon":        schema.StringAttribute{Computed: true, MarkdownDescription: "URL of the vendor icon."},
					},
				},
			},
		},
	}
}

func (d *MCPVendorsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *MCPVendorsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MCPVendorsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var api mcpVendorsAPIResponse
	if err := c.Get(ctx, "/v1/platform/mcp-vendors", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error listing MCP vendors", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(api.MCPVendors))
	for _, v := range api.MCPVendors {
		obj, diags := types.ObjectValue(mcpVendorsObjectType.AttrTypes, map[string]attr.Value{
			"vendor_id":   types.StringValue(v.VendorID),
			"name":        types.StringValue(v.Name),
			"description": types.StringValue(v.Description),
			"status":      types.StringValue(v.Status),
			"icon":        types.StringValue(v.Icon),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(mcpVendorsObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Vendors = list

	// The list endpoint does not echo a workspace id; guarantee workspace_id is
	// never left unknown after read.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read mcp_vendors data source", map[string]interface{}{"count": len(api.MCPVendors)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

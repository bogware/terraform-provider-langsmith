// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// toolsPageSize is the page size used when listing tools. The list endpoint
// returns a `next_offset` cursor (null when exhausted), so we page until it is
// absent.
const toolsPageSize = 100

var _ datasource.DataSource = &ToolsDataSource{}

// NewToolsDataSource returns a data source that lists LangSmith platform tools.
func NewToolsDataSource() datasource.DataSource {
	return &ToolsDataSource{}
}

// ToolsDataSource lists LangSmith platform tools. It pages through
// GET /v1/platform/tools until the response no longer carries a next_offset.
type ToolsDataSource struct {
	client *client.Client
}

// ToolsDataSourceModel holds the workspace override input and the resulting
// tools list.
type ToolsDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Tools       types.List   `tfsdk:"tools"`
}

// toolsListResponse mirrors the envelope returned by GET /v1/platform/tools.
type toolsListResponse struct {
	Tools      []toolAPI `json:"tools"`
	Total      int64     `json:"total"`
	NextOffset *int64    `json:"next_offset"`
}

var toolsObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"handle":      types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"parameters":  types.StringType,
	"returns":     types.StringType,
	"metadata":    types.StringType,
	"enabled":     types.BoolType,
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}}

func (d *ToolsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tools"
}

func (d *ToolsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith platform-level tool definitions in a workspace.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list tools from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tools": schema.ListNestedAttribute{
				MarkdownDescription: "The tools defined in the workspace.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the tool.",
							Computed:            true,
						},
						"handle": schema.StringAttribute{
							MarkdownDescription: "The stable handle used to reference the tool.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The display name of the tool.",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "The tool description shown to model callers.",
							Computed:            true,
						},
						"parameters": schema.StringAttribute{
							MarkdownDescription: "JSON-encoded JSON Schema object describing the tool's input parameters.",
							Computed:            true,
						},
						"returns": schema.StringAttribute{
							MarkdownDescription: "JSON-encoded JSON Schema object describing the tool's return type.",
							Computed:            true,
						},
						"metadata": schema.StringAttribute{
							MarkdownDescription: "JSON-encoded free-form metadata.",
							Computed:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether the tool is enabled.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the tool.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the tool.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ToolsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ToolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ToolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var all []toolAPI
	offset := 0
	for {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(toolsPageSize))
		query.Set("offset", strconv.Itoa(offset))

		var page toolsListResponse
		if err := c.Get(ctx, "/api/v1/platform/tools", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing tools", err.Error())
			return
		}
		all = append(all, page.Tools...)
		if page.NextOffset == nil || len(page.Tools) == 0 {
			break
		}
		offset = int(*page.NextOffset)
	}

	elems := make([]attr.Value, 0, len(all))
	for _, t := range all {
		obj, diags := types.ObjectValue(toolsObjectType.AttrTypes, map[string]attr.Value{
			"id":          types.StringValue(t.ID),
			"handle":      types.StringValue(t.Handle),
			"name":        types.StringValue(t.Name),
			"description": types.StringValue(t.Description),
			"parameters":  toolsJSONObjectValue(t.Parameters),
			"returns":     toolsJSONObjectValue(t.Returns),
			"metadata":    toolsJSONObjectValue(t.Metadata),
			"enabled":     types.BoolValue(t.Enabled),
			"created_at":  types.StringValue(t.CreatedAt),
			"updated_at":  types.StringValue(t.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(toolsObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Tools = list

	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read tools data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// toolsJSONObjectValue serializes a decoded JSON sub-object back to a
// normalized JSON string, returning null when the object is absent.
func toolsJSONObjectValue(m map[string]interface{}) types.String {
	if len(m) == 0 {
		return types.StringNull()
	}
	b, _ := json.Marshal(m)
	return jsonStringValue(b)
}

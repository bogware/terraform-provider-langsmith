// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ datasource.DataSource                   = &ToolDataSource{}
	_ datasource.DataSourceWithConfigure      = &ToolDataSource{}
	_ datasource.DataSourceWithValidateConfig = &ToolDataSource{}
)

// NewToolDataSource returns a data source that reads a LangSmith platform tool
// from the registry (GET /v1/platform/tools/{handle} or .../id/{id}).
func NewToolDataSource() datasource.DataSource {
	return &ToolDataSource{}
}

// ToolDataSource reads platform tool metadata from LangSmith.
type ToolDataSource struct {
	client *client.Client
}

// ToolDataSourceModel maps the tools.Tool OpenAPI schema to Terraform state.
type ToolDataSourceModel struct {
	Handle      types.String `tfsdk:"handle"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	TenantID    types.String `tfsdk:"tenant_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	Metadata    types.String `tfsdk:"metadata"`
	Parameters  types.String `tfsdk:"parameters"`
	Returns     types.String `tfsdk:"returns"`
}

// platformToolAPIResponse matches components/schemas/tools.Tool in the LangSmith OpenAPI spec.
type platformToolAPIResponse struct {
	ID          string          `json:"id"`
	Handle      string          `json:"handle"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	TenantID    string          `json:"tenant_id"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	Metadata    json.RawMessage `json:"metadata"`
	Parameters  json.RawMessage `json:"parameters"`
	Returns     json.RawMessage `json:"returns"`
}

func (d *ToolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool"
}

func (d *ToolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read a LangSmith **platform tool** from the registry (`GET /v1/platform/tools/{handle}` or `GET /v1/platform/tools/id/{id}`). " +
			"Specify exactly one of `handle` or `id`. " +
			"Use the same authentication as other data sources: `api_key` / `LANGSMITH_API_KEY`, and for org-scoped keys `tenant_id` / `LANGSMITH_TENANT_ID` (sent as `X-Tenant-Id`).",
		Attributes: map[string]schema.Attribute{
			"handle": schema.StringAttribute{
				MarkdownDescription: "Stable tool handle. Exactly one of `handle` or `id` must be set.",
				Optional:            true,
				Computed:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Tool UUID. Exactly one of `handle` or `id` must be set.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable tool name from the API (`name` field).",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Tool description from the API (`description` field).",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the tool is enabled (`enabled` field).",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Workspace tenant id associated with the tool (`tenant_id` field).",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp (`created_at` field).",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp (`updated_at` field).",
				Computed:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON object of tool metadata (`metadata` field).",
				Computed:            true,
			},
			"parameters": schema.StringAttribute{
				MarkdownDescription: "JSON object describing tool parameters (`parameters` field).",
				Computed:            true,
			},
			"returns": schema.StringAttribute{
				MarkdownDescription: "JSON object describing return shape (`returns` field).",
				Computed:            true,
			},
		},
	}
}

func (d *ToolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ToolDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data ToolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	handleSet := !data.Handle.IsNull() && !data.Handle.IsUnknown()
	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()

	if handleSet && idSet {
		resp.Diagnostics.AddError(
			"Conflicting Arguments",
			"Only one of \"handle\" or \"id\" may be set. These arguments are mutually exclusive.",
		)
		return
	}
	if !handleSet && !idSet {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either \"handle\" or \"id\" must be specified.",
		)
	}
}

func (d *ToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ToolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	handleSet := !data.Handle.IsNull() && !data.Handle.IsUnknown()
	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()

	if handleSet && idSet {
		resp.Diagnostics.AddError(
			"Conflicting Arguments",
			"Only one of \"handle\" or \"id\" may be set. These arguments are mutually exclusive.",
		)
		return
	}
	if !handleSet && !idSet {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either \"handle\" or \"id\" must be specified.",
		)
		return
	}

	var path string
	switch {
	case handleSet:
		path = "/v1/platform/tools/" + url.PathEscape(data.Handle.ValueString())
	case idSet:
		path = "/v1/platform/tools/id/" + url.PathEscape(data.ID.ValueString())
	}

	var result platformToolAPIResponse
	if err := d.client.Get(ctx, path, nil, &result); err != nil {
		resp.Diagnostics.AddError("Error reading platform tool", err.Error())
		return
	}

	mapPlatformToolResponse(&data, &result)

	tflog.Trace(ctx, "read platform tool data source", map[string]interface{}{"id": data.ID.ValueString(), "handle": data.Handle.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapPlatformToolResponse(data *ToolDataSourceModel, result *platformToolAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Handle = types.StringValue(result.Handle)

	if result.Name != "" {
		data.Name = types.StringValue(result.Name)
	} else {
		data.Name = types.StringNull()
	}
	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.Enabled = types.BoolValue(result.Enabled)

	if result.TenantID != "" {
		data.TenantID = types.StringValue(result.TenantID)
	} else {
		data.TenantID = types.StringNull()
	}
	if result.CreatedAt != "" {
		data.CreatedAt = types.StringValue(result.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if result.UpdatedAt != "" {
		data.UpdatedAt = types.StringValue(result.UpdatedAt)
	} else {
		data.UpdatedAt = types.StringNull()
	}

	data.Metadata = jsonStringValue(result.Metadata)
	data.Parameters = jsonStringValue(result.Parameters)
	data.Returns = jsonStringValue(result.Returns)
}

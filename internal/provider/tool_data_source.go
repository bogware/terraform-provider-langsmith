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

var _ datasource.DataSource = &ToolDataSource{}

func NewToolDataSource() datasource.DataSource {
	return &ToolDataSource{}
}

type ToolDataSource struct {
	client *client.Client
}

type ToolDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Handle      types.String `tfsdk:"handle"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Parameters  types.String `tfsdk:"parameters"`
	Returns     types.String `tfsdk:"returns"`
	Metadata    types.String `tfsdk:"metadata"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	TenantID    types.String `tfsdk:"tenant_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (d *ToolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool"
}

func (d *ToolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a LangSmith platform tool by `handle`.",
		Attributes: map[string]schema.Attribute{
			"handle":      schema.StringAttribute{Required: true},
			"id":          schema.StringAttribute{Computed: true},
			"name":        schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Computed: true},
			"parameters":  schema.StringAttribute{Computed: true},
			"returns":     schema.StringAttribute{Computed: true},
			"metadata":    schema.StringAttribute{Computed: true},
			"enabled":     schema.BoolAttribute{Computed: true},
			"tenant_id":   schema.StringAttribute{Computed: true},
			"created_at":  schema.StringAttribute{Computed: true},
			"updated_at":  schema.StringAttribute{Computed: true},
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

func (d *ToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ToolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api toolAPI
	if err := d.client.Get(ctx, "/v1/platform/tools/"+data.Handle.ValueString(), nil, &api); err != nil {
		resp.Diagnostics.AddError("Error reading tool", err.Error())
		return
	}
	data.ID = types.StringValue(api.ID)
	data.Handle = types.StringValue(api.Handle)
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	if len(api.Parameters) > 0 {
		b, _ := json.Marshal(api.Parameters)
		data.Parameters = jsonStringValue(b)
	}
	if len(api.Returns) > 0 {
		b, _ := json.Marshal(api.Returns)
		data.Returns = jsonStringValue(b)
	} else {
		data.Returns = types.StringNull()
	}
	if len(api.Metadata) > 0 {
		b, _ := json.Marshal(api.Metadata)
		data.Metadata = jsonStringValue(b)
	} else {
		data.Metadata = types.StringNull()
	}
	data.Enabled = types.BoolValue(api.Enabled)
	data.TenantID = types.StringValue(api.TenantID)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

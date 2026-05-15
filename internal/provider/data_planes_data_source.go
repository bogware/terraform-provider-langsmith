// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &DataPlanesDataSource{}

func NewDataPlanesDataSource() datasource.DataSource {
	return &DataPlanesDataSource{}
}

type DataPlanesDataSource struct {
	client *client.Client
}

type DataPlanesDataSourceModel struct {
	DataPlanes types.List `tfsdk:"data_planes"`
}

type dataPlaneAPI struct {
	APIURL     string          `json:"api_url"`
	CreatedAt  string          `json:"created_at"`
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Region     string          `json:"region"`
	Status     json.RawMessage `json:"status"`
	Workspaces json.RawMessage `json:"workspaces"`
}

type dataPlanesAPIResponse struct {
	DataPlanes []dataPlaneAPI `json:"data_planes"`
}

var dataPlaneObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"api_url":    types.StringType,
	"region":     types.StringType,
	"status":     types.StringType,
	"workspaces": types.StringType,
	"created_at": types.StringType,
}}

func (d *DataPlanesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_planes"
}

func (d *DataPlanesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the hybrid/self-hosted data planes registered for the current organization.",
		Attributes: map[string]schema.Attribute{
			"data_planes": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true},
						"name":       schema.StringAttribute{Computed: true},
						"api_url":    schema.StringAttribute{Computed: true},
						"region":     schema.StringAttribute{Computed: true},
						"status":     schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded status object."},
						"workspaces": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded list of workspaces attached to this data plane."},
						"created_at": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *DataPlanesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DataPlanesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataPlanesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api dataPlanesAPIResponse
	if err := d.client.Get(ctx, "/orgs/current/data-planes", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error listing data planes", err.Error())
		return
	}
	elems := make([]attr.Value, 0, len(api.DataPlanes))
	for _, dp := range api.DataPlanes {
		statusJSON := ""
		if len(dp.Status) > 0 {
			statusJSON = string(dp.Status)
		}
		workspacesJSON := ""
		if len(dp.Workspaces) > 0 {
			workspacesJSON = string(dp.Workspaces)
		}
		obj, diags := types.ObjectValue(dataPlaneObjectType.AttrTypes, map[string]attr.Value{
			"id":         types.StringValue(dp.ID),
			"name":       types.StringValue(dp.Name),
			"api_url":    types.StringValue(dp.APIURL),
			"region":     types.StringValue(dp.Region),
			"status":     types.StringValue(statusJSON),
			"workspaces": types.StringValue(workspacesJSON),
			"created_at": types.StringValue(dp.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(dataPlaneObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.DataPlanes = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

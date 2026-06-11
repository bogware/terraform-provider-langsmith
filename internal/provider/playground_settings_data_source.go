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

var _ datasource.DataSource = &PlaygroundSettingsDataSource{}

func NewPlaygroundSettingsDataSource() datasource.DataSource {
	return &PlaygroundSettingsDataSource{}
}

// PlaygroundSettingsDataSource lists all saved playground settings in the
// workspace. It shares the langsmith_playground_settings type name with the
// resource (data sources and resources live in separate Terraform namespaces).
type PlaygroundSettingsDataSource struct {
	client *client.Client
}

type PlaygroundSettingsDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Settings    types.List   `tfsdk:"settings"`
}

var playgroundSettingObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":            types.StringType,
	"name":          types.StringType,
	"description":   types.StringType,
	"settings":      types.StringType,
	"options":       types.StringType,
	"settings_type": types.StringType,
	"created_at":    types.StringType,
	"updated_at":    types.StringType,
}}

func (d *PlaygroundSettingsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_playground_settings"
}

func (d *PlaygroundSettingsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all saved LangSmith playground settings in the workspace.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"settings": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Computed: true},
						"name":          schema.StringAttribute{Computed: true},
						"description":   schema.StringAttribute{Computed: true},
						"settings":      schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded settings object."},
						"options":       schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded options object."},
						"settings_type": schema.StringAttribute{Computed: true},
						"created_at":    schema.StringAttribute{Computed: true},
						"updated_at":    schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *PlaygroundSettingsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PlaygroundSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PlaygroundSettingsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []playgroundSettingsAPIResponse
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/playground-settings", nil, &results); err != nil {
		resp.Diagnostics.AddError("Error listing playground settings", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for _, ps := range results {
		name := types.StringNull()
		if ps.Name != nil {
			name = types.StringValue(*ps.Name)
		}
		description := types.StringNull()
		if ps.Description != nil {
			description = types.StringValue(*ps.Description)
		}
		obj, diags := types.ObjectValue(playgroundSettingObjectType.AttrTypes, map[string]attr.Value{
			"id":            types.StringValue(ps.ID),
			"name":          name,
			"description":   description,
			"settings":      jsonStringValue(ps.Settings),
			"options":       jsonStringValue(ps.Options),
			"settings_type": types.StringValue(ps.SettingsType),
			"created_at":    types.StringValue(ps.CreatedAt),
			"updated_at":    types.StringValue(ps.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(playgroundSettingObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Settings = list

	tflog.Trace(ctx, "read playground settings data source", map[string]interface{}{"count": len(results)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

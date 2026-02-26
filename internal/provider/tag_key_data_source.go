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

var _ datasource.DataSource = &TagKeyDataSource{}

func NewTagKeyDataSource() datasource.DataSource {
	return &TagKeyDataSource{}
}

type TagKeyDataSource struct {
	client *client.Client
}

type TagKeyDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Key         types.String `tfsdk:"key"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (d *TagKeyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_key"
}

func (d *TagKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith tag key by ID or key name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier. Either `id` or `key` must be specified.",
				Optional:            true, Computed: true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The tag key name. Either `id` or `key` must be specified.",
				Optional:            true, Computed: true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the tag key.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.", Computed: true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.", Computed: true,
			},
		},
	}
}

func (d *TagKeyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TagKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	keySet := !data.Key.IsNull() && !data.Key.IsUnknown()

	if !idSet && !keySet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"key\" must be specified.")
		return
	}

	if idSet {
		var result tagKeyAPIResponse
		err := d.client.Get(ctx, "/api/v1/workspaces/current/tag-keys/"+data.ID.ValueString(), nil, &result)
		if err != nil {
			resp.Diagnostics.AddError("Error reading tag key", err.Error())
			return
		}
		mapTagKeyDataSourceResponse(&data, &result)
	} else {
		var results []tagKeyAPIResponse
		err := d.client.Get(ctx, "/api/v1/workspaces/current/tag-keys", nil, &results)
		if err != nil {
			resp.Diagnostics.AddError("Error reading tag keys", err.Error())
			return
		}

		var found *tagKeyAPIResponse
		for i := range results {
			if results[i].Key == data.Key.ValueString() {
				found = &results[i]
				break
			}
		}

		if found == nil {
			resp.Diagnostics.AddError("Tag Key Not Found", fmt.Sprintf("No tag key found with key %q.", data.Key.ValueString()))
			return
		}
		mapTagKeyDataSourceResponse(&data, found)
	}

	tflog.Trace(ctx, "read tag key data source", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapTagKeyDataSourceResponse(data *TagKeyDataSourceModel, result *tagKeyAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Key = types.StringValue(result.Key)
	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

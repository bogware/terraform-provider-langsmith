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

var _ datasource.DataSource = &TagValueDataSource{}

func NewTagValueDataSource() datasource.DataSource {
	return &TagValueDataSource{}
}

type TagValueDataSource struct {
	client *client.Client
}

type TagValueDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	TagKeyID    types.String `tfsdk:"tag_key_id"`
	Value       types.String `tfsdk:"value"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

func (d *TagValueDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_value"
}

func (d *TagValueDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith tag value within a tag key, by ID or by value name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the tag value. Either `id` or `value` must be specified.",
				Optional:            true, Computed: true,
			},
			"tag_key_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the parent tag key.",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The tag value. Either `id` or `value` must be specified.",
				Optional:            true, Computed: true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the tag value.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.", Computed: true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.", Computed: true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *TagValueDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TagValueDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagValueDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	valueSet := !data.Value.IsNull() && !data.Value.IsUnknown()

	if !idSet && !valueSet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"value\" must be specified.")
		return
	}

	basePath := fmt.Sprintf("/api/v1/workspaces/current/tag-keys/%s/tag-values", data.TagKeyID.ValueString())

	if idSet {
		var result tagValueAPIResponse
		err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, basePath+"/"+data.ID.ValueString(), nil, &result)
		if err != nil {
			if client.IsNotFound(err) {
				resp.Diagnostics.AddError("Tag Value Not Found", fmt.Sprintf("No tag value found with ID %q under tag key %q.", data.ID.ValueString(), data.TagKeyID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("Error reading tag value", err.Error())
			return
		}
		mapTagValueDataSourceResponse(&data, &result)
	} else {
		var results []tagValueAPIResponse
		err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, basePath, nil, &results)
		if err != nil {
			if client.IsNotFound(err) {
				resp.Diagnostics.AddError("Tag Key Not Found", fmt.Sprintf("No tag key found with ID %q.", data.TagKeyID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("Error listing tag values", err.Error())
			return
		}

		var found *tagValueAPIResponse
		for i := range results {
			if results[i].Value == data.Value.ValueString() {
				found = &results[i]
				break
			}
		}

		if found == nil {
			resp.Diagnostics.AddError("Tag Value Not Found", fmt.Sprintf("No tag value found with value %q under tag key %q.", data.Value.ValueString(), data.TagKeyID.ValueString()))
			return
		}
		mapTagValueDataSourceResponse(&data, found)
	}

	tflog.Trace(ctx, "read tag value data source", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapTagValueDataSourceResponse(data *TagValueDataSourceModel, result *tagValueAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Value = types.StringValue(result.Value)
	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

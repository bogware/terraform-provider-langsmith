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

var _ datasource.DataSource = &HubEnvironmentsDataSource{}

// NewHubEnvironmentsDataSource returns a new HubEnvironmentsDataSource for
// reading the workspace's prompt-hub environment list.
func NewHubEnvironmentsDataSource() datasource.DataSource {
	return &HubEnvironmentsDataSource{}
}

// HubEnvironmentsDataSource reads the per-workspace prompt-hub environments
// record (a single record holding all environments).
type HubEnvironmentsDataSource struct {
	client *client.Client
}

// HubEnvironmentsDataSourceModel holds the read-only attributes for the hub
// environments lookup.
type HubEnvironmentsDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Environments types.List   `tfsdk:"environments"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
}

func (d *HubEnvironmentsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_environments"
}

func (d *HubEnvironmentsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to read the workspace's prompt-hub environment list. The API exposes one per-workspace record holding all environments.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the hub environments record.",
				Computed:            true,
			},
			"environments": schema.ListNestedAttribute{
				MarkdownDescription: "The list of hub environment entries.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Environment name.",
							Computed:            true,
						},
					},
				},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *HubEnvironmentsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *HubEnvironmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data HubEnvironmentsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api hubEnvAPI
	err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/hub/environments", nil, &api)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"Hub Environments Not Found",
				"No hub environments record was found for this workspace.",
			)
			return
		}
		resp.Diagnostics.AddError("Error reading hub environments", err.Error())
		return
	}

	if api.ID != "" {
		data.ID = types.StringValue(api.ID)
	} else {
		data.ID = types.StringNull()
	}

	data.Environments = buildHubEnvList(api.Environments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read hub environments data source", map[string]interface{}{"id": api.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

var _ datasource.DataSource = &OrganizationDataSource{}

// NewOrganizationDataSource returns a new OrganizationDataSource.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

// OrganizationDataSource defines the data source implementation.
type OrganizationDataSource struct {
	client *client.Client
}

// OrganizationDataSourceModel describes the data source data model.
type OrganizationDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	IsPersonal  types.Bool   `tfsdk:"is_personal"`
	Tier        types.String `tfsdk:"tier"`
}

// orgDataSourceAPIResponse is the API response for the org endpoint.
type orgDataSourceAPIResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	IsPersonal  bool   `json:"is_personal"`
	Tier        string `json:"tier"`
}

func (d *OrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *OrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to retrieve information about the current LangSmith organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the organization.",
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the organization.",
				Computed:            true,
			},
			"is_personal": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a personal organization.",
				Computed:            true,
			},
			"tier": schema.StringAttribute{
				MarkdownDescription: "The plan tier of the organization (e.g., `free`, `developer`, `plus`, `enterprise`).",
				Computed:            true,
			},
		},
	}
}

func (d *OrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result orgDataSourceAPIResponse
	err := d.client.Get(ctx, "/api/v1/orgs/current", nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.IsPersonal = types.BoolValue(result.IsPersonal)
	data.Tier = types.StringValue(result.Tier)

	tflog.Trace(ctx, "read organization data source", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

var _ datasource.DataSource = &ChartSectionDataSource{}

func NewChartSectionDataSource() datasource.DataSource {
	return &ChartSectionDataSource{}
}

type ChartSectionDataSource struct {
	client *client.Client
}

type ChartSectionDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Index       types.Int64  `tfsdk:"index"`
	ChartCount  types.Int64  `tfsdk:"chart_count"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

// chartSectionListAPIResponse matches CustomChartsSectionResponse.
type chartSectionListAPIResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Index       *int64  `json:"index"`
	ChartCount  *int64  `json:"chart_count"`
	CreatedAt   *string `json:"created_at"`
	ModifiedAt  *string `json:"modified_at"`
}

func (d *ChartSectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chart_section"
}

func (d *ChartSectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith custom chart section by ID or title. Backed by `GET /api/v1/charts/section`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the section. Either `id` or `title` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "The title of the section. Either `id` or `title` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the section.",
				Computed:            true,
			},
			"index": schema.Int64Attribute{
				MarkdownDescription: "The display order index.",
				Computed:            true,
			},
			"chart_count": schema.Int64Attribute{
				MarkdownDescription: "Number of charts in the section.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (d *ChartSectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ChartSectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ChartSectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	titleSet := !data.Title.IsNull() && !data.Title.IsUnknown()

	if !idSet && !titleSet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"title\" must be specified.")
		return
	}

	var sections []chartSectionListAPIResponse
	err := d.client.Get(ctx, "/api/v1/charts/section", nil, &sections)
	if err != nil {
		resp.Diagnostics.AddError("Error listing chart sections", err.Error())
		return
	}

	var found *chartSectionListAPIResponse
	for i := range sections {
		if idSet && sections[i].ID == data.ID.ValueString() {
			found = &sections[i]
			break
		}
		if titleSet && sections[i].Title == data.Title.ValueString() {
			found = &sections[i]
			break
		}
	}

	if found == nil {
		if idSet {
			resp.Diagnostics.AddError("Chart Section Not Found", fmt.Sprintf("No chart section found with ID %q.", data.ID.ValueString()))
		} else {
			resp.Diagnostics.AddError("Chart Section Not Found", fmt.Sprintf("No chart section found with title %q.", data.Title.ValueString()))
		}
		return
	}

	data.ID = types.StringValue(found.ID)
	data.Title = types.StringValue(found.Title)
	setStateOptionalString(&data.Description, found.Description)
	setStateOptionalString(&data.CreatedAt, found.CreatedAt)
	setStateOptionalString(&data.UpdatedAt, found.ModifiedAt)
	if found.Index != nil {
		data.Index = types.Int64Value(*found.Index)
	} else {
		data.Index = types.Int64Null()
	}
	if found.ChartCount != nil {
		data.ChartCount = types.Int64Value(*found.ChartCount)
	} else {
		data.ChartCount = types.Int64Null()
	}

	tflog.Trace(ctx, "read chart section data source", map[string]interface{}{"id": found.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

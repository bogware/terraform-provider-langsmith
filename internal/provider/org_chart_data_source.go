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

var _ datasource.DataSource = &OrgChartDataSource{}

func NewOrgChartDataSource() datasource.DataSource {
	return &OrgChartDataSource{}
}

type OrgChartDataSource struct {
	client *client.Client
}

func (d *OrgChartDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_chart"
}

func (d *OrgChartDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith organization-scoped custom chart by ID. Note: the LangSmith single-chart read endpoint does not return `section_id`; if you need to reference a chart's section, manage it via the `langsmith_org_chart` resource instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the org chart.",
				Required:            true,
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "The title of the chart.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the chart.",
				Computed:            true,
			},
			"index": schema.Int64Attribute{
				MarkdownDescription: "The display order index.",
				Computed:            true,
			},
			"chart_type": schema.StringAttribute{
				MarkdownDescription: "The chart type (`line` or `bar`).",
				Computed:            true,
			},
			"series": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of chart series configurations.",
				Computed:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded metadata object.",
				Computed:            true,
			},
			"common_filters": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded common filter configuration.",
				Computed:            true,
			},
		},
	}
}

func (d *OrgChartDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrgChartDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ChartDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	var result chartSingleAPIResponse
	err := d.client.Post(ctx, "/api/v1/org-charts/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading org chart", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Title = types.StringValue(result.Title)
	data.Index = types.Int64Value(result.Index)
	data.ChartType = types.StringValue(result.ChartType)
	data.Series = jsonStringValue(result.Series)
	data.Metadata = jsonStringValue(result.Metadata)
	data.CommonFilters = jsonStringValue(result.CommonFilters)
	setStateOptionalString(&data.Description, result.Description)

	tflog.Trace(ctx, "read org chart data source", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

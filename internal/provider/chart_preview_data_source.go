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
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &ChartPreviewDataSource{}

func NewChartPreviewDataSource() datasource.DataSource {
	return &ChartPreviewDataSource{}
}

type ChartPreviewDataSource struct {
	client *client.Client
}

type ChartPreviewDataSourceModel struct {
	Series        types.String `tfsdk:"series"`
	CommonFilters types.String `tfsdk:"common_filters"`
	StartTime     types.String `tfsdk:"start_time"`
	EndTime       types.String `tfsdk:"end_time"`
	Stride        types.String `tfsdk:"stride"`
	Timezone      types.String `tfsdk:"timezone"`
	Data          types.String `tfsdk:"data"`
}

type chartPreviewBucketInfo struct {
	Timezone  *string          `json:"timezone,omitempty"`
	StartTime *string          `json:"start_time,omitempty"`
	EndTime   *string          `json:"end_time,omitempty"`
	Stride    *json.RawMessage `json:"stride,omitempty"`
	OmitData  bool             `json:"omit_data"`
}

type chartPreviewChartBody struct {
	Series        json.RawMessage  `json:"series"`
	CommonFilters *json.RawMessage `json:"common_filters,omitempty"`
}

type chartPreviewRequest struct {
	BucketInfo chartPreviewBucketInfo `json:"bucket_info"`
	Chart      chartPreviewChartBody  `json:"chart"`
}

type chartPreviewResponse struct {
	Data json.RawMessage `json:"data"`
}

func chartPreviewSchema(orgScope bool) schema.Schema {
	scope := "workspace"
	endpoint := "/api/v1/charts/preview"
	scopeNote := " Workspace-scoped previews require each series to include a `filters.session` array (project IDs) — the API rejects requests without one with a 422."
	if orgScope {
		scope = "organization"
		endpoint = "/api/v1/org-charts/preview"
		scopeNote = ""
	}
	return schema.Schema{
		MarkdownDescription: fmt.Sprintf(
			"Computes preview data points for a hypothetical %s-scoped chart via `POST %s` without persisting it. "+
				"Returns the raw data point series as JSON.%s "+
				"**Note:** the result depends on the current state of LangSmith data, so consecutive plans may return different values; "+
				"this data source is intended for inspection/debugging rather than long-lived Terraform state.",
			scope, endpoint, scopeNote),
		Attributes: map[string]schema.Attribute{
			"series": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of chart series configurations. **Each series must include an `id`** (any UUID — preview series IDs have no persistence; the API requires the field for request validation).",
				Required:            true,
			},
			"common_filters": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded common filter configuration applied to all series.",
				Optional:            true,
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "RFC3339 start timestamp for the preview window.",
				Optional:            true,
			},
			"end_time": schema.StringAttribute{
				MarkdownDescription: "RFC3339 end timestamp for the preview window.",
				Optional:            true,
			},
			"stride": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded `TimedeltaInput` for bucket width (e.g. `jsonencode({minutes = 15})`). Defaults to 15 minutes server-side.",
				Optional:            true,
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "IANA timezone for bucket alignment. Defaults to `UTC` server-side.",
				Optional:            true,
			},
			"data": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of preview data points, each shaped as `{series_id, timestamp, value, group}`.",
				Computed:            true,
			},
		},
	}
}

func (d *ChartPreviewDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chart_preview"
}

func (d *ChartPreviewDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = chartPreviewSchema(false)
}

func (d *ChartPreviewDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ChartPreviewDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	readChartPreview(ctx, d.client, "/api/v1/charts/preview", "chart preview", req, resp)
}

func readChartPreview(ctx context.Context, c *client.Client, endpoint, label string, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ChartPreviewDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := chartPreviewRequest{
		BucketInfo: chartPreviewBucketInfo{OmitData: false},
		Chart:      chartPreviewChartBody{Series: json.RawMessage(data.Series.ValueString())},
	}
	setOptionalString(&body.BucketInfo.StartTime, data.StartTime)
	setOptionalString(&body.BucketInfo.EndTime, data.EndTime)
	setOptionalString(&body.BucketInfo.Timezone, data.Timezone)
	if !data.Stride.IsNull() && !data.Stride.IsUnknown() {
		raw := json.RawMessage(data.Stride.ValueString())
		body.BucketInfo.Stride = &raw
	}
	if !data.CommonFilters.IsNull() && !data.CommonFilters.IsUnknown() {
		raw := json.RawMessage(data.CommonFilters.ValueString())
		body.Chart.CommonFilters = &raw
	}

	var result chartPreviewResponse
	if err := c.Post(ctx, endpoint, body, &result); err != nil {
		resp.Diagnostics.AddError("Error fetching "+label, err.Error())
		return
	}

	data.Data = jsonStringValue(result.Data)
	tflog.Trace(ctx, "read "+label+" data source", nil)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

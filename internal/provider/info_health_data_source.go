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

var _ datasource.DataSource = &InfoHealthDataSource{}

// NewInfoHealthDataSource returns a data source reporting instance health.
func NewInfoHealthDataSource() datasource.DataSource {
	return &InfoHealthDataSource{}
}

// InfoHealthDataSource reads GET /api/v1/info/health.
type InfoHealthDataSource struct {
	client *client.Client
}

// InfoHealthDataSourceModel maps the Terraform schema.
type InfoHealthDataSourceModel struct {
	ClickhouseDiskFreePct types.Float64 `tfsdk:"clickhouse_disk_free_pct"`
}

type infoHealthAPIResponse struct {
	ClickhouseDiskFreePct *float64 `json:"clickhouse_disk_free_pct"`
}

func (d *InfoHealthDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_info_health"
}

func (d *InfoHealthDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reports health of the LangSmith instance. Most useful on a self-hosted deployment, where storage headroom is the operator's problem — on Cloud the value reflects infrastructure you do not manage.\n\n" +
			"This is a point-in-time reading taken during `terraform plan` and `apply`; it is not a monitor, and Terraform will not re-evaluate it between runs.",
		Attributes: map[string]schema.Attribute{
			"clickhouse_disk_free_pct": schema.Float64Attribute{
				MarkdownDescription: "Percentage of ClickHouse disk still free. Null when the deployment does not report it.",
				Computed:            true,
			},
		},
	}
}

func (d *InfoHealthDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *InfoHealthDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data InfoHealthDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api infoHealthAPIResponse
	if err := d.client.Get(ctx, "/api/v1/info/health", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error reading instance health", err.Error())
		return
	}

	if api.ClickhouseDiskFreePct != nil {
		data.ClickhouseDiskFreePct = types.Float64Value(*api.ClickhouseDiskFreePct)
	} else {
		data.ClickhouseDiskFreePct = types.Float64Null()
	}

	tflog.Trace(ctx, "read instance health", nil)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

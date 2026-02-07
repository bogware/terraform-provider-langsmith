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

var _ datasource.DataSource = &InfoDataSource{}

// NewInfoDataSource returns a new InfoDataSource for checking the lay of the land --
// server version, license status, and ingest configuration.
func NewInfoDataSource() datasource.DataSource {
	return &InfoDataSource{}
}

// InfoDataSource retrieves LangSmith server information from the /info endpoint.
// Takes no inputs -- just rides into town and asks what is going on.
type InfoDataSource struct {
	client *client.Client
}

// InfoDataSourceModel holds the server intel: version string, license expiration,
// batch ingest configuration, and instance flags -- everything you need to know
// before riding into town.
type InfoDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Version               types.String `tfsdk:"version"`
	LicenseExpirationTime types.String `tfsdk:"license_expiration_time"`
	BatchIngestConfig     types.String `tfsdk:"batch_ingest_config"`
	InstanceFlags         types.String `tfsdk:"instance_flags"`
}

// infoDataSourceAPIResponse is the API response for the info endpoint.
type infoDataSourceAPIResponse struct {
	Version               string          `json:"version"`
	LicenseExpirationTime *string         `json:"license_expiration_time"`
	BatchIngestConfig     json.RawMessage `json:"batch_ingest_config"`
	InstanceFlags         json.RawMessage `json:"instance_flags"`
}

func (d *InfoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_info"
}

func (d *InfoDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to retrieve LangSmith server information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Placeholder identifier, always set to `info`.",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "The LangSmith server version.",
				Computed:            true,
			},
			"license_expiration_time": schema.StringAttribute{
				MarkdownDescription: "The license expiration time of the LangSmith instance.",
				Computed:            true,
			},
			"batch_ingest_config": schema.StringAttribute{
				MarkdownDescription: "JSON string of the batch ingest configuration.",
				Computed:            true,
			},
			"instance_flags": schema.StringAttribute{
				MarkdownDescription: "JSON string of instance feature flags.",
				Computed:            true,
			},
		},
	}
}

func (d *InfoDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *InfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data InfoDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result infoDataSourceAPIResponse
	err := d.client.Get(ctx, "/api/v1/info", nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading LangSmith info", err.Error())
		return
	}

	data.ID = types.StringValue("info")
	data.Version = types.StringValue(result.Version)

	if result.LicenseExpirationTime != nil {
		data.LicenseExpirationTime = types.StringValue(*result.LicenseExpirationTime)
	} else {
		data.LicenseExpirationTime = types.StringNull()
	}

	if len(result.BatchIngestConfig) > 0 && string(result.BatchIngestConfig) != "null" {
		data.BatchIngestConfig = types.StringValue(string(result.BatchIngestConfig))
	} else {
		data.BatchIngestConfig = types.StringNull()
	}

	if len(result.InstanceFlags) > 0 && string(result.InstanceFlags) != "null" {
		data.InstanceFlags = types.StringValue(string(result.InstanceFlags))
	} else {
		data.InstanceFlags = types.StringNull()
	}

	tflog.Trace(ctx, "read info data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

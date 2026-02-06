// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &DatasetDataSource{}

// NewDatasetDataSource returns a new DatasetDataSource for tracking down
// an existing LangSmith dataset by ID or name.
func NewDatasetDataSource() datasource.DataSource {
	return &DatasetDataSource{}
}

// DatasetDataSource reads a LangSmith dataset by ID or name, returning its
// metadata without altering a single record. A peaceful visit to the ranch.
type DatasetDataSource struct {
	client *client.Client
}

// DatasetDataSourceModel holds the read-only attributes for a dataset lookup:
// name, description, data type, and the tally of examples it contains.
type DatasetDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	DataType     types.String `tfsdk:"data_type"`
	TenantID     types.String `tfsdk:"tenant_id"`
	CreatedAt    types.String `tfsdk:"created_at"`
	ExampleCount types.Int64  `tfsdk:"example_count"`
}

// datasetDataSourceAPIResponse is the API response for a dataset lookup.
type datasetDataSourceAPIResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	DataType     string  `json:"data_type"`
	TenantID     string  `json:"tenant_id"`
	CreatedAt    string  `json:"created_at"`
	ExampleCount int64   `json:"example_count"`
}

func (d *DatasetDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (d *DatasetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith dataset by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the dataset. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the dataset. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the dataset.",
				Computed:            true,
			},
			"data_type": schema.StringAttribute{
				MarkdownDescription: "The data type of the dataset (e.g., `kv`, `llm`, or `chat`).",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID of the dataset.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the dataset.",
				Computed:            true,
			},
			"example_count": schema.Int64Attribute{
				MarkdownDescription: "The number of examples in the dataset.",
				Computed:            true,
			},
		},
	}
}

func (d *DatasetDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DatasetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either \"id\" or \"name\" must be specified to look up a dataset.",
		)
		return
	}

	var result datasetDataSourceAPIResponse

	if idSet {
		err := d.client.Get(ctx, "/api/v1/datasets/"+data.ID.ValueString(), nil, &result)
		if err != nil {
			resp.Diagnostics.AddError("Error reading dataset", err.Error())
			return
		}
	} else {
		query := url.Values{}
		query.Set("name", data.Name.ValueString())

		var results []datasetDataSourceAPIResponse
		err := d.client.Get(ctx, "/api/v1/datasets", query, &results)
		if err != nil {
			resp.Diagnostics.AddError("Error reading dataset", err.Error())
			return
		}

		if len(results) == 0 {
			resp.Diagnostics.AddError(
				"Dataset Not Found",
				fmt.Sprintf("No dataset found with name %q.", data.Name.ValueString()),
			)
			return
		}

		result = results[0]
	}

	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.DataType = types.StringValue(result.DataType)
	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.ExampleCount = types.Int64Value(result.ExampleCount)

	tflog.Trace(ctx, "read dataset data source", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

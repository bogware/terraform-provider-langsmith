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

var _ datasource.DataSource = &ExampleDataSource{}

// NewExampleDataSource returns a new ExampleDataSource for looking up an
// existing dataset example by ID.
func NewExampleDataSource() datasource.DataSource {
	return &ExampleDataSource{}
}

// ExampleDataSource reads a single LangSmith dataset example by ID without
// modifying it.
type ExampleDataSource struct {
	client *client.Client
}

// ExampleDataSourceModel holds the read-only attributes for an example lookup.
type ExampleDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DatasetID   types.String `tfsdk:"dataset_id"`
	Inputs      types.String `tfsdk:"inputs"`
	Outputs     types.String `tfsdk:"outputs"`
	Metadata    types.String `tfsdk:"metadata"`
	SourceRunID types.String `tfsdk:"source_run_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	ModifiedAt  types.String `tfsdk:"modified_at"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

func (d *ExampleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_example"
}

func (d *ExampleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith dataset example by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the example.",
				Required:            true,
			},
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the dataset this example belongs to.",
				Computed:            true,
			},
			"inputs": schema.StringAttribute{
				MarkdownDescription: "JSON string containing the input data for the example.",
				Computed:            true,
			},
			"outputs": schema.StringAttribute{
				MarkdownDescription: "JSON string containing the output data for the example.",
				Computed:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON string containing metadata for the example.",
				Computed:            true,
			},
			"source_run_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the source run for this example.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the example.",
				Computed:            true,
			},
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "The last modification timestamp of the example.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *ExampleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ExampleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ExampleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result exampleAPIResponse
	err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/examples/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"Example Not Found",
				fmt.Sprintf("No example found with ID %q.", data.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading example", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.DatasetID = types.StringValue(result.DatasetID)
	data.Inputs = jsonStringValue(result.Inputs)
	data.Outputs = jsonStringValue(result.Outputs)
	data.Metadata = jsonStringValue(result.Metadata)

	if result.SourceRunID != nil {
		data.SourceRunID = types.StringValue(*result.SourceRunID)
	} else {
		data.SourceRunID = types.StringNull()
	}

	data.CreatedAt = types.StringValue(result.CreatedAt)

	if result.ModifiedAt != "" {
		data.ModifiedAt = types.StringValue(result.ModifiedAt)
	} else {
		data.ModifiedAt = types.StringNull()
	}

	tflog.Trace(ctx, "read example data source", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

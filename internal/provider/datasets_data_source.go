// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// datasetsPageSize is the page size used when listing datasets; the API caps
// `limit` at 100, so we page until a short page signals the end.
const datasetsPageSize = 100

var _ datasource.DataSource = &DatasetsDataSource{}

// NewDatasetsDataSource returns a new DatasetsDataSource that lists datasets
// in a workspace, with optional name and data type filters.
func NewDatasetsDataSource() datasource.DataSource {
	return &DatasetsDataSource{}
}

// DatasetsDataSource lists LangSmith datasets. It pages through
// GET /api/v1/datasets until every matching dataset is collected.
type DatasetsDataSource struct {
	client *client.Client
}

// DatasetsDataSourceModel holds the filter inputs and the resulting datasets list.
type DatasetsDataSourceModel struct {
	Name         types.String `tfsdk:"name"`
	NameContains types.String `tfsdk:"name_contains"`
	DataType     types.String `tfsdk:"data_type"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Datasets     types.List   `tfsdk:"datasets"`
}

// datasetListAPI mirrors the Dataset schema returned by GET /api/v1/datasets.
// The list response carries tenant_id; workspace_id is also decoded in case
// the API supplies it.
type datasetListAPI struct {
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Description             *string         `json:"description"`
	DataType                string          `json:"data_type"`
	InputsSchemaDefinition  json.RawMessage `json:"inputs_schema_definition"`
	OutputsSchemaDefinition json.RawMessage `json:"outputs_schema_definition"`
	ExternallyManaged       *bool           `json:"externally_managed"`
	Transformations         json.RawMessage `json:"transformations"`
	Metadata                json.RawMessage `json:"metadata"`
	WorkspaceID             string          `json:"workspace_id"`
	TenantID                string          `json:"tenant_id"`
	CreatedAt               string          `json:"created_at"`
	ModifiedAt              string          `json:"modified_at"`
	ExampleCount            *int64          `json:"example_count"`
	SessionCount            *int64          `json:"session_count"`
	LastSessionStartTime    *string         `json:"last_session_start_time"`
}

var datasetObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                        types.StringType,
	"name":                      types.StringType,
	"description":               types.StringType,
	"data_type":                 types.StringType,
	"inputs_schema_definition":  types.StringType,
	"outputs_schema_definition": types.StringType,
	"externally_managed":        types.BoolType,
	"transformations":           types.StringType,
	"metadata":                  types.StringType,
	"workspace_id":              types.StringType,
	"created_at":                types.StringType,
	"modified_at":               types.StringType,
	"example_count":             types.Int64Type,
	"session_count":             types.Int64Type,
	"last_session_start_time":   types.StringType,
}}

func (d *DatasetsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasets"
}

func (d *DatasetsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith datasets in a workspace, optionally filtered by name or data type.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Return only the dataset with this exact name.",
				Optional:            true,
			},
			"name_contains": schema.StringAttribute{
				MarkdownDescription: "Return only datasets whose name contains this substring.",
				Optional:            true,
			},
			"data_type": schema.StringAttribute{
				MarkdownDescription: "Return only datasets of this data type (`kv`, `llm`, or `chat`).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("kv", "llm", "chat"),
				},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list datasets from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"datasets": schema.ListNestedAttribute{
				MarkdownDescription: "The matching datasets.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the dataset.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the dataset.",
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
						"inputs_schema_definition": schema.StringAttribute{
							MarkdownDescription: "JSON string of the inputs JSON schema definition.",
							Computed:            true,
						},
						"outputs_schema_definition": schema.StringAttribute{
							MarkdownDescription: "JSON string of the outputs JSON schema definition.",
							Computed:            true,
						},
						"externally_managed": schema.BoolAttribute{
							MarkdownDescription: "Whether the dataset is externally managed.",
							Computed:            true,
						},
						"transformations": schema.StringAttribute{
							MarkdownDescription: "JSON string of the dataset transformations.",
							Computed:            true,
						},
						"metadata": schema.StringAttribute{
							MarkdownDescription: "JSON string of the dataset metadata.",
							Computed:            true,
						},
						"workspace_id": schema.StringAttribute{
							MarkdownDescription: "The workspace ID that owns the dataset.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the dataset.",
							Computed:            true,
						},
						"modified_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the dataset.",
							Computed:            true,
						},
						"example_count": schema.Int64Attribute{
							MarkdownDescription: "The number of examples in the dataset.",
							Computed:            true,
						},
						"session_count": schema.Int64Attribute{
							MarkdownDescription: "The number of sessions associated with the dataset.",
							Computed:            true,
						},
						"last_session_start_time": schema.StringAttribute{
							MarkdownDescription: "The start time of the last session associated with the dataset.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *DatasetsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatasetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DatasetsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var all []datasetListAPI
	for offset := 0; ; offset += datasetsPageSize {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(datasetsPageSize))
		query.Set("offset", strconv.Itoa(offset))
		if !data.Name.IsNull() && !data.Name.IsUnknown() {
			query.Set("name", data.Name.ValueString())
		}
		if !data.NameContains.IsNull() && !data.NameContains.IsUnknown() {
			query.Set("name_contains", data.NameContains.ValueString())
		}
		if !data.DataType.IsNull() && !data.DataType.IsUnknown() {
			query.Set("data_type", data.DataType.ValueString())
		}

		var page []datasetListAPI
		if err := c.Get(ctx, "/api/v1/datasets", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing datasets", err.Error())
			return
		}
		all = append(all, page...)
		if len(page) < datasetsPageSize {
			break
		}
	}

	elems := make([]attr.Value, 0, len(all))
	for _, ds := range all {
		description := types.StringNull()
		if ds.Description != nil {
			description = types.StringValue(*ds.Description)
		}
		externallyManaged := types.BoolNull()
		if ds.ExternallyManaged != nil {
			externallyManaged = types.BoolValue(*ds.ExternallyManaged)
		}
		workspaceID := ds.WorkspaceID
		if workspaceID == "" {
			workspaceID = ds.TenantID
		}
		exampleCount := types.Int64Null()
		if ds.ExampleCount != nil {
			exampleCount = types.Int64Value(*ds.ExampleCount)
		}
		sessionCount := types.Int64Null()
		if ds.SessionCount != nil {
			sessionCount = types.Int64Value(*ds.SessionCount)
		}
		lastSessionStartTime := types.StringNull()
		if ds.LastSessionStartTime != nil {
			lastSessionStartTime = types.StringValue(*ds.LastSessionStartTime)
		}
		obj, diags := types.ObjectValue(datasetObjectType.AttrTypes, map[string]attr.Value{
			"id":                        types.StringValue(ds.ID),
			"name":                      types.StringValue(ds.Name),
			"description":               description,
			"data_type":                 types.StringValue(ds.DataType),
			"inputs_schema_definition":  jsonStringValue(ds.InputsSchemaDefinition),
			"outputs_schema_definition": jsonStringValue(ds.OutputsSchemaDefinition),
			"externally_managed":        externallyManaged,
			"transformations":           jsonStringValue(ds.Transformations),
			"metadata":                  jsonStringValue(ds.Metadata),
			"workspace_id":              types.StringValue(workspaceID),
			"created_at":                types.StringValue(ds.CreatedAt),
			"modified_at":               types.StringValue(ds.ModifiedAt),
			"example_count":             exampleCount,
			"session_count":             sessionCount,
			"last_session_start_time":   lastSessionStartTime,
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(datasetObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Datasets = list

	tflog.Trace(ctx, "read datasets data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &DatasetResource{}
	_ resource.ResourceWithImportState = &DatasetResource{}
)

// NewDatasetResource constructs a fresh DatasetResource for managing LangSmith
// datasets.
func NewDatasetResource() resource.Resource {
	return &DatasetResource{}
}

// DatasetResource manages a LangSmith dataset — a well-organized stockyard for
// your evaluation data.
type DatasetResource struct {
	client *client.Client
}

// DatasetResourceModel holds the Terraform state for a dataset, from its name
// and schema down to who's managing the herd.
type DatasetResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	DataType                types.String `tfsdk:"data_type"`
	InputsSchemaDefinition  types.String `tfsdk:"inputs_schema_definition"`
	OutputsSchemaDefinition types.String `tfsdk:"outputs_schema_definition"`
	ExternallyManaged       types.Bool   `tfsdk:"externally_managed"`
	Transformations         types.String `tfsdk:"transformations"`
	Metadata                types.String `tfsdk:"metadata"`
	ExampleCount            types.Int64  `tfsdk:"example_count"`
	SessionCount            types.Int64  `tfsdk:"session_count"`
	ModifiedAt              types.String `tfsdk:"modified_at"`
	LastSessionStartTime    types.String `tfsdk:"last_session_start_time"`
	TenantID                types.String `tfsdk:"tenant_id"`
	CreatedAt               types.String `tfsdk:"created_at"`
}

// datasetAPIRequest is the wire format for creating or updating a dataset on
// the LangSmith API.
type datasetAPIRequest struct {
	Name                    string          `json:"name"`
	Description             *string         `json:"description,omitempty"`
	DataType                *string         `json:"data_type,omitempty"`
	InputsSchemaDefinition  json.RawMessage `json:"inputs_schema_definition,omitempty"`
	OutputsSchemaDefinition json.RawMessage `json:"outputs_schema_definition,omitempty"`
	ExternallyManaged       *bool           `json:"externally_managed,omitempty"`
	Transformations         json.RawMessage `json:"transformations,omitempty"`
	Metadata                json.RawMessage `json:"metadata,omitempty"`
}

// datasetAPIResponse is what the LangSmith API sends back about a dataset —
// the full bill of lading.
type datasetAPIResponse struct {
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Description             *string         `json:"description"`
	DataType                string          `json:"data_type"`
	InputsSchemaDefinition  json.RawMessage `json:"inputs_schema_definition"`
	OutputsSchemaDefinition json.RawMessage `json:"outputs_schema_definition"`
	ExternallyManaged       *bool           `json:"externally_managed"`
	Transformations         json.RawMessage `json:"transformations"`
	Metadata                json.RawMessage `json:"metadata"`
	ExampleCount            *int64          `json:"example_count"`
	SessionCount            int64           `json:"session_count"`
	ModifiedAt              string          `json:"modified_at"`
	LastSessionStartTime    *string         `json:"last_session_start_time"`
	TenantID                string          `json:"tenant_id"`
	CreatedAt               string          `json:"created_at"`
}

func (r *DatasetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *DatasetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith dataset.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the dataset.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the dataset.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the dataset.",
				Optional:            true,
			},
			"data_type": schema.StringAttribute{
				MarkdownDescription: "The data type of the dataset. One of `kv`, `llm`, or `chat`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("kv"),
			},
			"inputs_schema_definition": schema.StringAttribute{
				MarkdownDescription: "JSON string defining the inputs schema.",
				Optional:            true,
			},
			"outputs_schema_definition": schema.StringAttribute{
				MarkdownDescription: "JSON string defining the outputs schema.",
				Optional:            true,
			},
			"externally_managed": schema.BoolAttribute{
				MarkdownDescription: "Whether the dataset is externally managed.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"transformations": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of dataset transformations.",
				Optional:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded metadata object for the dataset.",
				Optional:            true,
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
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the dataset was last modified.",
				Computed:            true,
			},
			"last_session_start_time": schema.StringAttribute{
				MarkdownDescription: "The start time of the last session.",
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
		},
	}
}

func (r *DatasetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *DatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := datasetAPIRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.DataType.IsNull() && !data.DataType.IsUnknown() {
		v := data.DataType.ValueString()
		body.DataType = &v
	}
	if !data.InputsSchemaDefinition.IsNull() && !data.InputsSchemaDefinition.IsUnknown() {
		body.InputsSchemaDefinition = json.RawMessage(data.InputsSchemaDefinition.ValueString())
	}
	if !data.OutputsSchemaDefinition.IsNull() && !data.OutputsSchemaDefinition.IsUnknown() {
		body.OutputsSchemaDefinition = json.RawMessage(data.OutputsSchemaDefinition.ValueString())
	}
	if !data.ExternallyManaged.IsNull() && !data.ExternallyManaged.IsUnknown() {
		v := data.ExternallyManaged.ValueBool()
		body.ExternallyManaged = &v
	}
	// Transformations ride into town like a stagecoach full of new instructions.
	if !data.Transformations.IsNull() && !data.Transformations.IsUnknown() {
		body.Transformations = json.RawMessage(data.Transformations.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		body.Metadata = json.RawMessage(data.Metadata.ValueString())
	}

	var result datasetAPIResponse
	err := r.client.Post(ctx, "/api/v1/datasets", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating dataset", err.Error())
		return
	}

	mapDatasetResponseToState(&data, &result)
	tflog.Trace(ctx, "created dataset resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result datasetAPIResponse
	err := r.client.Get(ctx, "/api/v1/datasets/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading dataset", err.Error())
		return
	}

	mapDatasetResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := datasetAPIRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.DataType.IsNull() && !data.DataType.IsUnknown() {
		v := data.DataType.ValueString()
		body.DataType = &v
	}
	if !data.InputsSchemaDefinition.IsNull() && !data.InputsSchemaDefinition.IsUnknown() {
		body.InputsSchemaDefinition = json.RawMessage(data.InputsSchemaDefinition.ValueString())
	}
	if !data.OutputsSchemaDefinition.IsNull() && !data.OutputsSchemaDefinition.IsUnknown() {
		body.OutputsSchemaDefinition = json.RawMessage(data.OutputsSchemaDefinition.ValueString())
	}
	if !data.ExternallyManaged.IsNull() && !data.ExternallyManaged.IsUnknown() {
		v := data.ExternallyManaged.ValueBool()
		body.ExternallyManaged = &v
	}
	// Same stagecoach, different day — keep those transformations moving.
	if !data.Transformations.IsNull() && !data.Transformations.IsUnknown() {
		body.Transformations = json.RawMessage(data.Transformations.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		body.Metadata = json.RawMessage(data.Metadata.ValueString())
	}

	var result datasetAPIResponse
	err := r.client.Patch(ctx, "/api/v1/datasets/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating dataset", err.Error())
		return
	}

	mapDatasetResponseToState(&data, &result)
	tflog.Trace(ctx, "updated dataset resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/datasets/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting dataset", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted dataset resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *DatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapDatasetResponseToState translates the API response into Terraform state.
// Mind the nulls — an absent field means nothing's there, not that it's empty.
func mapDatasetResponseToState(data *DatasetResourceModel, result *datasetAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.DataType = types.StringValue(result.DataType)

	if len(result.InputsSchemaDefinition) > 0 && string(result.InputsSchemaDefinition) != "null" {
		data.InputsSchemaDefinition = types.StringValue(string(result.InputsSchemaDefinition))
	} else {
		data.InputsSchemaDefinition = types.StringNull()
	}

	if len(result.OutputsSchemaDefinition) > 0 && string(result.OutputsSchemaDefinition) != "null" {
		data.OutputsSchemaDefinition = types.StringValue(string(result.OutputsSchemaDefinition))
	} else {
		data.OutputsSchemaDefinition = types.StringNull()
	}

	if result.ExternallyManaged != nil {
		data.ExternallyManaged = types.BoolValue(*result.ExternallyManaged)
	} else {
		data.ExternallyManaged = types.BoolNull()
	}

	// Round up the extra fields — every head of cattle needs accounting for.
	if len(result.Transformations) > 0 && string(result.Transformations) != "null" {
		data.Transformations = types.StringValue(string(result.Transformations))
	} else {
		data.Transformations = types.StringNull()
	}
	if len(result.Metadata) > 0 && string(result.Metadata) != "null" {
		data.Metadata = types.StringValue(string(result.Metadata))
	} else {
		data.Metadata = types.StringNull()
	}
	if result.ExampleCount != nil {
		data.ExampleCount = types.Int64Value(*result.ExampleCount)
	} else {
		data.ExampleCount = types.Int64Value(0)
	}
	data.SessionCount = types.Int64Value(result.SessionCount)
	data.ModifiedAt = types.StringValue(result.ModifiedAt)
	if result.LastSessionStartTime != nil {
		data.LastSessionStartTime = types.StringValue(*result.LastSessionStartTime)
	} else {
		data.LastSessionStartTime = types.StringNull()
	}

	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
}

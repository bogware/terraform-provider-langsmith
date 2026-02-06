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

// NewDatasetResource returns a new DatasetResource.
func NewDatasetResource() resource.Resource {
	return &DatasetResource{}
}

// DatasetResource defines the resource implementation.
type DatasetResource struct {
	client *client.Client
}

// DatasetResourceModel describes the resource data model.
type DatasetResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	DataType                types.String `tfsdk:"data_type"`
	InputsSchemaDefinition  types.String `tfsdk:"inputs_schema_definition"`
	OutputsSchemaDefinition types.String `tfsdk:"outputs_schema_definition"`
	ExternallyManaged       types.Bool   `tfsdk:"externally_managed"`
	TenantID                types.String `tfsdk:"tenant_id"`
	CreatedAt               types.String `tfsdk:"created_at"`
}

// datasetAPIRequest is the request body for creating/updating a dataset.
type datasetAPIRequest struct {
	Name                    string          `json:"name"`
	Description             *string         `json:"description,omitempty"`
	DataType                *string         `json:"data_type,omitempty"`
	InputsSchemaDefinition  json.RawMessage `json:"inputs_schema_definition,omitempty"`
	OutputsSchemaDefinition json.RawMessage `json:"outputs_schema_definition,omitempty"`
	ExternallyManaged       *bool           `json:"externally_managed,omitempty"`
}

// datasetAPIResponse is the API response for a dataset.
type datasetAPIResponse struct {
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Description             *string         `json:"description"`
	DataType                string          `json:"data_type"`
	InputsSchemaDefinition  json.RawMessage `json:"inputs_schema_definition"`
	OutputsSchemaDefinition json.RawMessage `json:"outputs_schema_definition"`
	ExternallyManaged       *bool           `json:"externally_managed"`
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

// mapDatasetResponseToState maps an API response to the Terraform state model.
func mapDatasetResponseToState(data *DatasetResourceModel, result *datasetAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.DataType = types.StringValue(result.DataType)

	if result.InputsSchemaDefinition != nil && len(result.InputsSchemaDefinition) > 0 && string(result.InputsSchemaDefinition) != "null" {
		data.InputsSchemaDefinition = types.StringValue(string(result.InputsSchemaDefinition))
	} else {
		data.InputsSchemaDefinition = types.StringNull()
	}

	if result.OutputsSchemaDefinition != nil && len(result.OutputsSchemaDefinition) > 0 && string(result.OutputsSchemaDefinition) != "null" {
		data.OutputsSchemaDefinition = types.StringValue(string(result.OutputsSchemaDefinition))
	} else {
		data.OutputsSchemaDefinition = types.StringNull()
	}

	if result.ExternallyManaged != nil {
		data.ExternallyManaged = types.BoolValue(*result.ExternallyManaged)
	} else {
		data.ExternallyManaged = types.BoolNull()
	}

	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
}

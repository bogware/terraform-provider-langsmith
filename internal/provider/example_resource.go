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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &ExampleResource{}
	_ resource.ResourceWithImportState = &ExampleResource{}
)

// NewExampleResource returns a new ExampleResource.
func NewExampleResource() resource.Resource {
	return &ExampleResource{}
}

// ExampleResource defines the resource implementation.
type ExampleResource struct {
	client *client.Client
}

// ExampleResourceModel describes the resource data model.
type ExampleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DatasetID   types.String `tfsdk:"dataset_id"`
	Inputs      types.String `tfsdk:"inputs"`
	Outputs     types.String `tfsdk:"outputs"`
	Metadata    types.String `tfsdk:"metadata"`
	Split       types.String `tfsdk:"split"`
	SourceRunID types.String `tfsdk:"source_run_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	ModifiedAt  types.String `tfsdk:"modified_at"`
}

// exampleAPICreateRequest is the request body for creating an example.
type exampleAPICreateRequest struct {
	DatasetID   string          `json:"dataset_id"`
	Inputs      json.RawMessage `json:"inputs"`
	Outputs     json.RawMessage `json:"outputs,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Split       *string         `json:"split,omitempty"`
	SourceRunID *string         `json:"source_run_id,omitempty"`
}

// exampleAPIUpdateRequest is the request body for updating an example.
type exampleAPIUpdateRequest struct {
	DatasetID   *string         `json:"dataset_id,omitempty"`
	Inputs      json.RawMessage `json:"inputs,omitempty"`
	Outputs     json.RawMessage `json:"outputs,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Split       *string         `json:"split,omitempty"`
	SourceRunID *string         `json:"source_run_id,omitempty"`
}

// exampleAPIResponse is the API response for an example.
type exampleAPIResponse struct {
	ID          string          `json:"id"`
	DatasetID   string          `json:"dataset_id"`
	Inputs      json.RawMessage `json:"inputs"`
	Outputs     json.RawMessage `json:"outputs"`
	Metadata    json.RawMessage `json:"metadata"`
	Split       *string         `json:"split"`
	SourceRunID *string         `json:"source_run_id"`
	CreatedAt   string          `json:"created_at"`
	ModifiedAt  string          `json:"modified_at"`
}

func (r *ExampleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_example"
}

func (r *ExampleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith example within a dataset.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the example.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the dataset this example belongs to.",
				Required:            true,
			},
			"inputs": schema.StringAttribute{
				MarkdownDescription: "JSON string containing the input data for the example.",
				Required:            true,
			},
			"outputs": schema.StringAttribute{
				MarkdownDescription: "JSON string containing the output data for the example.",
				Optional:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON string containing metadata for the example.",
				Optional:            true,
			},
			"split": schema.StringAttribute{
				MarkdownDescription: "The split for the example (e.g., `base`, `train`, `test`).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("base"),
			},
			"source_run_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the source run for this example.",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the example.",
				Computed:            true,
			},
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "The last modification timestamp of the example.",
				Computed:            true,
			},
		},
	}
}

func (r *ExampleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ExampleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ExampleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := exampleAPICreateRequest{
		DatasetID: data.DatasetID.ValueString(),
		Inputs:    json.RawMessage(data.Inputs.ValueString()),
	}

	if !data.Outputs.IsNull() && !data.Outputs.IsUnknown() {
		body.Outputs = json.RawMessage(data.Outputs.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		body.Metadata = json.RawMessage(data.Metadata.ValueString())
	}
	if !data.Split.IsNull() && !data.Split.IsUnknown() {
		v := data.Split.ValueString()
		body.Split = &v
	}
	if !data.SourceRunID.IsNull() && !data.SourceRunID.IsUnknown() {
		v := data.SourceRunID.ValueString()
		body.SourceRunID = &v
	}

	var result exampleAPIResponse
	err := r.client.Post(ctx, "/api/v1/examples", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating example", err.Error())
		return
	}

	mapExampleResponseToState(&data, &result)
	tflog.Trace(ctx, "created example resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExampleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ExampleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result exampleAPIResponse
	err := r.client.Get(ctx, "/api/v1/examples/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading example", err.Error())
		return
	}

	mapExampleResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExampleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ExampleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := exampleAPIUpdateRequest{
		Inputs: json.RawMessage(data.Inputs.ValueString()),
	}

	if !data.DatasetID.IsNull() && !data.DatasetID.IsUnknown() {
		v := data.DatasetID.ValueString()
		body.DatasetID = &v
	}
	if !data.Outputs.IsNull() && !data.Outputs.IsUnknown() {
		body.Outputs = json.RawMessage(data.Outputs.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		body.Metadata = json.RawMessage(data.Metadata.ValueString())
	}
	if !data.Split.IsNull() && !data.Split.IsUnknown() {
		v := data.Split.ValueString()
		body.Split = &v
	}
	if !data.SourceRunID.IsNull() && !data.SourceRunID.IsUnknown() {
		v := data.SourceRunID.ValueString()
		body.SourceRunID = &v
	}

	var result exampleAPIResponse
	err := r.client.Patch(ctx, "/api/v1/examples/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating example", err.Error())
		return
	}

	mapExampleResponseToState(&data, &result)
	tflog.Trace(ctx, "updated example resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExampleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ExampleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/examples/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting example", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted example resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ExampleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapExampleResponseToState maps an API response to the Terraform state model.
func mapExampleResponseToState(data *ExampleResourceModel, result *exampleAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.DatasetID = types.StringValue(result.DatasetID)

	if result.Inputs != nil && len(result.Inputs) > 0 && string(result.Inputs) != "null" {
		data.Inputs = types.StringValue(string(result.Inputs))
	}

	if result.Outputs != nil && len(result.Outputs) > 0 && string(result.Outputs) != "null" {
		data.Outputs = types.StringValue(string(result.Outputs))
	} else {
		data.Outputs = types.StringNull()
	}

	if result.Metadata != nil && len(result.Metadata) > 0 && string(result.Metadata) != "null" {
		data.Metadata = types.StringValue(string(result.Metadata))
	} else {
		data.Metadata = types.StringNull()
	}

	if result.Split != nil {
		data.Split = types.StringValue(*result.Split)
	}

	if result.SourceRunID != nil {
		data.SourceRunID = types.StringValue(*result.SourceRunID)
	} else {
		data.SourceRunID = types.StringNull()
	}

	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.ModifiedAt = types.StringValue(result.ModifiedAt)
}

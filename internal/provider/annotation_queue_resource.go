// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &AnnotationQueueResource{}
	_ resource.ResourceWithImportState = &AnnotationQueueResource{}
)

// NewAnnotationQueueResource returns a new AnnotationQueueResource.
func NewAnnotationQueueResource() resource.Resource {
	return &AnnotationQueueResource{}
}

// AnnotationQueueResource defines the resource implementation.
type AnnotationQueueResource struct {
	client *client.Client
}

// AnnotationQueueResourceModel describes the resource data model.
type AnnotationQueueResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	EnableReservations   types.Bool   `tfsdk:"enable_reservations"`
	NumReviewersPerItem  types.Int64  `tfsdk:"num_reviewers_per_item"`
	ReservationMinutes   types.Int64  `tfsdk:"reservation_minutes"`
	DefaultDataset       types.String `tfsdk:"default_dataset"`
	RubricInstructions   types.String `tfsdk:"rubric_instructions"`
	TenantID             types.String `tfsdk:"tenant_id"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

// annotationQueueAPIRequest is the request body for creating/updating an annotation queue.
type annotationQueueAPIRequest struct {
	Name                string  `json:"name"`
	Description         *string `json:"description,omitempty"`
	EnableReservations  *bool   `json:"enable_reservations,omitempty"`
	NumReviewersPerItem *int64  `json:"num_reviewers_per_item,omitempty"`
	ReservationMinutes  *int64  `json:"reservation_minutes,omitempty"`
	DefaultDataset      *string `json:"default_dataset,omitempty"`
	RubricInstructions  *string `json:"rubric_instructions,omitempty"`
}

// annotationQueueAPIResponse is the API response for an annotation queue.
type annotationQueueAPIResponse struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	EnableReservations  *bool   `json:"enable_reservations"`
	NumReviewersPerItem *int64  `json:"num_reviewers_per_item"`
	ReservationMinutes  *int64  `json:"reservation_minutes"`
	DefaultDataset      *string `json:"default_dataset"`
	RubricInstructions  *string `json:"rubric_instructions"`
	TenantID            string  `json:"tenant_id"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func (r *AnnotationQueueResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation_queue"
}

func (r *AnnotationQueueResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith annotation queue.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the annotation queue.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the annotation queue.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the annotation queue.",
				Optional:            true,
			},
			"enable_reservations": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable reservations for the annotation queue.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"num_reviewers_per_item": schema.Int64Attribute{
				MarkdownDescription: "The number of reviewers per item in the queue.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"reservation_minutes": schema.Int64Attribute{
				MarkdownDescription: "The number of minutes a reservation is held.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"default_dataset": schema.StringAttribute{
				MarkdownDescription: "The UUID of the default dataset for the annotation queue.",
				Optional:            true,
			},
			"rubric_instructions": schema.StringAttribute{
				MarkdownDescription: "Rubric instructions for reviewers.",
				Optional:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID of the annotation queue.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the annotation queue.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last update timestamp of the annotation queue.",
				Computed:            true,
			},
		},
	}
}

func (r *AnnotationQueueResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AnnotationQueueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AnnotationQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := annotationQueueAPIRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.EnableReservations.IsNull() && !data.EnableReservations.IsUnknown() {
		v := data.EnableReservations.ValueBool()
		body.EnableReservations = &v
	}
	if !data.NumReviewersPerItem.IsNull() && !data.NumReviewersPerItem.IsUnknown() {
		v := data.NumReviewersPerItem.ValueInt64()
		body.NumReviewersPerItem = &v
	}
	if !data.ReservationMinutes.IsNull() && !data.ReservationMinutes.IsUnknown() {
		v := data.ReservationMinutes.ValueInt64()
		body.ReservationMinutes = &v
	}
	if !data.DefaultDataset.IsNull() && !data.DefaultDataset.IsUnknown() {
		v := data.DefaultDataset.ValueString()
		body.DefaultDataset = &v
	}
	if !data.RubricInstructions.IsNull() && !data.RubricInstructions.IsUnknown() {
		v := data.RubricInstructions.ValueString()
		body.RubricInstructions = &v
	}

	var result annotationQueueAPIResponse
	err := r.client.Post(ctx, "/api/v1/annotation-queues", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating annotation queue", err.Error())
		return
	}

	mapAnnotationQueueResponseToState(&data, &result)
	tflog.Trace(ctx, "created annotation queue resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnotationQueueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AnnotationQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result annotationQueueAPIResponse
	err := r.client.Get(ctx, "/api/v1/annotation-queues/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading annotation queue", err.Error())
		return
	}

	mapAnnotationQueueResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnotationQueueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AnnotationQueueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := annotationQueueAPIRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.EnableReservations.IsNull() && !data.EnableReservations.IsUnknown() {
		v := data.EnableReservations.ValueBool()
		body.EnableReservations = &v
	}
	if !data.NumReviewersPerItem.IsNull() && !data.NumReviewersPerItem.IsUnknown() {
		v := data.NumReviewersPerItem.ValueInt64()
		body.NumReviewersPerItem = &v
	}
	if !data.ReservationMinutes.IsNull() && !data.ReservationMinutes.IsUnknown() {
		v := data.ReservationMinutes.ValueInt64()
		body.ReservationMinutes = &v
	}
	if !data.DefaultDataset.IsNull() && !data.DefaultDataset.IsUnknown() {
		v := data.DefaultDataset.ValueString()
		body.DefaultDataset = &v
	}
	if !data.RubricInstructions.IsNull() && !data.RubricInstructions.IsUnknown() {
		v := data.RubricInstructions.ValueString()
		body.RubricInstructions = &v
	}

	var result annotationQueueAPIResponse
	err := r.client.Patch(ctx, "/api/v1/annotation-queues/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating annotation queue", err.Error())
		return
	}

	mapAnnotationQueueResponseToState(&data, &result)
	tflog.Trace(ctx, "updated annotation queue resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnotationQueueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnnotationQueueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/annotation-queues/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting annotation queue", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted annotation queue resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *AnnotationQueueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapAnnotationQueueResponseToState maps an API response to the Terraform state model.
func mapAnnotationQueueResponseToState(data *AnnotationQueueResourceModel, result *annotationQueueAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	if result.EnableReservations != nil {
		data.EnableReservations = types.BoolValue(*result.EnableReservations)
	}

	if result.NumReviewersPerItem != nil {
		data.NumReviewersPerItem = types.Int64Value(*result.NumReviewersPerItem)
	}

	if result.ReservationMinutes != nil {
		data.ReservationMinutes = types.Int64Value(*result.ReservationMinutes)
	}

	if result.DefaultDataset != nil {
		data.DefaultDataset = types.StringValue(*result.DefaultDataset)
	} else {
		data.DefaultDataset = types.StringNull()
	}

	if result.RubricInstructions != nil {
		data.RubricInstructions = types.StringValue(*result.RubricInstructions)
	} else {
		data.RubricInstructions = types.StringNull()
	}

	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

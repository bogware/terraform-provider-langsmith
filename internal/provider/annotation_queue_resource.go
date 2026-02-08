// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

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

// NewAnnotationQueueResource returns a new AnnotationQueueResource, ready to
// line up items for human review like cattle at the stockyard chute.
func NewAnnotationQueueResource() resource.Resource {
	return &AnnotationQueueResource{}
}

// AnnotationQueueResource manages a LangSmith annotation queue for organizing
// human review of LLM runs. Supports reservations, reviewer counts, rubric
// instructions, and an optional default dataset for collected annotations.
type AnnotationQueueResource struct {
	client *client.Client
}

// AnnotationQueueResourceModel describes the resource data model, including
// reservation settings, reviewer configuration, and rubric instructions.
type AnnotationQueueResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	EnableReservations  types.Bool   `tfsdk:"enable_reservations"`
	NumReviewersPerItem types.Int64  `tfsdk:"num_reviewers_per_item"`
	ReservationMinutes  types.Int64  `tfsdk:"reservation_minutes"`
	DefaultDataset      types.String `tfsdk:"default_dataset"`
	RubricInstructions  types.String `tfsdk:"rubric_instructions"`
	RubricItems         types.String `tfsdk:"rubric_items"`
	Metadata            types.String `tfsdk:"metadata"`
	SourceRuleID        types.String `tfsdk:"source_rule_id"`
	RunRuleID           types.String `tfsdk:"run_rule_id"`
	QueueType           types.String `tfsdk:"queue_type"`
	TenantID            types.String `tfsdk:"tenant_id"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

// annotationQueueAPIRequest is the request body for creating/updating an annotation queue.
type annotationQueueAPIRequest struct {
	Name                string          `json:"name"`
	Description         *string         `json:"description,omitempty"`
	EnableReservations  *bool           `json:"enable_reservations,omitempty"`
	NumReviewersPerItem *int64          `json:"num_reviewers_per_item,omitempty"`
	ReservationMinutes  *int64          `json:"reservation_minutes,omitempty"`
	DefaultDataset      *string         `json:"default_dataset,omitempty"`
	RubricInstructions  *string         `json:"rubric_instructions,omitempty"`
	RubricItems         json.RawMessage `json:"rubric_items,omitempty"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
}

// annotationQueueAPIResponse is the API response for an annotation queue.
type annotationQueueAPIResponse struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Description         *string         `json:"description"`
	EnableReservations  *bool           `json:"enable_reservations"`
	NumReviewersPerItem *int64          `json:"num_reviewers_per_item"`
	ReservationMinutes  *int64          `json:"reservation_minutes"`
	DefaultDataset      *string         `json:"default_dataset"`
	RubricInstructions  *string         `json:"rubric_instructions"`
	RubricItems         json.RawMessage `json:"rubric_items"`
	Metadata            json.RawMessage `json:"metadata"`
	SourceRuleID        *string         `json:"source_rule_id"`
	RunRuleID           *string         `json:"run_rule_id"`
	QueueType           string          `json:"queue_type"`
	TenantID            string          `json:"tenant_id"`
	CreatedAt           string          `json:"created_at"`
	UpdatedAt           string          `json:"updated_at"`
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
			"rubric_items": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of rubric items for the annotation queue.",
				Optional:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded metadata object.",
				Optional:            true,
			},
			"source_rule_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the source rule that created this queue.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"run_rule_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the run rule associated with this queue.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"queue_type": schema.StringAttribute{
				MarkdownDescription: "The type of annotation queue.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID of the annotation queue.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the annotation queue.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last update timestamp of the annotation queue.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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
	// Rubric items and metadata ride along as raw JSON -- no need to break 'em in.
	if !data.RubricItems.IsNull() && !data.RubricItems.IsUnknown() {
		body.RubricItems = json.RawMessage(data.RubricItems.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		body.Metadata = json.RawMessage(data.Metadata.ValueString())
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
	// Same as Create -- hitch up the raw JSON fields for the ride to the API.
	if !data.RubricItems.IsNull() && !data.RubricItems.IsUnknown() {
		body.RubricItems = json.RawMessage(data.RubricItems.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		body.Metadata = json.RawMessage(data.Metadata.ValueString())
	}

	err := r.client.Patch(ctx, "/api/v1/annotation-queues/"+data.ID.ValueString(), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating annotation queue", err.Error())
		return
	}

	// The PATCH response only returns {"message": "..."}, not the full resource.
	// Like Festus reporting back with half the story, we need to go get the rest ourselves.
	var result annotationQueueAPIResponse
	err = r.client.Get(ctx, "/api/v1/annotation-queues/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading annotation queue after update", err.Error())
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

	q := url.Values{}
	q.Set("queue_ids", data.ID.ValueString())
	err := r.client.DeleteWithQuery(ctx, "/api/v1/annotation-queues", q)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting annotation queue", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted annotation queue resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *AnnotationQueueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapAnnotationQueueResponseToState maps the API response onto the Terraform state,
// setting null for any optional fields the API left unspoken.
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
	} else {
		data.EnableReservations = types.BoolNull()
	}

	if result.NumReviewersPerItem != nil {
		data.NumReviewersPerItem = types.Int64Value(*result.NumReviewersPerItem)
	} else {
		data.NumReviewersPerItem = types.Int64Null()
	}

	if result.ReservationMinutes != nil {
		data.ReservationMinutes = types.Int64Value(*result.ReservationMinutes)
	} else {
		data.ReservationMinutes = types.Int64Null()
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

	// Rubric items and metadata come back as raw JSON -- round 'em up carefully
	// so Terraform don't report phantom drift on empty corrals.
	if len(result.RubricItems) > 0 && string(result.RubricItems) != "null" && string(result.RubricItems) != "[]" {
		data.RubricItems = types.StringValue(string(result.RubricItems))
	} else {
		data.RubricItems = types.StringNull()
	}
	if len(result.Metadata) > 0 && string(result.Metadata) != "null" && string(result.Metadata) != "{}" {
		data.Metadata = types.StringValue(string(result.Metadata))
	} else {
		data.Metadata = types.StringNull()
	}

	if result.SourceRuleID != nil {
		data.SourceRuleID = types.StringValue(*result.SourceRuleID)
	} else {
		data.SourceRuleID = types.StringNull()
	}
	if result.RunRuleID != nil {
		data.RunRuleID = types.StringValue(*result.RunRuleID)
	} else {
		data.RunRuleID = types.StringNull()
	}

	data.QueueType = types.StringValue(result.QueueType)
	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

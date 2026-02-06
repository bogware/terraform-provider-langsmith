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
	_ resource.Resource                = &RunRuleResource{}
	_ resource.ResourceWithImportState = &RunRuleResource{}
)

func NewRunRuleResource() resource.Resource {
	return &RunRuleResource{}
}

type RunRuleResource struct {
	client *client.Client
}

type RunRuleResourceModel struct {
	ID                           types.String  `tfsdk:"id"`
	DisplayName                  types.String  `tfsdk:"display_name"`
	SamplingRate                 types.Float64 `tfsdk:"sampling_rate"`
	SessionID                    types.String  `tfsdk:"session_id"`
	IsEnabled                    types.Bool    `tfsdk:"is_enabled"`
	Filter                       types.String  `tfsdk:"filter"`
	TraceFilter                  types.String  `tfsdk:"trace_filter"`
	TreeFilter                   types.String  `tfsdk:"tree_filter"`
	AddToAnnotationQueueID       types.String  `tfsdk:"add_to_annotation_queue_id"`
	AddToDatasetID               types.String  `tfsdk:"add_to_dataset_id"`
	AddToDatasetPreferCorrection types.Bool    `tfsdk:"add_to_dataset_prefer_correction"`
	NumFewShotExamples           types.Int64   `tfsdk:"num_few_shot_examples"`
	TenantID                     types.String  `tfsdk:"tenant_id"`
	CreatedAt                    types.String  `tfsdk:"created_at"`
	UpdatedAt                    types.String  `tfsdk:"updated_at"`
}

type runRuleCreateRequest struct {
	DisplayName                  string  `json:"display_name"`
	SamplingRate                 float64 `json:"sampling_rate"`
	SessionID                    string  `json:"session_id,omitempty"`
	IsEnabled                    bool    `json:"is_enabled"`
	Filter                       string  `json:"filter,omitempty"`
	TraceFilter                  string  `json:"trace_filter,omitempty"`
	TreeFilter                   string  `json:"tree_filter,omitempty"`
	AddToAnnotationQueueID       string  `json:"add_to_annotation_queue_id,omitempty"`
	AddToDatasetID               string  `json:"add_to_dataset_id,omitempty"`
	AddToDatasetPreferCorrection bool    `json:"add_to_dataset_prefer_correction,omitempty"`
	NumFewShotExamples           int64   `json:"num_few_shot_examples,omitempty"`
}

type runRuleAPIResponse struct {
	ID                           string  `json:"id"`
	DisplayName                  string  `json:"display_name"`
	SamplingRate                 float64 `json:"sampling_rate"`
	SessionID                    string  `json:"session_id"`
	IsEnabled                    bool    `json:"is_enabled"`
	Filter                       string  `json:"filter"`
	TraceFilter                  string  `json:"trace_filter"`
	TreeFilter                   string  `json:"tree_filter"`
	AddToAnnotationQueueID       string  `json:"add_to_annotation_queue_id"`
	AddToDatasetID               string  `json:"add_to_dataset_id"`
	AddToDatasetPreferCorrection bool    `json:"add_to_dataset_prefer_correction"`
	NumFewShotExamples           int64   `json:"num_few_shot_examples"`
	TenantID                     string  `json:"tenant_id"`
	CreatedAt                    string  `json:"created_at"`
	UpdatedAt                    string  `json:"updated_at"`
}

func (r *RunRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run_rule"
}

func (r *RunRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an automation rule for runs in LangSmith.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the run rule.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the run rule.",
				Required:            true,
			},
			"sampling_rate": schema.Float64Attribute{
				MarkdownDescription: "The sampling rate (0.0 to 1.0).",
				Required:            true,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project/session UUID to scope this rule to.",
				Optional:            true,
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is enabled.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "Run filter expression.",
				Optional:            true,
			},
			"trace_filter": schema.StringAttribute{
				MarkdownDescription: "Trace filter expression.",
				Optional:            true,
			},
			"tree_filter": schema.StringAttribute{
				MarkdownDescription: "Tree filter expression.",
				Optional:            true,
			},
			"add_to_annotation_queue_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the annotation queue to add matching runs to.",
				Optional:            true,
			},
			"add_to_dataset_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the dataset to add matching runs to.",
				Optional:            true,
			},
			"add_to_dataset_prefer_correction": schema.BoolAttribute{
				MarkdownDescription: "Whether to prefer correction when adding to dataset.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"num_few_shot_examples": schema.Int64Attribute{
				MarkdownDescription: "Number of few-shot examples.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the rule was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "When the rule was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *RunRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *RunRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RunRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := runRuleCreateRequest{
		DisplayName:  data.DisplayName.ValueString(),
		SamplingRate: data.SamplingRate.ValueFloat64(),
		IsEnabled:    data.IsEnabled.ValueBool(),
	}
	if !data.SessionID.IsNull() {
		body.SessionID = data.SessionID.ValueString()
	}
	if !data.Filter.IsNull() {
		body.Filter = data.Filter.ValueString()
	}
	if !data.TraceFilter.IsNull() {
		body.TraceFilter = data.TraceFilter.ValueString()
	}
	if !data.TreeFilter.IsNull() {
		body.TreeFilter = data.TreeFilter.ValueString()
	}
	if !data.AddToAnnotationQueueID.IsNull() {
		body.AddToAnnotationQueueID = data.AddToAnnotationQueueID.ValueString()
	}
	if !data.AddToDatasetID.IsNull() {
		body.AddToDatasetID = data.AddToDatasetID.ValueString()
	}
	if !data.AddToDatasetPreferCorrection.IsNull() {
		body.AddToDatasetPreferCorrection = data.AddToDatasetPreferCorrection.ValueBool()
	}
	if !data.NumFewShotExamples.IsNull() {
		body.NumFewShotExamples = data.NumFewShotExamples.ValueInt64()
	}

	var result runRuleAPIResponse
	err := r.client.Post(ctx, "/api/v1/runs/rules", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating run rule", err.Error())
		return
	}

	r.mapResponseToModel(&result, &data)

	tflog.Trace(ctx, "created run rule resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RunRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rules []runRuleAPIResponse
	err := r.client.Get(ctx, "/api/v1/runs/rules", nil, &rules)
	if err != nil {
		resp.Diagnostics.AddError("Error reading run rules", err.Error())
		return
	}

	var found *runRuleAPIResponse
	for i := range rules {
		if rules[i].ID == data.ID.ValueString() {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.mapResponseToModel(found, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RunRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := runRuleCreateRequest{
		DisplayName:  data.DisplayName.ValueString(),
		SamplingRate: data.SamplingRate.ValueFloat64(),
		IsEnabled:    data.IsEnabled.ValueBool(),
	}
	if !data.SessionID.IsNull() {
		body.SessionID = data.SessionID.ValueString()
	}
	if !data.Filter.IsNull() {
		body.Filter = data.Filter.ValueString()
	}
	if !data.TraceFilter.IsNull() {
		body.TraceFilter = data.TraceFilter.ValueString()
	}
	if !data.TreeFilter.IsNull() {
		body.TreeFilter = data.TreeFilter.ValueString()
	}
	if !data.AddToAnnotationQueueID.IsNull() {
		body.AddToAnnotationQueueID = data.AddToAnnotationQueueID.ValueString()
	}
	if !data.AddToDatasetID.IsNull() {
		body.AddToDatasetID = data.AddToDatasetID.ValueString()
	}
	if !data.AddToDatasetPreferCorrection.IsNull() {
		body.AddToDatasetPreferCorrection = data.AddToDatasetPreferCorrection.ValueBool()
	}
	if !data.NumFewShotExamples.IsNull() {
		body.NumFewShotExamples = data.NumFewShotExamples.ValueInt64()
	}

	var result runRuleAPIResponse
	err := r.client.Patch(ctx, fmt.Sprintf("/api/v1/runs/rules/%s", data.ID.ValueString()), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating run rule", err.Error())
		return
	}

	r.mapResponseToModel(&result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RunRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, fmt.Sprintf("/api/v1/runs/rules/%s", data.ID.ValueString()))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting run rule", err.Error())
	}
}

func (r *RunRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *RunRuleResource) mapResponseToModel(result *runRuleAPIResponse, data *RunRuleResourceModel) {
	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.SamplingRate = types.Float64Value(result.SamplingRate)
	data.IsEnabled = types.BoolValue(result.IsEnabled)
	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	if result.SessionID != "" {
		data.SessionID = types.StringValue(result.SessionID)
	} else {
		data.SessionID = types.StringNull()
	}
	if result.Filter != "" {
		data.Filter = types.StringValue(result.Filter)
	} else {
		data.Filter = types.StringNull()
	}
	if result.TraceFilter != "" {
		data.TraceFilter = types.StringValue(result.TraceFilter)
	} else {
		data.TraceFilter = types.StringNull()
	}
	if result.TreeFilter != "" {
		data.TreeFilter = types.StringValue(result.TreeFilter)
	} else {
		data.TreeFilter = types.StringNull()
	}
	if result.AddToAnnotationQueueID != "" {
		data.AddToAnnotationQueueID = types.StringValue(result.AddToAnnotationQueueID)
	} else {
		data.AddToAnnotationQueueID = types.StringNull()
	}
	if result.AddToDatasetID != "" {
		data.AddToDatasetID = types.StringValue(result.AddToDatasetID)
	} else {
		data.AddToDatasetID = types.StringNull()
	}
	data.AddToDatasetPreferCorrection = types.BoolValue(result.AddToDatasetPreferCorrection)
	data.NumFewShotExamples = types.Int64Value(result.NumFewShotExamples)
}

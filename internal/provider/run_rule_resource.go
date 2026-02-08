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

// NewRunRuleResource returns a new RunRuleResource, badge and all.
func NewRunRuleResource() resource.Resource {
	return &RunRuleResource{}
}

// RunRuleResource implements CRUD for LangSmith automation rules --
// the law that governs which runs get rounded up and where they end up.
type RunRuleResource struct {
	client *client.Client
}

// RunRuleResourceModel is the Terraform state for an automation rule,
// tracking everything from sampling rates to which corral the runs land in.
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
	DatasetID                    types.String  `tfsdk:"dataset_id"`
	BackfillFrom                 types.String  `tfsdk:"backfill_from"`
	UseCorrectionsDataset        types.Bool    `tfsdk:"use_corrections_dataset"`
	ExtendOnly                   types.Bool    `tfsdk:"extend_only"`
	Transient                    types.Bool    `tfsdk:"transient"`
	IncludeExtendedStats         types.Bool    `tfsdk:"include_extended_stats"`
	GroupBy                      types.String  `tfsdk:"group_by"`
	Evaluators                   types.String  `tfsdk:"evaluators"`
	CodeEvaluators               types.String  `tfsdk:"code_evaluators"`
	Alerts                       types.String  `tfsdk:"alerts"`
	Webhooks                     types.String  `tfsdk:"webhooks"`
	SessionName                  types.String  `tfsdk:"session_name"`
	DatasetName                  types.String  `tfsdk:"dataset_name"`
	CorrectionsDatasetID         types.String  `tfsdk:"corrections_dataset_id"`
	EvaluatorID                  types.String  `tfsdk:"evaluator_id"`
	AlignmentAnnotationQueueID   types.String  `tfsdk:"alignment_annotation_queue_id"`
	TenantID                     types.String  `tfsdk:"tenant_id"`
	CreatedAt                    types.String  `tfsdk:"created_at"`
	UpdatedAt                    types.String  `tfsdk:"updated_at"`
}

// runRuleCreateRequest is the warrant for establishing a new automation rule.
// Every field's accounted for -- Miss Kitty wouldn't let us cut corners.
type runRuleCreateRequest struct {
	DisplayName                  string          `json:"display_name"`
	SamplingRate                 float64         `json:"sampling_rate"`
	SessionID                    string          `json:"session_id,omitempty"`
	IsEnabled                    bool            `json:"is_enabled"`
	Filter                       string          `json:"filter,omitempty"`
	TraceFilter                  string          `json:"trace_filter,omitempty"`
	TreeFilter                   string          `json:"tree_filter,omitempty"`
	AddToAnnotationQueueID       string          `json:"add_to_annotation_queue_id,omitempty"`
	AddToDatasetID               string          `json:"add_to_dataset_id,omitempty"`
	AddToDatasetPreferCorrection bool            `json:"add_to_dataset_prefer_correction,omitempty"`
	NumFewShotExamples           int64           `json:"num_few_shot_examples,omitempty"`
	DatasetID                    *string         `json:"dataset_id,omitempty"`
	BackfillFrom                 *string         `json:"backfill_from,omitempty"`
	UseCorrectionsDataset        *bool           `json:"use_corrections_dataset,omitempty"`
	ExtendOnly                   *bool           `json:"extend_only,omitempty"`
	Transient                    *bool           `json:"transient,omitempty"`
	IncludeExtendedStats         *bool           `json:"include_extended_stats,omitempty"`
	GroupBy                      *string         `json:"group_by,omitempty"`
	Evaluators                   json.RawMessage `json:"evaluators,omitempty"`
	CodeEvaluators               json.RawMessage `json:"code_evaluators,omitempty"`
	Alerts                       json.RawMessage `json:"alerts,omitempty"`
	Webhooks                     json.RawMessage `json:"webhooks,omitempty"`
}

// runRuleAPIResponse is the full dossier the API returns on a run rule --
// every last detail, like a wanted poster nailed to the Long Branch wall.
type runRuleAPIResponse struct {
	ID                           string          `json:"id"`
	DisplayName                  string          `json:"display_name"`
	SamplingRate                 float64         `json:"sampling_rate"`
	SessionID                    string          `json:"session_id"`
	IsEnabled                    bool            `json:"is_enabled"`
	Filter                       string          `json:"filter"`
	TraceFilter                  string          `json:"trace_filter"`
	TreeFilter                   string          `json:"tree_filter"`
	AddToAnnotationQueueID       string          `json:"add_to_annotation_queue_id"`
	AddToDatasetID               string          `json:"add_to_dataset_id"`
	AddToDatasetPreferCorrection bool            `json:"add_to_dataset_prefer_correction"`
	NumFewShotExamples           int64           `json:"num_few_shot_examples"`
	DatasetID                    *string         `json:"dataset_id"`
	BackfillFrom                 *string         `json:"backfill_from"`
	UseCorrectionsDataset        bool            `json:"use_corrections_dataset"`
	ExtendOnly                   bool            `json:"extend_only"`
	Transient                    bool            `json:"transient"`
	IncludeExtendedStats         bool            `json:"include_extended_stats"`
	GroupBy                      *string         `json:"group_by"`
	Evaluators                   json.RawMessage `json:"evaluators"`
	CodeEvaluators               json.RawMessage `json:"code_evaluators"`
	Alerts                       json.RawMessage `json:"alerts"`
	Webhooks                     json.RawMessage `json:"webhooks"`
	SessionName                  *string         `json:"session_name"`
	DatasetName                  *string         `json:"dataset_name"`
	CorrectionsDatasetID         *string         `json:"corrections_dataset_id"`
	EvaluatorID                  *string         `json:"evaluator_id"`
	AlignmentAnnotationQueueID   *string         `json:"alignment_annotation_queue_id"`
	TenantID                     string          `json:"tenant_id"`
	CreatedAt                    string          `json:"created_at"`
	UpdatedAt                    string          `json:"updated_at"`
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
			// New fields ride into town -- Dodge City keeps growing.
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the associated dataset.",
				Optional:            true,
			},
			"backfill_from": schema.StringAttribute{
				MarkdownDescription: "ISO timestamp to backfill rules from.",
				Optional:            true,
			},
			"use_corrections_dataset": schema.BoolAttribute{
				MarkdownDescription: "Whether to use a corrections dataset.",
				Optional:            true,
				Computed:            true,
			},
			"extend_only": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule only extends existing annotations.",
				Optional:            true,
				Computed:            true,
			},
			"transient": schema.BoolAttribute{
				MarkdownDescription: "Whether the rule is transient.",
				Optional:            true,
				Computed:            true,
			},
			"include_extended_stats": schema.BoolAttribute{
				MarkdownDescription: "Whether to include extended statistics.",
				Optional:            true,
				Computed:            true,
			},
			"group_by": schema.StringAttribute{
				MarkdownDescription: "Field to group runs by.",
				Optional:            true,
			},
			"evaluators": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of evaluator configurations.",
				Optional:            true,
			},
			"code_evaluators": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of code evaluator configurations.",
				Optional:            true,
			},
			"alerts": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of alert configurations.",
				Optional:            true,
			},
			"webhooks": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of webhook configurations.",
				Optional:            true,
			},
			// Computed fields the API sends back -- read-only dispatches from the marshal's office.
			"session_name": schema.StringAttribute{
				MarkdownDescription: "The name of the associated session/project.",
				Computed:            true,
			},
			"dataset_name": schema.StringAttribute{
				MarkdownDescription: "The name of the associated dataset.",
				Computed:            true,
			},
			"corrections_dataset_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the corrections dataset.",
				Computed:            true,
			},
			"evaluator_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the evaluator.",
				Computed:            true,
			},
			"alignment_annotation_queue_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the alignment annotation queue.",
				Computed:            true,
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
	// Round up the rest of the posse -- new fields Matt Dillon would approve of.
	if !data.DatasetID.IsNull() && !data.DatasetID.IsUnknown() {
		v := data.DatasetID.ValueString()
		body.DatasetID = &v
	}
	if !data.BackfillFrom.IsNull() && !data.BackfillFrom.IsUnknown() {
		v := data.BackfillFrom.ValueString()
		body.BackfillFrom = &v
	}
	if !data.UseCorrectionsDataset.IsNull() && !data.UseCorrectionsDataset.IsUnknown() {
		v := data.UseCorrectionsDataset.ValueBool()
		body.UseCorrectionsDataset = &v
	}
	if !data.ExtendOnly.IsNull() && !data.ExtendOnly.IsUnknown() {
		v := data.ExtendOnly.ValueBool()
		body.ExtendOnly = &v
	}
	if !data.Transient.IsNull() && !data.Transient.IsUnknown() {
		v := data.Transient.ValueBool()
		body.Transient = &v
	}
	if !data.IncludeExtendedStats.IsNull() && !data.IncludeExtendedStats.IsUnknown() {
		v := data.IncludeExtendedStats.ValueBool()
		body.IncludeExtendedStats = &v
	}
	if !data.GroupBy.IsNull() && !data.GroupBy.IsUnknown() {
		v := data.GroupBy.ValueString()
		body.GroupBy = &v
	}
	if !data.Evaluators.IsNull() && !data.Evaluators.IsUnknown() {
		body.Evaluators = json.RawMessage(data.Evaluators.ValueString())
	}
	if !data.CodeEvaluators.IsNull() && !data.CodeEvaluators.IsUnknown() {
		body.CodeEvaluators = json.RawMessage(data.CodeEvaluators.ValueString())
	}
	if !data.Alerts.IsNull() && !data.Alerts.IsUnknown() {
		body.Alerts = json.RawMessage(data.Alerts.ValueString())
	}
	if !data.Webhooks.IsNull() && !data.Webhooks.IsUnknown() {
		body.Webhooks = json.RawMessage(data.Webhooks.ValueString())
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
	// Same drill as Create -- Festus would say "you gotta be thorough, Matthew."
	if !data.DatasetID.IsNull() && !data.DatasetID.IsUnknown() {
		v := data.DatasetID.ValueString()
		body.DatasetID = &v
	}
	if !data.BackfillFrom.IsNull() && !data.BackfillFrom.IsUnknown() {
		v := data.BackfillFrom.ValueString()
		body.BackfillFrom = &v
	}
	if !data.UseCorrectionsDataset.IsNull() && !data.UseCorrectionsDataset.IsUnknown() {
		v := data.UseCorrectionsDataset.ValueBool()
		body.UseCorrectionsDataset = &v
	}
	if !data.ExtendOnly.IsNull() && !data.ExtendOnly.IsUnknown() {
		v := data.ExtendOnly.ValueBool()
		body.ExtendOnly = &v
	}
	if !data.Transient.IsNull() && !data.Transient.IsUnknown() {
		v := data.Transient.ValueBool()
		body.Transient = &v
	}
	if !data.IncludeExtendedStats.IsNull() && !data.IncludeExtendedStats.IsUnknown() {
		v := data.IncludeExtendedStats.ValueBool()
		body.IncludeExtendedStats = &v
	}
	if !data.GroupBy.IsNull() && !data.GroupBy.IsUnknown() {
		v := data.GroupBy.ValueString()
		body.GroupBy = &v
	}
	if !data.Evaluators.IsNull() && !data.Evaluators.IsUnknown() {
		body.Evaluators = json.RawMessage(data.Evaluators.ValueString())
	}
	if !data.CodeEvaluators.IsNull() && !data.CodeEvaluators.IsUnknown() {
		body.CodeEvaluators = json.RawMessage(data.CodeEvaluators.ValueString())
	}
	if !data.Alerts.IsNull() && !data.Alerts.IsUnknown() {
		body.Alerts = json.RawMessage(data.Alerts.ValueString())
	}
	if !data.Webhooks.IsNull() && !data.Webhooks.IsUnknown() {
		body.Webhooks = json.RawMessage(data.Webhooks.ValueString())
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

// mapResponseToModel translates the API's response into Terraform state,
// setting null for any optional fields that came back empty from the territory.
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

	// Map the new arrivals -- every soul in Dodge City gets accounted for.
	if result.DatasetID != nil {
		data.DatasetID = types.StringValue(*result.DatasetID)
	} else {
		data.DatasetID = types.StringNull()
	}
	if result.BackfillFrom != nil {
		data.BackfillFrom = types.StringValue(*result.BackfillFrom)
	} else {
		data.BackfillFrom = types.StringNull()
	}
	data.UseCorrectionsDataset = types.BoolValue(result.UseCorrectionsDataset)
	data.ExtendOnly = types.BoolValue(result.ExtendOnly)
	data.Transient = types.BoolValue(result.Transient)
	data.IncludeExtendedStats = types.BoolValue(result.IncludeExtendedStats)
	if result.GroupBy != nil {
		data.GroupBy = types.StringValue(*result.GroupBy)
	} else {
		data.GroupBy = types.StringNull()
	}
	// JSON fields -- Doc Adams keeps meticulous records and so do we.
	if len(result.Evaluators) > 0 && string(result.Evaluators) != "null" {
		data.Evaluators = types.StringValue(string(result.Evaluators))
	} else {
		data.Evaluators = types.StringNull()
	}
	if len(result.CodeEvaluators) > 0 && string(result.CodeEvaluators) != "null" {
		data.CodeEvaluators = types.StringValue(string(result.CodeEvaluators))
	} else {
		data.CodeEvaluators = types.StringNull()
	}
	if len(result.Alerts) > 0 && string(result.Alerts) != "null" {
		data.Alerts = types.StringValue(string(result.Alerts))
	} else {
		data.Alerts = types.StringNull()
	}
	if len(result.Webhooks) > 0 && string(result.Webhooks) != "null" {
		data.Webhooks = types.StringValue(string(result.Webhooks))
	} else {
		data.Webhooks = types.StringNull()
	}
	// Computed fields -- dispatches that only the API can write.
	if result.SessionName != nil {
		data.SessionName = types.StringValue(*result.SessionName)
	} else {
		data.SessionName = types.StringNull()
	}
	if result.DatasetName != nil {
		data.DatasetName = types.StringValue(*result.DatasetName)
	} else {
		data.DatasetName = types.StringNull()
	}
	if result.CorrectionsDatasetID != nil {
		data.CorrectionsDatasetID = types.StringValue(*result.CorrectionsDatasetID)
	} else {
		data.CorrectionsDatasetID = types.StringNull()
	}
	if result.EvaluatorID != nil {
		data.EvaluatorID = types.StringValue(*result.EvaluatorID)
	} else {
		data.EvaluatorID = types.StringNull()
	}
	if result.AlignmentAnnotationQueueID != nil {
		data.AlignmentAnnotationQueueID = types.StringValue(*result.AlignmentAnnotationQueueID)
	} else {
		data.AlignmentAnnotationQueueID = types.StringNull()
	}
}

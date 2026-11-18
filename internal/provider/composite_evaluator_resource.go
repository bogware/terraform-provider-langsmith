// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                     = &CompositeEvaluatorResource{}
	_ resource.ResourceWithImportState      = &CompositeEvaluatorResource{}
	_ resource.ResourceWithConfigValidators = &CompositeEvaluatorResource{}
)

// NewCompositeEvaluatorResource returns the replacement for the retired
// langsmith_feedback_formula (#98).
func NewCompositeEvaluatorResource() resource.Resource {
	return &CompositeEvaluatorResource{}
}

// CompositeEvaluatorResource manages a composite feedback score: a weighted
// combination of other feedback keys, computed for each run.
//
// LangSmith implements it as two objects, created together by
// POST /api/v1/feedback/composite-evaluators (not in the published OpenAPI
// spec; shape established against the live API):
//
//   - a code evaluator that computes the score from the weights, and
//   - a run rule, scoped to the project or dataset, that runs it on traces
//     carrying the input feedback keys.
//
// There is no update endpoint and no way to read the formula back (the weights
// end up inside generated code), so every input forces replacement and Read
// only confirms both objects still exist.
type CompositeEvaluatorResource struct {
	client *client.Client
}

// CompositeEvaluatorResourceModel is the Terraform state.
type CompositeEvaluatorResourceModel struct {
	ID              types.String `tfsdk:"id"`
	FeedbackKey     types.String `tfsdk:"feedback_key"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	FormulaParts    types.String `tfsdk:"formula_parts"`
	SessionID       types.String `tfsdk:"session_id"`
	DatasetID       types.String `tfsdk:"dataset_id"`
	RuleID          types.String `tfsdk:"rule_id"`
	EvaluatorID     types.String `tfsdk:"evaluator_id"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
}

type compositeEvaluatorCreateRequest struct {
	FeedbackKey     string          `json:"feedback_key"`
	AggregationType string          `json:"aggregation_type"`
	FormulaParts    json.RawMessage `json:"formula_parts"`
	SessionID       *string         `json:"session_id,omitempty"`
	DatasetID       *string         `json:"dataset_id,omitempty"`
}

type compositeEvaluatorCreateResponse struct {
	RuleID      string `json:"rule_id"`
	EvaluatorID string `json:"evaluator_id"`
}

// compositeEvaluatorRule is the slice of a run rule Read needs.
type compositeEvaluatorRule struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
}

// compositeEvaluatorEvaluator is the slice of an evaluator Read needs.
type compositeEvaluatorEvaluator struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
}

func (r *CompositeEvaluatorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_composite_evaluator"
}

func (r *CompositeEvaluatorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith composite evaluator: a feedback score computed for each run as a weighted " +
			"combination of other feedback keys. It replaces the retired `langsmith_feedback_formula` and takes the same " +
			"inputs.\n\n" +
			"LangSmith creates two objects for it: a code evaluator that computes the score, and a run rule that runs it " +
			"on the project's or dataset's traces. This resource manages both, and destroying it deletes both. LangSmith " +
			"has no update endpoint for composite evaluators and does not return the formula, so changing any input " +
			"replaces the resource, and a change made to the generated evaluator code in the LangSmith UI is not detected.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The resource ID, equal to `rule_id`.",
				Computed:            true,
				PlanModifiers:       keep,
			},
			"feedback_key": schema.StringAttribute{
				MarkdownDescription: "The feedback key the composite score is recorded under. Changing it forces replacement.",
				Required:            true,
				PlanModifiers:       replace,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"aggregation_type": schema.StringAttribute{
				MarkdownDescription: "How the weighted parts combine: `avg` (weighted average) or `sum` (weighted sum). " +
					"Changing it forces replacement.",
				Required:      true,
				PlanModifiers: replace,
				Validators:    []validator.String{stringvalidator.OneOf("avg", "sum")},
			},
			"formula_parts": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of weighted feedback keys, e.g. " +
					"`jsonencode([{ part_type = \"weighted_key\", weight = 1.0, key = \"correctness\" }])`. The score is only " +
					"computed for a run once every key in the formula has a score. Changing it forces replacement.",
				Required:      true,
				PlanModifiers: replace,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project (session) to compute the score for. Exactly one of `session_id` or " +
					"`dataset_id` must be set. Changing it forces replacement.",
				Optional:      true,
				PlanModifiers: replace,
			},
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "The dataset to compute the score for. Exactly one of `session_id` or `dataset_id` " +
					"must be set. Changing it forces replacement.",
				Optional:      true,
				PlanModifiers: replace,
			},
			"rule_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the run rule LangSmith created to run the evaluator.",
				Computed:            true,
				PlanModifiers:       keep,
			},
			"evaluator_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the code evaluator LangSmith created to compute the score.",
				Computed:            true,
				PlanModifiers:       keep,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *CompositeEvaluatorResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("session_id"), path.MatchRoot("dataset_id")),
	}
}

func (r *CompositeEvaluatorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *CompositeEvaluatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CompositeEvaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := compositeEvaluatorCreateRequest{
		FeedbackKey:     data.FeedbackKey.ValueString(),
		AggregationType: data.AggregationType.ValueString(),
		FormulaParts:    json.RawMessage(data.FormulaParts.ValueString()),
	}
	setOptionalString(&body.SessionID, data.SessionID)
	setOptionalString(&body.DatasetID, data.DatasetID)

	c := effectiveClient(r.client, data.WorkspaceID)
	var result compositeEvaluatorCreateResponse
	if err := c.Post(ctx, "/api/v1/feedback/composite-evaluators", body, &result); err != nil {
		resp.Diagnostics.AddError("Error creating composite evaluator", err.Error())
		return
	}
	if result.RuleID == "" || result.EvaluatorID == "" {
		resp.Diagnostics.AddError("Error creating composite evaluator",
			"LangSmith did not return both a rule_id and an evaluator_id, so the created objects cannot be tracked.")
		return
	}

	data.ID = types.StringValue(result.RuleID)
	data.RuleID = types.StringValue(result.RuleID)
	data.EvaluatorID = types.StringValue(result.EvaluatorID)
	// The create response carries no workspace, so the evaluator is read once
	// to settle workspace_id; if that fails the client's workspace is used.
	var ev compositeEvaluatorEvaluator
	apiWorkspace := ""
	if err := c.Get(ctx, "/api/v1/platform/evaluators/"+result.EvaluatorID, nil, &ev); err == nil {
		apiWorkspace = firstNonEmpty(ev.WorkspaceID, ev.TenantID)
	}
	finalizeWorkspaceID(&data.WorkspaceID, c, apiWorkspace, &resp.Diagnostics)

	tflog.Trace(ctx, "created composite evaluator", map[string]interface{}{"rule_id": result.RuleID, "evaluator_id": result.EvaluatorID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read confirms the run rule and the evaluator still exist. The formula is not
// returned by the API, so the configured inputs stay as they are in state.
// If either object is gone the resource is removed from state, and the next
// apply recreates the pair.
func (r *CompositeEvaluatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CompositeEvaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)

	var ev compositeEvaluatorEvaluator
	if err := c.Get(ctx, "/api/v1/platform/evaluators/"+data.EvaluatorID.ValueString(), nil, &ev); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading composite evaluator", err.Error())
		return
	}

	// There is no GET for a single run rule; list and search, as
	// langsmith_run_rule does.
	var rules []compositeEvaluatorRule
	if err := c.Get(ctx, "/api/v1/runs/rules", nil, &rules); err != nil {
		resp.Diagnostics.AddError("Error reading composite evaluator run rule", err.Error())
		return
	}
	found := false
	for _, rule := range rules {
		if rule.ID == data.RuleID.ValueString() {
			found = true
			break
		}
	}
	if !found {
		resp.Diagnostics.AddWarning("Composite evaluator run rule is gone",
			fmt.Sprintf("Run rule %s, which runs evaluator %s, no longer exists, so the composite evaluator has been "+
				"removed from state and will be recreated. The leftover evaluator is not deleted automatically; "+
				"remove it in LangSmith if it is no longer needed.", data.RuleID.ValueString(), data.EvaluatorID.ValueString()))
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = data.RuleID
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(ev.WorkspaceID, ev.TenantID), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is never called with a real change: every configurable attribute
// forces replacement. It only carries state forward.
func (r *CompositeEvaluatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CompositeEvaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete removes both objects. Deleting the run rule alone leaves the
// evaluator behind, so both are always attempted.
func (r *CompositeEvaluatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CompositeEvaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	if err := c.Delete(ctx, "/api/v1/runs/rules/"+data.RuleID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting composite evaluator run rule", err.Error())
	}
	if err := c.Delete(ctx, "/api/v1/platform/evaluators/"+data.EvaluatorID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting composite evaluator", err.Error())
	}
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "deleted composite evaluator", map[string]interface{}{"rule_id": data.RuleID.ValueString()})
}

// ImportState takes "<rule_id>/<evaluator_id>": Read needs both, and neither
// can be derived from the other. The formula inputs cannot be imported because
// LangSmith does not return them, so the first plan after an import shows
// them as needing replacement unless configuration matches what was created.
func (r *CompositeEvaluatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'rule_id/evaluator_id', got: %s", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("rule_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("evaluator_id"), parts[1])...)
}

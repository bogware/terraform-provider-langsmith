// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

const evaluatorsAPIPath = "/v1/platform/evaluators"

var (
	_ resource.Resource                = &EvaluatorResource{}
	_ resource.ResourceWithImportState = &EvaluatorResource{}
)

// NewEvaluatorResource returns a resource for hosted evaluators
// (`/v1/platform/evaluators`, OpenAPI tag evaluators).
func NewEvaluatorResource() resource.Resource {
	return &EvaluatorResource{}
}

// EvaluatorResource manages LangSmith hosted evaluators.
type EvaluatorResource struct {
	client *client.Client
}

// EvaluatorResourceModel is Terraform state for one evaluator.
type EvaluatorResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	LLMEvaluator   types.String `tfsdk:"llm_evaluator"`
	CodeEvaluator  types.String `tfsdk:"code_evaluator"`
	DeleteRunRules types.Bool   `tfsdk:"delete_run_rules"`
	TenantID       types.String `tfsdk:"tenant_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	CreatedBy      types.String `tfsdk:"created_by"`
	FeedbackKeys   types.List   `tfsdk:"feedback_keys"`
	RunRules       types.String `tfsdk:"run_rules"`
}

// evaluatorAPI mirrors evaluators.Evaluator from the LangSmith OpenAPI spec.
type evaluatorAPI struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	TenantID      string          `json:"tenant_id"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	CreatedBy     string          `json:"created_by"`
	FeedbackKeys  []string        `json:"feedback_keys"`
	RunRules      json.RawMessage `json:"run_rules"`
	LLMEvaluator  json.RawMessage `json:"llm_evaluator"`
	CodeEvaluator json.RawMessage `json:"code_evaluator"`
}

type createEvaluatorAPIResponse struct {
	Evaluator evaluatorAPI `json:"evaluator"`
}

type updateEvaluatorAPIResponse struct {
	Evaluator evaluatorAPI `json:"evaluator"`
}

type evaluatorCreateRequest struct {
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	LLMEvaluator  json.RawMessage `json:"llm_evaluator,omitempty"`
	CodeEvaluator json.RawMessage `json:"code_evaluator,omitempty"`
}

type evaluatorPatchRequest struct {
	Name          *string         `json:"name,omitempty"`
	LLMEvaluator  json.RawMessage `json:"llm_evaluator,omitempty"`
	CodeEvaluator json.RawMessage `json:"code_evaluator,omitempty"`
}

func evaluatorDetailPath(id string) string {
	return evaluatorsAPIPath + "/" + url.PathEscape(id)
}

func (r *EvaluatorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator"
}

func (r *EvaluatorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith **hosted evaluator** (`POST/PATCH/DELETE /v1/platform/evaluators`, `GET /v1/platform/evaluators/{evaluator_id}`). " +
			"Use `type` `llm` with `llm_evaluator` or `type` `code` with `code_evaluator` as JSON objects matching the OpenAPI `evaluators.CreateEvaluatorRequest` nested payloads. " +
			"On read, `llm_evaluator` and `code_evaluator` contain JSON for `evaluators.LLMEvaluator` / `evaluators.CodeEvaluator` as returned by the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Evaluator ID (`evaluator_id` in the API).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name (`name` in the API).",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Evaluator discriminator: `llm` or `code` (`evaluators.EvaluatorType`).",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("llm", "code")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"llm_evaluator": schema.StringAttribute{
				MarkdownDescription: "JSON object for `llm_evaluator` on create/update (`evaluators.CreateLLMEvaluatorRequest` / `evaluators.UpdateLLMEvaluatorRequest` fields). Conflicts with `code_evaluator`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("code_evaluator")),
				},
			},
			"code_evaluator": schema.StringAttribute{
				MarkdownDescription: "JSON object for `code_evaluator` on create/update (`evaluators.CreateCodeEvaluatorRequest` / `evaluators.UpdateCodeEvaluatorRequest` fields). Conflicts with `llm_evaluator`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("llm_evaluator")),
				},
			},
			"delete_run_rules": schema.BoolAttribute{
				MarkdownDescription: "When true, `DELETE` sends `delete_run_rules=true` so the API removes dependent run rules before deleting the evaluator.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Workspace tenant ID owning the evaluator.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp from the API.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp from the API.",
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "Creator user ID from the API.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"feedback_keys": schema.ListAttribute{
				MarkdownDescription: "Feedback keys associated with this evaluator (`feedback_keys` in the API).",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"run_rules": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded `run_rules` array from the API (`evaluators.EvaluatorRunRule`).",
				Computed:            true,
			},
		},
	}
}

func (r *EvaluatorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EvaluatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EvaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	llmRaw, codeRaw, diags := validateEvaluatorPlan(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := evaluatorCreateRequest{
		Name: plan.Name.ValueString(),
		Type: plan.Type.ValueString(),
	}
	if len(llmRaw) > 0 {
		body.LLMEvaluator = llmRaw
	}
	if len(codeRaw) > 0 {
		body.CodeEvaluator = codeRaw
	}

	var wrap createEvaluatorAPIResponse
	if err := r.client.Post(ctx, evaluatorsAPIPath, body, &wrap); err != nil {
		resp.Diagnostics.AddError("Error creating evaluator", err.Error())
		return
	}

	diags = applyEvaluatorAPIToModel(&wrap.Evaluator, plan.DeleteRunRules, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created evaluator resource", map[string]any{"id": wrap.Evaluator.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EvaluatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EvaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	preserveDeleteRunRules := data.DeleteRunRules
	if preserveDeleteRunRules.IsUnknown() || preserveDeleteRunRules.IsNull() {
		preserveDeleteRunRules = types.BoolValue(false)
	}

	var ev evaluatorAPI
	if err := r.client.Get(ctx, evaluatorDetailPath(data.ID.ValueString()), nil, &ev); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading evaluator", err.Error())
		return
	}

	diags := applyEvaluatorAPIToModel(&ev, preserveDeleteRunRules, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EvaluatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state EvaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	llmRaw, codeRaw, diags := validateEvaluatorPlan(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := evaluatorPatchRequest{
		Name: stringPtr(plan.Name.ValueString()),
	}
	if len(llmRaw) > 0 && !evaluatorJSONAttrEqual(state.LLMEvaluator, plan.LLMEvaluator) {
		patch.LLMEvaluator = llmRaw
	}
	if len(codeRaw) > 0 && !evaluatorJSONAttrEqual(state.CodeEvaluator, plan.CodeEvaluator) {
		patch.CodeEvaluator = codeRaw
	}

	var wrap updateEvaluatorAPIResponse
	if err := r.client.Patch(ctx, evaluatorDetailPath(state.ID.ValueString()), patch, &wrap); err != nil {
		resp.Diagnostics.AddError("Error updating evaluator", err.Error())
		return
	}

	diags = applyEvaluatorAPIToModel(&wrap.Evaluator, plan.DeleteRunRules, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "updated evaluator resource", map[string]any{"id": wrap.Evaluator.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EvaluatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EvaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := evaluatorDetailPath(data.ID.ValueString())
	var err error
	if !data.DeleteRunRules.IsNull() && !data.DeleteRunRules.IsUnknown() && data.DeleteRunRules.ValueBool() {
		q := url.Values{}
		q.Set("delete_run_rules", "true")
		err = r.client.DeleteWithQuery(ctx, path, q)
	} else {
		err = r.client.Delete(ctx, path)
	}
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting evaluator", err.Error())
		return
	}
	tflog.Trace(ctx, "deleted evaluator resource", map[string]any{"id": data.ID.ValueString()})
}

func (r *EvaluatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func stringPtr(s string) *string {
	return &s
}

func evaluatorJSONAttrEqual(state, plan types.String) bool {
	switch {
	case plan.IsUnknown():
		return true
	case plan.IsNull() && state.IsNull():
		return true
	case plan.IsNull() || state.IsNull():
		return false
	default:
		return normalizeJSON(state.ValueString()) == normalizeJSON(plan.ValueString())
	}
}

func validateEvaluatorPlan(plan EvaluatorResourceModel) (json.RawMessage, json.RawMessage, diag.Diagnostics) {
	llmRaw, llmSet, diags := parseEvaluatorJSONAttribute(plan.LLMEvaluator, "llm_evaluator")
	codeRaw, codeSet, codeDiags := parseEvaluatorJSONAttribute(plan.CodeEvaluator, "code_evaluator")
	diags.Append(codeDiags...)
	switch plan.Type.ValueString() {
	case "llm":
		if !llmSet {
			diags.AddAttributeError(
				path.Root("llm_evaluator"),
				"Missing llm_evaluator for type llm",
				`Set "llm_evaluator" to a non-empty valid JSON object when "type" is "llm".`,
			)
		}
		if codeSet {
			diags.AddAttributeError(
				path.Root("code_evaluator"),
				"Invalid code_evaluator for type llm",
				`Remove "code_evaluator" when "type" is "llm".`,
			)
		}
	case "code":
		if !codeSet {
			diags.AddAttributeError(
				path.Root("code_evaluator"),
				"Missing code_evaluator for type code",
				`Set "code_evaluator" to a non-empty valid JSON object when "type" is "code".`,
			)
		}
		if llmSet {
			diags.AddAttributeError(
				path.Root("llm_evaluator"),
				"Invalid llm_evaluator for type code",
				`Remove "llm_evaluator" when "type" is "code".`,
			)
		}
	}
	return llmRaw, codeRaw, diags
}

func parseEvaluatorJSONAttribute(s types.String, attr string) (json.RawMessage, bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	if s.IsNull() || s.IsUnknown() {
		return nil, false, diags
	}
	v := strings.TrimSpace(s.ValueString())
	if v == "" {
		diags.AddAttributeError(
			path.Root(attr),
			"Invalid JSON value",
			fmt.Sprintf(`"%s" must be non-empty valid JSON.`, attr),
		)
		return nil, false, diags
	}
	if !json.Valid([]byte(v)) {
		diags.AddAttributeError(
			path.Root(attr),
			"Invalid JSON value",
			fmt.Sprintf(`"%s" must be valid JSON.`, attr),
		)
		return nil, false, diags
	}
	return json.RawMessage(v), true, diags
}

func applyEvaluatorAPIToModel(ev *evaluatorAPI, deleteRunRules types.Bool, dst *EvaluatorResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	dst.ID = types.StringValue(ev.ID)
	dst.Name = types.StringValue(ev.Name)
	dst.Type = types.StringValue(ev.Type)
	dst.TenantID = types.StringValue(ev.TenantID)
	dst.CreatedAt = types.StringValue(ev.CreatedAt)
	dst.UpdatedAt = types.StringValue(ev.UpdatedAt)
	dst.CreatedBy = types.StringValue(ev.CreatedBy)
	dst.DeleteRunRules = deleteRunRules

	listVals, ldiags := types.ListValue(types.StringType, stringSliceToAttrValues(ev.FeedbackKeys))
	diags.Append(ldiags...)
	dst.FeedbackKeys = listVals

	dst.RunRules = jsonStringValue(ev.RunRules)

	switch ev.Type {
	case "code":
		dst.CodeEvaluator = jsonStringValue(ev.CodeEvaluator)
		dst.LLMEvaluator = types.StringNull()
	case "llm":
		dst.LLMEvaluator = jsonStringValue(ev.LLMEvaluator)
		dst.CodeEvaluator = types.StringNull()
	default:
		dst.LLMEvaluator = jsonStringValue(ev.LLMEvaluator)
		dst.CodeEvaluator = jsonStringValue(ev.CodeEvaluator)
	}
	return diags
}

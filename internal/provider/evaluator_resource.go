// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &EvaluatorResource{}
	_ resource.ResourceWithImportState = &EvaluatorResource{}
)

func NewEvaluatorResource() resource.Resource {
	return &EvaluatorResource{}
}

type EvaluatorResource struct {
	client *client.Client
}

type EvaluatorResourceModel struct {
	ID            types.String        `tfsdk:"id"`
	Name          types.String        `tfsdk:"name"`
	Type          types.String        `tfsdk:"type"`
	TenantID      types.String        `tfsdk:"tenant_id"`
	CodeEvaluator *codeEvaluatorModel `tfsdk:"code_evaluator"`
	LLMEvaluator  *llmEvaluatorModel  `tfsdk:"llm_evaluator"`
	CreatedAt     types.String        `tfsdk:"created_at"`
	UpdatedAt     types.String        `tfsdk:"updated_at"`
	CreatedBy     types.String        `tfsdk:"created_by"`
	FeedbackKeys  types.List          `tfsdk:"feedback_keys"`
}

type codeEvaluatorModel struct {
	Code     types.String `tfsdk:"code"`
	Language types.String `tfsdk:"language"`
}

type llmEvaluatorModel struct {
	PromptRepoHandle   types.String `tfsdk:"prompt_repo_handle"`
	CommitHashOrTag    types.String `tfsdk:"commit_hash_or_tag"`
	VariableMapping    types.String `tfsdk:"variable_mapping"`
	NumFewShotExamples types.Int64  `tfsdk:"num_few_shot_examples"`
	UseCorrectionsData types.Bool   `tfsdk:"use_corrections_dataset"`
}

type evaluatorCodePayload struct {
	Code     *string `json:"code,omitempty"`
	Language *string `json:"language,omitempty"`
}

type evaluatorLLMCreatePayload struct {
	PromptRepoHandle *string                `json:"prompt_repo_handle,omitempty"`
	CommitHashOrTag  *string                `json:"commit_hash_or_tag,omitempty"`
	VariableMapping  map[string]interface{} `json:"variable_mapping,omitempty"`
}

type evaluatorLLMUpdatePayload struct {
	PromptRepoHandle      *string                `json:"prompt_repo_handle,omitempty"`
	CommitHashOrTag       *string                `json:"commit_hash_or_tag,omitempty"`
	VariableMapping       map[string]interface{} `json:"variable_mapping,omitempty"`
	NumFewShotExamples    *int64                 `json:"num_few_shot_examples,omitempty"`
	UseCorrectionsDataset *bool                  `json:"use_corrections_dataset,omitempty"`
}

type evaluatorCreateRequest struct {
	Name          string                     `json:"name"`
	Type          string                     `json:"type"`
	CodeEvaluator *evaluatorCodePayload      `json:"code_evaluator,omitempty"`
	LLMEvaluator  *evaluatorLLMCreatePayload `json:"llm_evaluator,omitempty"`
}

type evaluatorUpdateRequest struct {
	Name          *string                    `json:"name,omitempty"`
	CodeEvaluator *evaluatorCodePayload      `json:"code_evaluator,omitempty"`
	LLMEvaluator  *evaluatorLLMUpdatePayload `json:"llm_evaluator,omitempty"`
}

type evaluatorAPI struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	TenantID      string            `json:"tenant_id"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	CreatedBy     string            `json:"created_by"`
	FeedbackKeys  []string          `json:"feedback_keys"`
	CodeEvaluator *evaluatorCodeAPI `json:"code_evaluator,omitempty"`
	LLMEvaluator  *evaluatorLLMAPI  `json:"llm_evaluator,omitempty"`
}

type evaluatorCodeAPI struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

type evaluatorLLMAPI struct {
	PromptRepoHandle      string                 `json:"prompt_repo_handle"`
	CommitHashOrTag       string                 `json:"commit_hash_or_tag"`
	VariableMapping       map[string]interface{} `json:"variable_mapping"`
	NumFewShotExamples    *int64                 `json:"num_few_shot_examples"`
	UseCorrectionsDataset *bool                  `json:"use_corrections_dataset"`
}

type evaluatorCreateResponse struct {
	Evaluator evaluatorAPI `json:"evaluator"`
}

func (r *EvaluatorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator"
}

func (r *EvaluatorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith evaluator. An evaluator scores runs either by executing user code (`type = \"code\"`) or by invoking a prompt as an LLM-as-judge (`type = \"llm\"`). Exactly one of `code_evaluator` or `llm_evaluator` must be set, matching `type`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the evaluator.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable evaluator name.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Evaluator type. One of `code` or `llm`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("code", "llm")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The workspace/tenant the evaluator lives in.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"code_evaluator": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for a code evaluator. Required when `type = \"code\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"code": schema.StringAttribute{
						MarkdownDescription: "Source code for the evaluator. The entry-point function must be named `perform_eval(run, example)` (e.g. `def perform_eval(run, example): return {\"score\": 1}`).",
						Required:            true,
					},
					"language": schema.StringAttribute{
						MarkdownDescription: "Language of the evaluator code (defaults server-side to `python`).",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},
			"llm_evaluator": schema.SingleNestedAttribute{
				MarkdownDescription: "Configuration for an LLM-as-judge evaluator. Required when `type = \"llm\"`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"prompt_repo_handle": schema.StringAttribute{
						MarkdownDescription: "Handle of the prompt repo (LangSmith Hub) to use as the judge prompt.",
						Required:            true,
					},
					"commit_hash_or_tag": schema.StringAttribute{
						MarkdownDescription: "Commit hash or named tag of the prompt to pin. Defaults to the latest commit when omitted.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"variable_mapping": schema.StringAttribute{
						MarkdownDescription: "JSON-encoded object that maps prompt variables to run fields.",
						Optional:            true,
					},
					"num_few_shot_examples": schema.Int64Attribute{
						MarkdownDescription: "Number of few-shot examples to draw from a corrections dataset on each invocation.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
					},
					"use_corrections_dataset": schema.BoolAttribute{
						MarkdownDescription: "Whether to source few-shot examples from a corrections dataset attached via a run rule.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
				},
			},
			"feedback_keys": schema.ListAttribute{
				MarkdownDescription: "Feedback keys this evaluator writes to. Derived server-side from `name`, so changing `name` changes this set.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "Identity that created the evaluator.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *EvaluatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EvaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := evaluatorCreateRequest{
		Name: data.Name.ValueString(),
		Type: data.Type.ValueString(),
	}

	switch body.Type {
	case "code":
		if data.CodeEvaluator == nil {
			resp.Diagnostics.AddError("Missing code_evaluator", "type=\"code\" requires the code_evaluator block.")
			return
		}
		body.CodeEvaluator = &evaluatorCodePayload{}
		if !data.CodeEvaluator.Code.IsNull() && !data.CodeEvaluator.Code.IsUnknown() {
			v := data.CodeEvaluator.Code.ValueString()
			body.CodeEvaluator.Code = &v
		}
		if !data.CodeEvaluator.Language.IsNull() && !data.CodeEvaluator.Language.IsUnknown() {
			v := data.CodeEvaluator.Language.ValueString()
			body.CodeEvaluator.Language = &v
		}
	case "llm":
		if data.LLMEvaluator == nil {
			resp.Diagnostics.AddError("Missing llm_evaluator", "type=\"llm\" requires the llm_evaluator block.")
			return
		}
		llm := &evaluatorLLMCreatePayload{}
		if !data.LLMEvaluator.PromptRepoHandle.IsNull() {
			v := data.LLMEvaluator.PromptRepoHandle.ValueString()
			llm.PromptRepoHandle = &v
		}
		if !data.LLMEvaluator.CommitHashOrTag.IsNull() && !data.LLMEvaluator.CommitHashOrTag.IsUnknown() {
			v := data.LLMEvaluator.CommitHashOrTag.ValueString()
			llm.CommitHashOrTag = &v
		}
		if !data.LLMEvaluator.VariableMapping.IsNull() && !data.LLMEvaluator.VariableMapping.IsUnknown() && data.LLMEvaluator.VariableMapping.ValueString() != "" {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(data.LLMEvaluator.VariableMapping.ValueString()), &m); err != nil {
				resp.Diagnostics.AddError("Invalid variable_mapping JSON", err.Error())
				return
			}
			llm.VariableMapping = m
		}
		body.LLMEvaluator = llm
	}

	var created evaluatorCreateResponse
	if err := r.client.Post(ctx, "/v1/platform/evaluators", body, &created); err != nil {
		resp.Diagnostics.AddError("Error creating evaluator", err.Error())
		return
	}

	r.mapResponseToModel(ctx, &created.Evaluator, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created evaluator", map[string]interface{}{"id": created.Evaluator.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EvaluatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EvaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api evaluatorAPI
	if err := r.client.Get(ctx, "/v1/platform/evaluators/"+data.ID.ValueString(), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading evaluator", err.Error())
		return
	}

	r.mapResponseToModel(ctx, &api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EvaluatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EvaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := evaluatorUpdateRequest{}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}

	switch data.Type.ValueString() {
	case "code":
		if data.CodeEvaluator != nil {
			body.CodeEvaluator = &evaluatorCodePayload{}
			if !data.CodeEvaluator.Code.IsNull() && !data.CodeEvaluator.Code.IsUnknown() {
				v := data.CodeEvaluator.Code.ValueString()
				body.CodeEvaluator.Code = &v
			}
			if !data.CodeEvaluator.Language.IsNull() && !data.CodeEvaluator.Language.IsUnknown() {
				v := data.CodeEvaluator.Language.ValueString()
				body.CodeEvaluator.Language = &v
			}
		}
	case "llm":
		if data.LLMEvaluator != nil {
			llm := &evaluatorLLMUpdatePayload{}
			if !data.LLMEvaluator.PromptRepoHandle.IsNull() && !data.LLMEvaluator.PromptRepoHandle.IsUnknown() {
				v := data.LLMEvaluator.PromptRepoHandle.ValueString()
				llm.PromptRepoHandle = &v
			}
			if !data.LLMEvaluator.CommitHashOrTag.IsNull() && !data.LLMEvaluator.CommitHashOrTag.IsUnknown() {
				v := data.LLMEvaluator.CommitHashOrTag.ValueString()
				llm.CommitHashOrTag = &v
			}
			if !data.LLMEvaluator.VariableMapping.IsNull() && !data.LLMEvaluator.VariableMapping.IsUnknown() && data.LLMEvaluator.VariableMapping.ValueString() != "" {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(data.LLMEvaluator.VariableMapping.ValueString()), &m); err != nil {
					resp.Diagnostics.AddError("Invalid variable_mapping JSON", err.Error())
					return
				}
				llm.VariableMapping = m
			}
			if !data.LLMEvaluator.NumFewShotExamples.IsNull() && !data.LLMEvaluator.NumFewShotExamples.IsUnknown() {
				v := data.LLMEvaluator.NumFewShotExamples.ValueInt64()
				llm.NumFewShotExamples = &v
			}
			if !data.LLMEvaluator.UseCorrectionsData.IsNull() && !data.LLMEvaluator.UseCorrectionsData.IsUnknown() {
				v := data.LLMEvaluator.UseCorrectionsData.ValueBool()
				llm.UseCorrectionsDataset = &v
			}
			body.LLMEvaluator = llm
		}
	}

	var updated evaluatorCreateResponse
	if err := r.client.Patch(ctx, "/v1/platform/evaluators/"+data.ID.ValueString(), body, &updated); err != nil {
		resp.Diagnostics.AddError("Error updating evaluator", err.Error())
		return
	}

	r.mapResponseToModel(ctx, &updated.Evaluator, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EvaluatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EvaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Delete(ctx, "/v1/platform/evaluators/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting evaluator", err.Error())
		return
	}
}

func (r *EvaluatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *EvaluatorResource) mapResponseToModel(ctx context.Context, api *evaluatorAPI, data *EvaluatorResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.Type = types.StringValue(api.Type)
	data.TenantID = types.StringValue(api.TenantID)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	data.CreatedBy = types.StringValue(api.CreatedBy)

	if len(api.FeedbackKeys) > 0 {
		fk, d := types.ListValueFrom(ctx, types.StringType, api.FeedbackKeys)
		diags.Append(d...)
		data.FeedbackKeys = fk
	} else {
		data.FeedbackKeys = types.ListNull(types.StringType)
	}

	if api.CodeEvaluator != nil {
		data.CodeEvaluator = &codeEvaluatorModel{
			Code:     types.StringValue(api.CodeEvaluator.Code),
			Language: types.StringValue(api.CodeEvaluator.Language),
		}
	}
	if api.LLMEvaluator != nil {
		llm := &llmEvaluatorModel{
			PromptRepoHandle: types.StringValue(api.LLMEvaluator.PromptRepoHandle),
			CommitHashOrTag:  types.StringValue(api.LLMEvaluator.CommitHashOrTag),
		}
		if len(api.LLMEvaluator.VariableMapping) > 0 {
			b, _ := json.Marshal(api.LLMEvaluator.VariableMapping)
			llm.VariableMapping = jsonStringValue(b)
		} else {
			llm.VariableMapping = types.StringNull()
		}
		if api.LLMEvaluator.NumFewShotExamples != nil {
			llm.NumFewShotExamples = types.Int64Value(*api.LLMEvaluator.NumFewShotExamples)
		} else {
			llm.NumFewShotExamples = types.Int64Null()
		}
		if api.LLMEvaluator.UseCorrectionsDataset != nil {
			llm.UseCorrectionsData = types.BoolValue(*api.LLMEvaluator.UseCorrectionsDataset)
		} else {
			llm.UseCorrectionsData = types.BoolNull()
		}
		data.LLMEvaluator = llm
	}
}

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &GatewayPolicyResource{}
	_ resource.ResourceWithImportState = &GatewayPolicyResource{}
)

// NewGatewayPolicyResource returns a resource for LangSmith LLM Gateway policies
// (spend caps, default spend caps, and guardrails).
func NewGatewayPolicyResource() resource.Resource {
	return &GatewayPolicyResource{}
}

// GatewayPolicyResource manages organization-level gateway policies.
type GatewayPolicyResource struct {
	client *client.Client
}

// GatewayPolicyResourceModel is Terraform state for a gateway policy.
type GatewayPolicyResourceModel struct {
	ID                  types.String  `tfsdk:"id"`
	Name                types.String  `tfsdk:"name"`
	Description         types.String  `tfsdk:"description"`
	PolicyType          types.String  `tfsdk:"policy_type"`
	Action              types.String  `tfsdk:"action"`
	SubjectMatchers     types.String  `tfsdk:"subject_matchers"`
	Config              types.String  `tfsdk:"config"`
	Enabled             types.Bool    `tfsdk:"enabled"`
	Priority            types.Int64   `tfsdk:"priority"`
	OrganizationID      types.String  `tfsdk:"organization_id"`
	ParentPolicyID      types.String  `tfsdk:"parent_policy_id"`
	IsSystemGenerated   types.Bool    `tfsdk:"is_system_generated"`
	CurrentSpendUSD     types.Float64 `tfsdk:"current_spend_usd"`
	CreatedAt           types.String  `tfsdk:"created_at"`
	UpdatedAt           types.String  `tfsdk:"updated_at"`
	CreatedBy           types.String  `tfsdk:"created_by"`
}

type gatewaySubjectMatcher struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type gatewayPolicyCreateRequest struct {
	Name            string                  `json:"name"`
	PolicyType      string                  `json:"policy_type"`
	Action          string                  `json:"action"`
	SubjectMatchers []gatewaySubjectMatcher `json:"subject_matchers"`
	Config          json.RawMessage         `json:"config"`
	Description     *string                 `json:"description,omitempty"`
	Enabled         *bool                   `json:"enabled,omitempty"`
	Priority        *int64                  `json:"priority,omitempty"`
}

type gatewayPolicyUpdateRequest struct {
	Name            *string                  `json:"name,omitempty"`
	Description     *string                  `json:"description,omitempty"`
	Action          *string                  `json:"action,omitempty"`
	SubjectMatchers []gatewaySubjectMatcher  `json:"subject_matchers,omitempty"`
	Config          json.RawMessage          `json:"config,omitempty"`
	Enabled         *bool                    `json:"enabled,omitempty"`
	Priority        *int64                   `json:"priority,omitempty"`
}

type gatewayPolicyRecord struct {
	ID                  string                  `json:"id"`
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	PolicyType          string                  `json:"policy_type"`
	Action              string                  `json:"action"`
	SubjectMatchers     []gatewaySubjectMatcher `json:"subject_matchers"`
	Config              json.RawMessage         `json:"config"`
	Enabled             bool                    `json:"enabled"`
	Priority            int64                   `json:"priority"`
	OrganizationID      string                  `json:"organization_id"`
	ParentPolicyID      *string                 `json:"parent_policy_id"`
	IsSystemGenerated   bool                    `json:"is_system_generated"`
	CurrentSpendUSD     *float64                `json:"current_spend_usd"`
	CreatedAt           string                  `json:"created_at"`
	UpdatedAt           string                  `json:"updated_at"`
	CreatedBy           string                  `json:"created_by"`
}

func (r *GatewayPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gateway_policy"
}

func (r *GatewayPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith **LLM Gateway** policy for the organization (`/v1/platform/gateway-policies`). " +
			"Policies are evaluated on gateway traffic; they are **not** the same as workspace `langsmith_secret` values. " +
			"**Guard** policies may enable `detect.secrets` / `detect.pii` in the policy `config` to redact model traffic — that refers to secrets embedded in prompts/responses, not LangSmith secret store keys. " +
			"Use `subject_matchers` to scope a policy (for example `workspace_id` to a single workspace, or `organization_id` for the whole org). " +
			"**Requires** `organization_id` on the provider (or `LANGSMITH_ORGANIZATION_ID`) so requests include `X-Organization-Id`, and an org API key with gateway permissions. " +
			"**Note:** `policy_type` cannot be changed in place; update the resource in Terraform by replacing it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Gateway policy UUID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Policy name (unique per organization).",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional human-readable description.",
				Optional:            true,
			},
			"policy_type": schema.StringAttribute{
				MarkdownDescription: "One of `spend_cap`, `default_spend_cap`, or `guard`. Immutable after create.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.OneOf("spend_cap", "default_spend_cap", "guard")},
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Enforcement action. The API currently supports `block`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("block")},
			},
			"subject_matchers": schema.StringAttribute{
				MarkdownDescription: "JSON array of `{\"key\",\"value\"}` matchers. `key` is one of `organization_id`, `workspace_id`, `user_id`, `api_key_id`. Matchers are ANDed. " +
					"For `default_spend_cap`, the API may use `{ \"key\": \"...\", \"value\": \"\" }` so runtime children are materialized per subject.",
				Required: true,
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "JSON policy configuration. Shape depends on `policy_type` (see LangSmith OpenAPI `gateway_policies.CreateGatewayPolicyRequest`). " +
					"Examples: spend cap `{\"window\":\"monthly\",\"limit_usd\":100}`; guard `{\"version\":1,\"detect\":{\"pii\":true,\"secrets\":true}}`.",
				Required: true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy is active. Omit to use the API default on create.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "Evaluation priority (lower runs first). Omit to use the API default on create.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Organization that owns this policy (from the API).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"parent_policy_id": schema.StringAttribute{
				MarkdownDescription: "Set on materialized children of a `default_spend_cap` template policy.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"is_system_generated": schema.BoolAttribute{
				MarkdownDescription: "True when the row was materialized or managed by the platform.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"current_spend_usd": schema.Float64Attribute{
				MarkdownDescription: "Spend accumulated in the active window for spend-cap policies; unset for guard policies.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Float64{float64planmodifier.UseStateForUnknown()},
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
				MarkdownDescription: "Actor that created the policy, when exposed by the API.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *GatewayPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GatewayPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := buildGatewayPolicyCreateRequest(&data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created gatewayPolicyRecord
	err := r.client.Post(ctx, "/v1/platform/gateway-policies", body, &created)
	if err != nil {
		resp.Diagnostics.AddError("Error creating gateway policy", err.Error())
		return
	}

	mapGatewayPolicyRecordToState(&data, &created)
	tflog.Trace(ctx, "created gateway policy", map[string]any{"id": created.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GatewayPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rec gatewayPolicyRecord
	err := r.client.Get(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString(), nil, &rec)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading gateway policy", err.Error())
		return
	}

	mapGatewayPolicyRecordToState(&data, &rec)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GatewayPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := buildGatewayPolicyUpdateRequest(&data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updated gatewayPolicyRecord
	err := r.client.Patch(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString(), body, &updated)
	if err != nil {
		resp.Diagnostics.AddError("Error updating gateway policy", err.Error())
		return
	}

	mapGatewayPolicyRecordToState(&data, &updated)
	tflog.Trace(ctx, "updated gateway policy", map[string]any{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GatewayPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting gateway policy", err.Error())
		return
	}
	tflog.Trace(ctx, "deleted gateway policy", map[string]any{"id": data.ID.ValueString()})
}

func (r *GatewayPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func parseGatewaySubjectMatchersJSON(s string) ([]gatewaySubjectMatcher, error) {
	if !json.Valid([]byte(s)) {
		return nil, fmt.Errorf("invalid JSON")
	}
	var matchers []gatewaySubjectMatcher
	if err := json.Unmarshal([]byte(s), &matchers); err != nil {
		return nil, err
	}
	for i := range matchers {
		if matchers[i].Key == "" {
			return nil, fmt.Errorf("subject_matchers[%d] missing key", i)
		}
	}
	return matchers, nil
}

func buildGatewayPolicyCreateRequest(data *GatewayPolicyResourceModel) (*gatewayPolicyCreateRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	matchers, err := parseGatewaySubjectMatchersJSON(data.SubjectMatchers.ValueString())
	if err != nil {
		diags.AddError("Invalid subject_matchers", err.Error())
		return nil, diags
	}
	configJSON := data.Config.ValueString()
	if !json.Valid([]byte(configJSON)) {
		diags.AddError("Invalid config", "config must be valid JSON")
		return nil, diags
	}

	body := &gatewayPolicyCreateRequest{
		Name:            data.Name.ValueString(),
		PolicyType:      data.PolicyType.ValueString(),
		Action:          data.Action.ValueString(),
		SubjectMatchers: matchers,
		Config:          json.RawMessage(configJSON),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		v := data.Enabled.ValueBool()
		body.Enabled = &v
	}
	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		v := data.Priority.ValueInt64()
		body.Priority = &v
	}
	return body, diags
}

func buildGatewayPolicyUpdateRequest(data *GatewayPolicyResourceModel) (*gatewayPolicyUpdateRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	matchers, err := parseGatewaySubjectMatchersJSON(data.SubjectMatchers.ValueString())
	if err != nil {
		diags.AddError("Invalid subject_matchers", err.Error())
		return nil, diags
	}
	configJSON := data.Config.ValueString()
	if !json.Valid([]byte(configJSON)) {
		diags.AddError("Invalid config", "config must be valid JSON")
		return nil, diags
	}

	n := data.Name.ValueString()
	a := data.Action.ValueString()
	body := &gatewayPolicyUpdateRequest{
		Name:            &n,
		Action:          &a,
		SubjectMatchers: matchers,
		Config:          json.RawMessage(configJSON),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		v := data.Enabled.ValueBool()
		body.Enabled = &v
	}
	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		v := data.Priority.ValueInt64()
		body.Priority = &v
	}
	return body, diags
}

func mapGatewayPolicyRecordToState(data *GatewayPolicyResourceModel, rec *gatewayPolicyRecord) {
	data.ID = types.StringValue(rec.ID)
	data.Name = types.StringValue(rec.Name)
	data.PolicyType = types.StringValue(rec.PolicyType)
	data.Action = types.StringValue(rec.Action)
	data.Enabled = types.BoolValue(rec.Enabled)
	data.Priority = types.Int64Value(rec.Priority)
	data.IsSystemGenerated = types.BoolValue(rec.IsSystemGenerated)

	if rec.Description != "" {
		data.Description = types.StringValue(rec.Description)
	} else {
		data.Description = types.StringNull()
	}

	sm, err := json.Marshal(rec.SubjectMatchers)
	if err == nil {
		data.SubjectMatchers = types.StringValue(normalizeJSON(string(sm)))
	} else {
		data.SubjectMatchers = types.StringValue("[]")
	}
	data.Config = jsonStringValue(rec.Config)

	if rec.OrganizationID != "" {
		data.OrganizationID = types.StringValue(rec.OrganizationID)
	} else {
		data.OrganizationID = types.StringNull()
	}
	if rec.ParentPolicyID != nil && *rec.ParentPolicyID != "" {
		data.ParentPolicyID = types.StringValue(*rec.ParentPolicyID)
	} else {
		data.ParentPolicyID = types.StringNull()
	}
	if rec.CurrentSpendUSD != nil {
		data.CurrentSpendUSD = types.Float64Value(*rec.CurrentSpendUSD)
	} else {
		data.CurrentSpendUSD = types.Float64Null()
	}
	if rec.CreatedAt != "" {
		data.CreatedAt = types.StringValue(rec.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if rec.UpdatedAt != "" {
		data.UpdatedAt = types.StringValue(rec.UpdatedAt)
	} else {
		data.UpdatedAt = types.StringNull()
	}
	if rec.CreatedBy != "" {
		data.CreatedBy = types.StringValue(rec.CreatedBy)
	} else {
		data.CreatedBy = types.StringNull()
	}
}

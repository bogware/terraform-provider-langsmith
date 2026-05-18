// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &GatewayPolicyResource{}
	_ resource.ResourceWithImportState = &GatewayPolicyResource{}
)

func NewGatewayPolicyResource() resource.Resource {
	return &GatewayPolicyResource{}
}

type GatewayPolicyResource struct {
	client *client.Client
}

type GatewayPolicyResourceModel struct {
	ID                types.String  `tfsdk:"id"`
	Name              types.String  `tfsdk:"name"`
	Description       types.String  `tfsdk:"description"`
	PolicyType        types.String  `tfsdk:"policy_type"`
	Action            types.String  `tfsdk:"action"`
	Priority          types.Int64   `tfsdk:"priority"`
	Enabled           types.Bool    `tfsdk:"enabled"`
	Config            types.String  `tfsdk:"config"`
	SubjectMatchers   types.List    `tfsdk:"subject_matchers"`
	OrganizationID    types.String  `tfsdk:"organization_id"`
	IsSystemGenerated types.Bool    `tfsdk:"is_system_generated"`
	ParentPolicyID    types.String  `tfsdk:"parent_policy_id"`
	CurrentSpendUSD   types.Float64 `tfsdk:"current_spend_usd"`
	CreatedAt         types.String  `tfsdk:"created_at"`
	UpdatedAt         types.String  `tfsdk:"updated_at"`
	CreatedBy         types.String  `tfsdk:"created_by"`
}

type gatewayPolicySubjectMatcher struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type gatewayPolicyCreate struct {
	Name            string                        `json:"name"`
	Description     *string                       `json:"description,omitempty"`
	PolicyType      string                        `json:"policy_type"`
	Action          string                        `json:"action"`
	Priority        *int64                        `json:"priority,omitempty"`
	Enabled         *bool                         `json:"enabled,omitempty"`
	Config          map[string]interface{}        `json:"config,omitempty"`
	SubjectMatchers []gatewayPolicySubjectMatcher `json:"subject_matchers,omitempty"`
}

type gatewayPolicyUpdate struct {
	Name            *string                       `json:"name,omitempty"`
	Description     *string                       `json:"description,omitempty"`
	Action          *string                       `json:"action,omitempty"`
	Priority        *int64                        `json:"priority,omitempty"`
	Enabled         *bool                         `json:"enabled,omitempty"`
	Config          map[string]interface{}        `json:"config,omitempty"`
	SubjectMatchers []gatewayPolicySubjectMatcher `json:"subject_matchers,omitempty"`
}

type gatewayPolicyAPI struct {
	ID                string                        `json:"id"`
	Name              string                        `json:"name"`
	Description       string                        `json:"description"`
	PolicyType        string                        `json:"policy_type"`
	Action            string                        `json:"action"`
	Priority          int64                         `json:"priority"`
	Enabled           bool                          `json:"enabled"`
	Config            map[string]interface{}        `json:"config"`
	SubjectMatchers   []gatewayPolicySubjectMatcher `json:"subject_matchers"`
	OrganizationID    string                        `json:"organization_id"`
	IsSystemGenerated bool                          `json:"is_system_generated"`
	ParentPolicyID    string                        `json:"parent_policy_id"`
	CurrentSpendUSD   *float64                      `json:"current_spend_usd"`
	CreatedAt         string                        `json:"created_at"`
	UpdatedAt         string                        `json:"updated_at"`
	CreatedBy         string                        `json:"created_by"`
}

var subjectMatcherObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{"key": types.StringType, "value": types.StringType}}

func (r *GatewayPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gateway_policy"
}

func (r *GatewayPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith LLM Gateway policy. Policies select subjects via `subject_matchers` and apply an `action` (e.g. spend cap, allow/deny) governed by the policy-type-specific `config`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Policy name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-form description.",
			},
			"policy_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Policy kind (e.g. `spend_cap`). Cannot be changed after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"action": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Action applied when the policy matches (e.g. `block`).",
			},
			"priority": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Evaluation priority — lower values are evaluated first.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the policy is active.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JSON-encoded policy-type-specific configuration (e.g. for `spend_cap`: `{\"amount_usd\": 100, \"window\": \"month\"}`).",
			},
			"subject_matchers": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Predicates that select which API calls the policy applies to.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
			"organization_id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"is_system_generated": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True for policies materialized from a default spend cap.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"parent_policy_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Set on materialized children of a default spend cap; references the parent policy.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"current_spend_usd": schema.Float64Attribute{
				Computed:            true,
				MarkdownDescription: "Current spend in the policy's window for `spend_cap` policies. Null otherwise.",
				PlanModifiers:       []planmodifier.Float64{float64planmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
			"created_by": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
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

func (r *GatewayPolicyResource) buildCreate(ctx context.Context, data *GatewayPolicyResourceModel, diags *diag.Diagnostics) gatewayPolicyCreate {
	body := gatewayPolicyCreate{
		Name:       data.Name.ValueString(),
		PolicyType: data.PolicyType.ValueString(),
		Action:     data.Action.ValueString(),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		v := data.Priority.ValueInt64()
		body.Priority = &v
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		v := data.Enabled.ValueBool()
		body.Enabled = &v
	}
	if !data.Config.IsNull() && !data.Config.IsUnknown() && data.Config.ValueString() != "" {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(data.Config.ValueString()), &m); err != nil {
			diags.AddError("Invalid config JSON", err.Error())
			return body
		}
		body.Config = m
	}
	body.SubjectMatchers = readSubjectMatchers(ctx, data.SubjectMatchers, diags)
	return body
}

func readSubjectMatchers(ctx context.Context, list types.List, diags *diag.Diagnostics) []gatewayPolicySubjectMatcher {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var items []struct {
		Key   types.String `tfsdk:"key"`
		Value types.String `tfsdk:"value"`
	}
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	if diags.HasError() {
		return nil
	}
	out := make([]gatewayPolicySubjectMatcher, 0, len(items))
	for _, it := range items {
		out = append(out, gatewayPolicySubjectMatcher{Key: it.Key.ValueString(), Value: it.Value.ValueString()})
	}
	return out
}

func (r *GatewayPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.buildCreate(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var api gatewayPolicyAPI
	if err := r.client.Post(ctx, "/v1/platform/gateway-policies", body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating gateway policy", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created gateway policy", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GatewayPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api gatewayPolicyAPI
	if err := r.client.Get(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString(), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading gateway policy", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GatewayPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := gatewayPolicyUpdate{}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.Action.IsNull() && !data.Action.IsUnknown() {
		v := data.Action.ValueString()
		body.Action = &v
	}
	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		v := data.Priority.ValueInt64()
		body.Priority = &v
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		v := data.Enabled.ValueBool()
		body.Enabled = &v
	}
	if !data.Config.IsNull() && !data.Config.IsUnknown() && data.Config.ValueString() != "" {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(data.Config.ValueString()), &m); err != nil {
			resp.Diagnostics.AddError("Invalid config JSON", err.Error())
			return
		}
		body.Config = m
	}
	body.SubjectMatchers = readSubjectMatchers(ctx, data.SubjectMatchers, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var api gatewayPolicyAPI
	if err := r.client.Patch(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString(), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating gateway policy", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GatewayPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GatewayPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting gateway policy", err.Error())
		return
	}
}

func (r *GatewayPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GatewayPolicyResource) mapResponse(api *gatewayPolicyAPI, data *GatewayPolicyResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.PolicyType = types.StringValue(api.PolicyType)
	data.Action = types.StringValue(api.Action)
	data.Priority = types.Int64Value(api.Priority)
	data.Enabled = types.BoolValue(api.Enabled)
	if api.Description != "" {
		data.Description = types.StringValue(api.Description)
	} else if data.Description.IsUnknown() {
		data.Description = types.StringNull()
	}
	if len(api.Config) > 0 {
		b, _ := json.Marshal(api.Config)
		data.Config = jsonStringValue(b)
	} else {
		data.Config = types.StringNull()
	}
	if len(api.SubjectMatchers) > 0 {
		elems := make([]attr.Value, 0, len(api.SubjectMatchers))
		for _, sm := range api.SubjectMatchers {
			ov, d := types.ObjectValue(subjectMatcherObjectType.AttrTypes, map[string]attr.Value{
				"key":   types.StringValue(sm.Key),
				"value": types.StringValue(sm.Value),
			})
			diags.Append(d...)
			elems = append(elems, ov)
		}
		list, d := types.ListValue(subjectMatcherObjectType, elems)
		diags.Append(d...)
		data.SubjectMatchers = list
	} else {
		data.SubjectMatchers = types.ListNull(subjectMatcherObjectType)
	}
	data.OrganizationID = types.StringValue(api.OrganizationID)
	data.IsSystemGenerated = types.BoolValue(api.IsSystemGenerated)
	if api.ParentPolicyID != "" {
		data.ParentPolicyID = types.StringValue(api.ParentPolicyID)
	} else {
		data.ParentPolicyID = types.StringNull()
	}
	if api.CurrentSpendUSD != nil {
		data.CurrentSpendUSD = types.Float64Value(*api.CurrentSpendUSD)
	} else {
		data.CurrentSpendUSD = types.Float64Null()
	}
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	data.CreatedBy = types.StringValue(api.CreatedBy)
}

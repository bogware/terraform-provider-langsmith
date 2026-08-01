// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &GatewayPoliciesDataSource{}

// NewGatewayPoliciesDataSource returns a data source that lists LLM Gateway
// policies, optionally filtered server-side by policy type or subject matcher.
func NewGatewayPoliciesDataSource() datasource.DataSource {
	return &GatewayPoliciesDataSource{}
}

// GatewayPoliciesDataSource lists LLM Gateway policies via
// GET /v1/platform/gateway-policies, which returns a bare JSON array (no
// pagination envelope) and accepts policy_type / subject_matcher_key /
// subject_matcher_value as query filters.
type GatewayPoliciesDataSource struct {
	client *client.Client
}

// GatewayPoliciesDataSourceModel holds the filter inputs, the workspace
// override, and the resulting policy list.
type GatewayPoliciesDataSourceModel struct {
	WorkspaceID         types.String `tfsdk:"workspace_id"`
	PolicyType          types.String `tfsdk:"policy_type"`
	SubjectMatcherKey   types.String `tfsdk:"subject_matcher_key"`
	SubjectMatcherValue types.String `tfsdk:"subject_matcher_value"`
	GatewayPolicies     types.List   `tfsdk:"gateway_policies"`
}

var gatewayPoliciesObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                  types.StringType,
	"name":                types.StringType,
	"description":         types.StringType,
	"policy_type":         types.StringType,
	"action":              types.StringType,
	"priority":            types.Int64Type,
	"enabled":             types.BoolType,
	"config":              types.StringType,
	"subject_matchers":    types.ListType{ElemType: subjectMatcherObjectType},
	"organization_id":     types.StringType,
	"is_system_generated": types.BoolType,
	"parent_policy_id":    types.StringType,
	"current_spend_usd":   types.Float64Type,
	"created_at":          types.StringType,
	"updated_at":          types.StringType,
	"created_by":          types.StringType,
}}

func (d *GatewayPoliciesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gateway_policies"
}

func (d *GatewayPoliciesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the LLM Gateway policies defined in the current LangSmith organization, " +
			"optionally filtered by policy type or subject matcher. Use it to discover existing policy IDs " +
			"(including system-generated ones materialized from a default spend cap) rather than hard-coding them.\n\n" +
			"**Requires the LLM Gateway feature to be enabled on the organization.** If it is not, the API " +
			"returns `403 LLM Gateway not enabled for this organization` and this data source fails.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"policy_type": schema.StringAttribute{
				MarkdownDescription: "Only return policies of this kind (e.g. `spend_cap`). Applied server-side; omit to return every policy type.",
				Optional:            true,
			},
			"subject_matcher_key": schema.StringAttribute{
				MarkdownDescription: "Only return policies carrying a subject matcher with this key. Applied server-side.",
				Optional:            true,
			},
			"subject_matcher_value": schema.StringAttribute{
				MarkdownDescription: "Only return policies carrying a subject matcher with this value. The API pairs it with `subject_matcher_key`, so set both together.",
				Optional:            true,
			},
			"gateway_policies": schema.ListNestedAttribute{
				MarkdownDescription: "The LLM Gateway policies matching the filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the gateway policy.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the gateway policy.",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "The description of the gateway policy. Null when the policy has none.",
							Computed:            true,
						},
						"policy_type": schema.StringAttribute{
							MarkdownDescription: "The policy kind (e.g. `spend_cap`).",
							Computed:            true,
						},
						"action": schema.StringAttribute{
							MarkdownDescription: "The action applied when the policy matches (e.g. `block`).",
							Computed:            true,
						},
						"priority": schema.Int64Attribute{
							MarkdownDescription: "The evaluation priority — lower values are evaluated first.",
							Computed:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether the policy is active.",
							Computed:            true,
						},
						"config": schema.StringAttribute{
							MarkdownDescription: "JSON-encoded policy-type-specific configuration. Null when the policy has none.",
							Computed:            true,
						},
						"subject_matchers": schema.ListNestedAttribute{
							MarkdownDescription: "The predicates that select which API calls the policy applies to.",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										MarkdownDescription: "The subject attribute matched on.",
										Computed:            true,
									},
									"value": schema.StringAttribute{
										MarkdownDescription: "The value the subject attribute must equal.",
										Computed:            true,
									},
								},
							},
						},
						"organization_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the organization owning the policy.",
							Computed:            true,
						},
						"is_system_generated": schema.BoolAttribute{
							MarkdownDescription: "True for policies materialized from a default spend cap rather than created explicitly.",
							Computed:            true,
						},
						"parent_policy_id": schema.StringAttribute{
							MarkdownDescription: "For a materialized child of a default spend cap, the ID of the parent policy. Null otherwise.",
							Computed:            true,
						},
						"current_spend_usd": schema.Float64Attribute{
							MarkdownDescription: "The spend in the policy's current window for `spend_cap` policies. Null for other policy types, or when the spend lookup failed.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the gateway policy.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the gateway policy.",
							Computed:            true,
						},
						"created_by": schema.StringAttribute{
							MarkdownDescription: "The identifier of the principal that created the policy.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *GatewayPoliciesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *GatewayPoliciesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GatewayPoliciesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	query := url.Values{}
	if v := data.PolicyType; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		query.Set("policy_type", v.ValueString())
	}
	if v := data.SubjectMatcherKey; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		query.Set("subject_matcher_key", v.ValueString())
	}
	if v := data.SubjectMatcherValue; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		query.Set("subject_matcher_value", v.ValueString())
	}

	// The list endpoint returns a bare JSON array, not a paginated envelope.
	var policies []gatewayPolicyAPI
	if err := c.Get(ctx, "/api/v1/platform/gateway-policies", query, &policies); err != nil {
		resp.Diagnostics.AddError(
			"Error listing gateway policies",
			"Listing gateway policies requires the LLM Gateway feature to be enabled on the organization; a 403 response means it is not.\n\n"+err.Error(),
		)
		return
	}

	elems := make([]attr.Value, 0, len(policies))
	for _, p := range policies {
		matchers, diags := gatewayPolicySubjectMatchersValue(p.SubjectMatchers)
		resp.Diagnostics.Append(diags...)

		obj, diags := types.ObjectValue(gatewayPoliciesObjectType.AttrTypes, map[string]attr.Value{
			"id":                  types.StringValue(p.ID),
			"name":                types.StringValue(p.Name),
			"description":         nullIfEmpty(p.Description),
			"policy_type":         types.StringValue(p.PolicyType),
			"action":              types.StringValue(p.Action),
			"priority":            types.Int64Value(p.Priority),
			"enabled":             types.BoolValue(p.Enabled),
			"config":              gatewayPolicyConfigValue(p.Config),
			"subject_matchers":    matchers,
			"organization_id":     types.StringValue(p.OrganizationID),
			"is_system_generated": types.BoolValue(p.IsSystemGenerated),
			"parent_policy_id":    nullIfEmpty(p.ParentPolicyID),
			"current_spend_usd":   gatewayPolicySpendValue(p.CurrentSpendUSD),
			"created_at":          types.StringValue(p.CreatedAt),
			"updated_at":          types.StringValue(p.UpdatedAt),
			"created_by":          types.StringValue(p.CreatedBy),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValue(gatewayPoliciesObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.GatewayPolicies = list

	// Gateway policies are organization-scoped: the records carry an
	// organization_id but no workspace/tenant field, so fall back to the
	// client's workspace.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read gateway policies data source", map[string]interface{}{"count": len(policies)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// gatewayPolicySubjectMatchersValue converts the API's subject_matchers array
// to a Terraform list, mapping an absent array to an empty list rather than
// null so callers can always safely iterate it.
func gatewayPolicySubjectMatchersValue(matchers []gatewayPolicySubjectMatcher) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elems := make([]attr.Value, 0, len(matchers))
	for _, sm := range matchers {
		obj, d := types.ObjectValue(subjectMatcherObjectType.AttrTypes, map[string]attr.Value{
			"key":   types.StringValue(sm.Key),
			"value": types.StringValue(sm.Value),
		})
		diags.Append(d...)
		elems = append(elems, obj)
	}
	list, d := types.ListValue(subjectMatcherObjectType, elems)
	diags.Append(d...)
	return list, diags
}

// gatewayPolicyConfigValue serializes the decoded policy config back to a
// normalized JSON string, returning null when the config is absent.
func gatewayPolicyConfigValue(config map[string]interface{}) types.String {
	if len(config) == 0 {
		return types.StringNull()
	}
	b, err := json.Marshal(config)
	if err != nil {
		return types.StringNull()
	}
	return jsonStringValue(b)
}

// gatewayPolicySpendValue maps the nullable current_spend_usd field, which the
// API sets only for spend_cap policies.
func gatewayPolicySpendValue(spend *float64) types.Float64 {
	if spend == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*spend)
}

// nullIfEmpty maps the API's empty-string spelling of "unset" to a Terraform
// null, so optional string fields read as absent rather than blank.
func nullIfEmpty(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

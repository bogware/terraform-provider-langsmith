// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &AccessPoliciesDataSource{}

// NewAccessPoliciesDataSource returns a data source that lists every ABAC
// access policy in the current organization. Its main use is discovering the
// policy IDs needed to populate `langsmith_role_access_policies`.
func NewAccessPoliciesDataSource() datasource.DataSource {
	return &AccessPoliciesDataSource{}
}

// AccessPoliciesDataSource lists org-wide ABAC access policies via
// GET /v1/platform/orgs/current/access-policies. The endpoint is unpaginated
// and takes no query parameters, so the whole set is returned in one call.
type AccessPoliciesDataSource struct {
	client *client.Client
}

// AccessPoliciesDataSourceModel holds the workspace override input and the
// resulting access policy list.
type AccessPoliciesDataSourceModel struct {
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	AccessPolicies types.List   `tfsdk:"access_policies"`
}

// accessPoliciesListResponse mirrors the ListAccessPoliciesResponse envelope
// returned by GET /v1/platform/orgs/current/access-policies. The items reuse
// the single-policy wire struct from access_policy_resource.go.
type accessPoliciesListResponse struct {
	AccessPolicies []accessPolicyAPIResponse `json:"access_policies"`
}

var accessPoliciesObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":               types.StringType,
	"name":             types.StringType,
	"description":      types.StringType,
	"effect":           types.StringType,
	"condition_groups": types.StringType,
	"role_ids":         types.ListType{ElemType: types.StringType},
	"created_at":       types.StringType,
	"updated_at":       types.StringType,
}}

func (d *AccessPoliciesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_policies"
}

func (d *AccessPoliciesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the ABAC access policies defined in the current LangSmith organization. " +
			"Use it to discover the policy IDs required by `langsmith_role_access_policies.access_policy_ids` " +
			"instead of hard-coding them.\n\n" +
			"**Requires ABAC (attribute-based access control) to be enabled on the organization** " +
			"(an enterprise-tier feature). If it is not, the API returns " +
			"`403 ABAC is not enabled for this organization` and this data source fails.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source. Access policies are organization-scoped, so this only selects which workspace's credentials are used.",
				Optional:            true,
				Computed:            true,
			},
			"access_policies": schema.ListNestedAttribute{
				MarkdownDescription: "The access policies defined in the organization.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the access policy.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the access policy.",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "The description of the access policy. Null when the policy has none.",
							Computed:            true,
						},
						"effect": schema.StringAttribute{
							MarkdownDescription: "The policy effect (`allow` or `deny`).",
							Computed:            true,
						},
						"condition_groups": schema.StringAttribute{
							MarkdownDescription: "JSON-encoded array of condition groups evaluated by the policy. Null when the policy is unconditional.",
							Computed:            true,
						},
						"role_ids": schema.ListAttribute{
							MarkdownDescription: "The IDs of the organization roles this policy is attached to.",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the access policy.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the access policy.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *AccessPoliciesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AccessPoliciesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AccessPoliciesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var result accessPoliciesListResponse
	if err := c.Get(ctx, "/v1/platform/orgs/current/access-policies", nil, &result); err != nil {
		resp.Diagnostics.AddError(
			"Error listing access policies",
			"Listing access policies requires ABAC to be enabled on the organization; a 403 response means it is not.\n\n"+err.Error(),
		)
		return
	}

	elems := make([]attr.Value, 0, len(result.AccessPolicies))
	for _, p := range result.AccessPolicies {
		roleIDs, diags := accessPolicyRoleIDsValue(p.RoleIDs)
		resp.Diagnostics.Append(diags...)

		description := types.StringNull()
		if p.Description != "" {
			description = types.StringValue(p.Description)
		}

		obj, diags := types.ObjectValue(accessPoliciesObjectType.AttrTypes, map[string]attr.Value{
			"id":               types.StringValue(p.ID),
			"name":             types.StringValue(p.Name),
			"description":      description,
			"effect":           types.StringValue(p.Effect),
			"condition_groups": jsonEmptyArrayIsNull(p.ConditionGroups),
			"role_ids":         roleIDs,
			"created_at":       types.StringValue(p.CreatedAt),
			"updated_at":       types.StringValue(p.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValue(accessPoliciesObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.AccessPolicies = list

	// The endpoint returns no workspace field (access policies are org-scoped),
	// so fall back to the client's workspace.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read access policies data source", map[string]interface{}{"count": len(result.AccessPolicies)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// accessPolicyRoleIDsValue converts the API's role_ids array to a Terraform
// list, mapping an absent array to an empty list rather than null so callers
// can always safely iterate it.
func accessPolicyRoleIDsValue(ids []string) (types.List, diag.Diagnostics) {
	elems := make([]attr.Value, 0, len(ids))
	for _, id := range ids {
		elems = append(elems, types.StringValue(id))
	}
	return types.ListValue(types.StringType, elems)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &RoleAccessPoliciesResource{}
	_ resource.ResourceWithImportState = &RoleAccessPoliciesResource{}
)

// NewRoleAccessPoliciesResource constructs a RoleAccessPoliciesResource. It
// manages the set of access policies attached to a single organization role as
// a discrete association resource, separate from langsmith_access_policy.
func NewRoleAccessPoliciesResource() resource.Resource {
	return &RoleAccessPoliciesResource{}
}

// RoleAccessPoliciesResource binds a set of access policies to an organization
// role. The API exposes only an attach (POST) endpoint that replaces the full
// set, so Create, Update, and Delete all post the desired (or empty) list.
//
// There is no role-scoped GET, but Read is still authoritative: the org-wide
// access policy list returns each policy's role_ids, so the set attached to a
// role is recovered by filtering that list. See attachedPolicyIDs.
type RoleAccessPoliciesResource struct {
	client *client.Client
}

// RoleAccessPoliciesResourceModel is the Terraform state for the association.
type RoleAccessPoliciesResourceModel struct {
	ID              types.String `tfsdk:"id"`
	RoleID          types.String `tfsdk:"role_id"`
	AccessPolicyIDs types.Set    `tfsdk:"access_policy_ids"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
}

// roleAccessPoliciesAttachRequest is the wire format for the attach endpoint
// (AttachAccessPoliciesPayload). It always carries the full desired list.
type roleAccessPoliciesAttachRequest struct {
	AccessPolicyIDs []string `json:"access_policy_ids"`
}

// roleAccessPoliciesListResponse is the wire format of the org-wide access
// policy list (ListAccessPoliciesResponse).
type roleAccessPoliciesListResponse struct {
	AccessPolicies []roleAccessPolicyListItem `json:"access_policies"`
}

// roleAccessPolicyListItem is a single entry of the org-wide access policy
// list. Only the fields needed to resolve role membership are decoded;
// role_ids is the reverse index the API gives us in lieu of a role-scoped GET.
type roleAccessPolicyListItem struct {
	ID      string   `json:"id"`
	RoleIDs []string `json:"role_ids"`
}

func (r *RoleAccessPoliciesResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_access_policies"
}

func (r *RoleAccessPoliciesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the set of access policies attached to an organization role. This is an association resource: the entire set is replaced on every apply. The underlying LangSmith endpoint only supports attaching the full list, so applying this resource overwrites whatever access policies are currently bound to the role, and destroying it detaches all of them by posting an empty list. Requires ABAC (attribute-based access control) to be enabled on the organization; otherwise the API returns `403 ABAC is not enabled`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of this association, equal to `role_id`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the organization role the access policies are attached to. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"access_policy_ids": schema.SetAttribute{
				MarkdownDescription: "The complete set of access policy IDs to attach to the role. This set is authoritative: any access policy not listed here is detached on apply.",
				Required:            true,
				ElementType:         types.StringType,
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

func (r *RoleAccessPoliciesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// attachPath builds the role-scoped attach endpoint for the given role ID.
func (r *RoleAccessPoliciesResource) attachPath(roleID string) string {
	return "/api/v1/platform/orgs/current/access-policies/roles/" + roleID + "/access-policies"
}

// roleAccessPoliciesListPath is the org-wide access policy list endpoint.
const roleAccessPoliciesListPath = "/api/v1/platform/orgs/current/access-policies"

// attachedPolicyIDs returns the IDs of every access policy in the organization
// currently attached to roleID. The API has no role-scoped GET, so we read the
// org-wide list and filter on each policy's role_ids. The result is sorted so
// the value is stable across refreshes.
func (r *RoleAccessPoliciesResource) attachedPolicyIDs(ctx context.Context, c *client.Client, roleID string) ([]string, error) {
	var result roleAccessPoliciesListResponse
	if err := c.Get(ctx, roleAccessPoliciesListPath, nil, &result); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(result.AccessPolicies))
	for _, policy := range result.AccessPolicies {
		for _, rid := range policy.RoleIDs {
			if rid == roleID {
				ids = append(ids, policy.ID)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// attach posts the supplied access policy IDs (or an empty slice) to the role,
// replacing the role's current access policy set. The endpoint returns 204 with
// no body.
func (r *RoleAccessPoliciesResource) attach(ctx context.Context, c *client.Client, roleID string, ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	body := roleAccessPoliciesAttachRequest{AccessPolicyIDs: ids}
	return c.Post(ctx, r.attachPath(roleID), body, nil)
}

func (r *RoleAccessPoliciesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleAccessPoliciesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ids, err := readStringSet(ctx, data.AccessPolicyIDs)
	if err != nil {
		resp.Diagnostics.AddError("Error reading access_policy_ids", err.Error())
		return
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)
	if err := r.attach(ctx, effClient, data.RoleID.ValueString(), ids); err != nil {
		resp.Diagnostics.AddError("Error attaching access policies to role", err.Error())
		return
	}

	data.ID = data.RoleID
	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)
	tflog.Trace(ctx, "attached access policies to role", map[string]interface{}{"role_id": data.RoleID.ValueString(), "count": len(ids)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read repopulates access_policy_ids from the API by filtering the org-wide
// access policy list on role membership, so external drift (a policy attached
// or detached outside Terraform) is detected, and importing by role ID yields a
// complete, accurate state rather than a null set.
//
// An empty result is a legitimate state, not a deleted resource: `[]` is a
// valid configuration for access_policy_ids, and posting an empty list is
// exactly what Delete does. We therefore record the empty set rather than
// removing the resource, which would otherwise produce a permanent create-diff
// for anyone who legitimately configures an empty set.
func (r *RoleAccessPoliciesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleAccessPoliciesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)

	ids, err := r.attachedPolicyIDs(ctx, effClient, data.RoleID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading access policies for role", err.Error())
		return
	}

	policyIDs, diags := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.AccessPolicyIDs = policyIDs

	if data.ID.IsNull() || data.ID.IsUnknown() {
		data.ID = data.RoleID
	}
	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)
	tflog.Trace(ctx, "read access policies attached to role", map[string]interface{}{"role_id": data.RoleID.ValueString(), "count": len(ids)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleAccessPoliciesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleAccessPoliciesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ids, err := readStringSet(ctx, data.AccessPolicyIDs)
	if err != nil {
		resp.Diagnostics.AddError("Error reading access_policy_ids", err.Error())
		return
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)
	if err := r.attach(ctx, effClient, data.RoleID.ValueString(), ids); err != nil {
		resp.Diagnostics.AddError("Error updating access policies on role", err.Error())
		return
	}

	data.ID = data.RoleID
	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete detaches every access policy from the role by posting an empty list.
func (r *RoleAccessPoliciesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleAccessPoliciesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)
	if err := r.attach(ctx, effClient, data.RoleID.ValueString(), []string{}); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error detaching access policies from role", err.Error())
		return
	}
	tflog.Trace(ctx, "detached access policies from role", map[string]interface{}{"role_id": data.RoleID.ValueString()})
}

func (r *RoleAccessPoliciesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_id"), req.ID)...)
}

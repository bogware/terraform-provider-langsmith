// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

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
	return "/v1/platform/orgs/current/access-policies/roles/" + roleID + "/access-policies"
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

// Read is best-effort. The LangSmith API exposes no endpoint to list the access
// policies currently attached to a role, so we cannot detect external drift.
// Prior state is preserved unchanged.
func (r *RoleAccessPoliciesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleAccessPoliciesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.IsNull() || data.ID.IsUnknown() {
		data.ID = data.RoleID
	}
	finalizeWorkspaceID(&data.WorkspaceID, effectiveClient(r.client, data.WorkspaceID), "", &resp.Diagnostics)
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

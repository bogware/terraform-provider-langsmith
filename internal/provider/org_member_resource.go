// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &OrgMemberResource{}
	_ resource.ResourceWithImportState = &OrgMemberResource{}
)

func NewOrgMemberResource() resource.Resource {
	return &OrgMemberResource{}
}

type OrgMemberResource struct {
	client *client.Client
}

type OrgMemberResourceModel struct {
	ID             types.String `tfsdk:"id"`
	UserID         types.String `tfsdk:"user_id"`
	Email          types.String `tfsdk:"email"`
	RoleID         types.String `tfsdk:"role_id"`
	FullName       types.String `tfsdk:"full_name"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	Pending        types.Bool   `tfsdk:"pending"`
}

type orgMemberCreateRequest struct {
	Email    string  `json:"email"`
	RoleID   *string `json:"role_id,omitempty"`
	FullName *string `json:"full_name,omitempty"`
}

type orgMemberUpdateRequest struct {
	RoleID *string `json:"role_id,omitempty"`
}

type orgMemberCreateResponse struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	RoleID    *string `json:"role_id"`
	FullName  *string `json:"full_name"`
	CreatedAt string  `json:"created_at"`
}

type orgMembersListResponse struct {
	OrganizationID string              `json:"organization_id"`
	Members        []orgMemberIdentity `json:"members"`
	Pending        []orgMemberPending  `json:"pending"`
}

type orgMemberIdentity struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Email          *string `json:"email"`
	FullName       *string `json:"full_name"`
	RoleID         *string `json:"role_id"`
	CreatedAt      string  `json:"created_at"`
	UserID         string  `json:"user_id"`
}

type orgMemberPending struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	RoleID    *string `json:"role_id"`
	FullName  *string `json:"full_name"`
	CreatedAt string  `json:"created_at"`
	// A pending (unaccepted) invitation usually has no backing user yet, so the
	// API commonly omits user_id here. Decode it anyway: if the invite is for an
	// existing LangSmith user the ID is present, and it is strictly better to
	// surface it than to force a second apply.
	UserID string `json:"user_id"`
}

// orgMemberNullableString converts an API string into a Terraform value, mapping
// the empty string to null. The org members endpoint returns "" (not a missing
// key) for a user_id it cannot resolve, and "" would be a lie: it is not a
// usable langsmith_workspace_member.user_id.
func orgMemberNullableString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (r *OrgMemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_member"
}

func (r *OrgMemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith organization member invitation. Creating this resource invites a user by email.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the member/invitation.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID backing this membership. This is the value to feed into " +
					"`langsmith_workspace_member.user_id` to grant the member access to a workspace.\n\n" +
					"This is `null` while the invitation is still pending: an invited user who has not yet " +
					"accepted may not have a resolvable user ID. It is populated on the next refresh once the " +
					"invitation is accepted, so a `langsmith_workspace_member` chained off a brand-new invite " +
					"may require a second apply.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the member to invite.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The role ID to assign to the member.",
				Optional:            true,
			},
			"full_name": schema.StringAttribute{
				MarkdownDescription: "The full name of the member.",
				Optional:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pending": schema.BoolAttribute{
				MarkdownDescription: "Whether the invitation is still unaccepted. A pending member is addressed by different API endpoints than an accepted one, so the provider tracks which applies.",
				Computed:            true,
			},
		},
	}
}

func (r *OrgMemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrgMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrgMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := orgMemberCreateRequest{
		Email: data.Email.ValueString(),
	}
	setOptionalString(&body.RoleID, data.RoleID)
	setOptionalString(&body.FullName, data.FullName)

	var result orgMemberCreateResponse
	err := r.client.Post(ctx, "/api/v1/orgs/current/members", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error inviting org member", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Email = types.StringValue(result.Email)
	data.CreatedAt = types.StringValue(result.CreatedAt)

	if result.RoleID != nil {
		data.RoleID = types.StringValue(*result.RoleID)
	}
	if result.FullName != nil {
		data.FullName = types.StringValue(*result.FullName)
	}

	// Read back to get organization_id and user_id.
	found := r.refreshMemberData(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		// Persist partial state so the created invitation is tracked (and
		// tainted) instead of orphaned when the read-back fails. The read-back
		// is what populates these, so null out anything still unknown --
		// Terraform hard-fails on an unknown value after apply.
		if data.OrganizationID.IsUnknown() {
			data.OrganizationID = types.StringNull()
		}
		if data.UserID.IsUnknown() {
			data.UserID = types.StringNull()
		}
		if data.Pending.IsUnknown() {
			data.Pending = types.BoolNull()
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	if !found {
		resp.Diagnostics.AddError("Error reading org member", "Member not found after creation.")
		// Persist partial state so the created invitation is tracked (and
		// tainted) instead of orphaned.
		if data.UserID.IsUnknown() {
			data.UserID = types.StringNull()
		}
		if data.Pending.IsUnknown() {
			data.Pending = types.BoolNull()
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	tflog.Trace(ctx, "created org member resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrgMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found := r.refreshMemberData(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// refreshMemberData fetches the member list and populates the model. Returns false if not found.
func (r *OrgMemberResource) refreshMemberData(ctx context.Context, data *OrgMemberResourceModel, diags *diag.Diagnostics) bool {
	var listResult orgMembersListResponse
	err := r.client.Get(ctx, "/api/v1/orgs/current/members", nil, &listResult)
	if err != nil {
		diags.AddError("Error reading org members", err.Error())
		return false
	}

	data.OrganizationID = types.StringValue(listResult.OrganizationID)

	// Default user_id to null. A pending invite may have no resolvable user yet,
	// and the framework hard-fails on an attribute left unknown after apply.
	data.UserID = types.StringNull()

	// Search active members first.
	for _, m := range listResult.Members {
		if m.ID == data.ID.ValueString() {
			data.Pending = types.BoolValue(false)
			data.UserID = orgMemberNullableString(m.UserID)
			if m.Email != nil {
				data.Email = types.StringValue(*m.Email)
			}
			if m.RoleID != nil {
				data.RoleID = types.StringValue(*m.RoleID)
			} else {
				data.RoleID = types.StringNull()
			}
			if m.FullName != nil {
				data.FullName = types.StringValue(*m.FullName)
			} else {
				data.FullName = types.StringNull()
			}
			data.CreatedAt = types.StringValue(m.CreatedAt)
			return true
		}
	}

	// Search pending members. An unaccepted invite typically has no user_id;
	// orgMemberNullableString keeps it null rather than "" in that case.
	for _, p := range listResult.Pending {
		if p.ID == data.ID.ValueString() {
			data.Pending = types.BoolValue(true)
			data.UserID = orgMemberNullableString(p.UserID)
			data.Email = types.StringValue(p.Email)
			if p.RoleID != nil {
				data.RoleID = types.StringValue(*p.RoleID)
			} else {
				data.RoleID = types.StringNull()
			}
			if p.FullName != nil {
				data.FullName = types.StringValue(*p.FullName)
			} else {
				data.FullName = types.StringNull()
			}
			data.CreatedAt = types.StringValue(p.CreatedAt)
			return true
		}
	}

	return false
}

func (r *OrgMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrgMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state OrgMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := orgMemberUpdateRequest{}
	setOptionalString(&body.RoleID, data.RoleID)

	err := patchMember(ctx, r.client, "/api/v1/orgs/current/members", data.ID.ValueString(), state.Pending.ValueBool(), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating org member", err.Error())
		return
	}

	// Re-read to get updated state.
	found := r.refreshMemberData(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.Diagnostics.AddError("Error reading org member", "Member not found after update.")
		return
	}

	tflog.Trace(ctx, "updated org member resource", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrgMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := deleteMember(ctx, r.client, "/api/v1/orgs/current/members", data.ID.ValueString(), data.Pending.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Error deleting org member", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted org member resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *OrgMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

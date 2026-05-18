// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                = &RepoOwnerResource{}
	_ resource.ResourceWithImportState = &RepoOwnerResource{}
)

func NewRepoOwnerResource() resource.Resource {
	return &RepoOwnerResource{}
}

type RepoOwnerResource struct {
	client *client.Client
}

type RepoOwnerResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Owner      types.String `tfsdk:"owner"`
	Repo       types.String `tfsdk:"repo"`
	Email      types.String `tfsdk:"email"`
	IdentityID types.String `tfsdk:"identity_id"`
	LSUserID   types.String `tfsdk:"ls_user_id"`
	FullName   types.String `tfsdk:"full_name"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

type repoOwnerAPI struct {
	IdentityID *string `json:"identity_id"`
	LSUserID   string  `json:"ls_user_id"`
	Email      *string `json:"email"`
	FullName   *string `json:"full_name"`
	CreatedAt  string  `json:"created_at"`
}

type addRepoOwnerRequest struct {
	Email string `json:"email"`
}

type removeRepoOwnerRequest struct {
	IdentityID string `json:"identity_id"`
}

type listRepoOwnersResponse struct {
	Owners []repoOwnerAPI `json:"owners"`
}

func (r *RepoOwnerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repo_owner"
}

func (r *RepoOwnerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Adds a collaborator (\"owner\") to a LangSmith prompt repo by email. The user must already exist in the workspace; LangSmith looks them up by `email` and binds the resulting `identity_id` for management.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"owner": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Repo owner handle (e.g. workspace handle).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"repo": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Repo name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Email address of the user to add as an owner.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity UUID resolved from `email`. Used for the remove call.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ls_user_id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"full_name":  schema.StringAttribute{Computed: true},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *RepoOwnerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RepoOwnerResource) repoPath(owner, repo string) string {
	return "/api/v1/repos/" + owner + "/" + repo + "/owners"
}

func (r *RepoOwnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RepoOwnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api repoOwnerAPI
	if err := r.client.Post(ctx, r.repoPath(data.Owner.ValueString(), data.Repo.ValueString()),
		addRepoOwnerRequest{Email: data.Email.ValueString()}, &api); err != nil {
		resp.Diagnostics.AddError("Error adding repo owner", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	tflog.Trace(ctx, "added repo owner", map[string]interface{}{"email": data.Email.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RepoOwnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RepoOwnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list listRepoOwnersResponse
	if err := r.client.Get(ctx, r.repoPath(data.Owner.ValueString(), data.Repo.ValueString()), nil, &list); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading repo owners", err.Error())
		return
	}
	wanted := data.IdentityID.ValueString()
	for _, o := range list.Owners {
		if o.IdentityID != nil && *o.IdentityID == wanted {
			r.mapResponse(&o, &data)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
		if o.LSUserID == data.LSUserID.ValueString() && data.LSUserID.ValueString() != "" {
			r.mapResponse(&o, &data)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *RepoOwnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All inputs are RequiresReplace; no Update.
}

func (r *RepoOwnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RepoOwnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.IdentityID.IsNull() || data.IdentityID.ValueString() == "" {
		// No identity_id stored — likely a server that withheld it for PII reasons.
		resp.Diagnostics.AddWarning("Cannot remove repo owner",
			"identity_id is not set in state, so the owner cannot be removed via the API. Remove the resource from state only.")
		return
	}
	if err := r.client.DeleteWithBody(ctx, r.repoPath(data.Owner.ValueString(), data.Repo.ValueString()),
		removeRepoOwnerRequest{IdentityID: data.IdentityID.ValueString()}); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing repo owner", err.Error())
		return
	}
}

func (r *RepoOwnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<owner>:<repo>:<identity_id>\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("owner"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("identity_id"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func (r *RepoOwnerResource) mapResponse(api *repoOwnerAPI, data *RepoOwnerResourceModel) {
	if api.IdentityID != nil {
		data.IdentityID = types.StringValue(*api.IdentityID)
	} else {
		data.IdentityID = types.StringNull()
	}
	data.LSUserID = types.StringValue(api.LSUserID)
	if api.Email != nil {
		data.Email = types.StringValue(*api.Email)
	}
	if api.FullName != nil {
		data.FullName = types.StringValue(*api.FullName)
	} else {
		data.FullName = types.StringNull()
	}
	data.CreatedAt = types.StringValue(api.CreatedAt)
	if data.IdentityID.IsNull() {
		data.ID = types.StringValue(api.LSUserID)
	} else {
		data.ID = data.IdentityID
	}
}

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
	_ resource.Resource                = &WorkspaceMemberResource{}
	_ resource.ResourceWithImportState = &WorkspaceMemberResource{}
)

// NewWorkspaceMemberResource returns a new WorkspaceMemberResource -- ready to
// deputize a new hand for the workspace crew.
func NewWorkspaceMemberResource() resource.Resource {
	return &WorkspaceMemberResource{}
}

// WorkspaceMemberResource manages workspace members in LangSmith. Every outfit
// needs good hands, and this resource handles who's on the roster and what
// badge they wear.
type WorkspaceMemberResource struct {
	client *client.Client
}

// WorkspaceMemberResourceModel describes the Terraform state for a workspace member.
type WorkspaceMemberResourceModel struct {
	ID        types.String `tfsdk:"id"`
	UserID    types.String `tfsdk:"user_id"`
	RoleID    types.String `tfsdk:"role_id"`
	Email     types.String `tfsdk:"email"`
	FullName  types.String `tfsdk:"full_name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

// workspaceMemberCreateRequest is the summons to bring a new member into the
// workspace fold. The API needs a user_id -- you can't just holler a name
// across the prairie and expect somebody to show up.
type workspaceMemberCreateRequest struct {
	UserID string `json:"user_id"`
	RoleID string `json:"role_id"`
}

// workspaceMemberUpdateRequest adjusts a member's standing -- maybe they
// earned a promotion since the last cattle drive.
type workspaceMemberUpdateRequest struct {
	RoleID string `json:"role_id"`
}

// workspaceMemberCreateResponse is what the API sends back after a new member
// signs the register -- the id is the brand we track 'em by.
type workspaceMemberCreateResponse struct {
	ID string `json:"id"`
}

// workspaceMemberAPIResponse is the full accounting of a workspace member,
// as recorded in the territory's ledger.
type workspaceMemberAPIResponse struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Email     string  `json:"email"`
	FullName  *string `json:"full_name"`
	RoleID    string  `json:"role_id"`
	CreatedAt string  `json:"created_at"`
}

// workspaceMemberListAPIResponse wraps the bunkhouse roster.
type workspaceMemberListAPIResponse struct {
	Members []workspaceMemberAPIResponse `json:"members"`
}

func (r *WorkspaceMemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_member"
}

func (r *WorkspaceMemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith workspace member.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the workspace member (identity_id).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID of the member to add to the workspace.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The role ID to assign to the member.",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the member.",
				Computed:            true,
			},
			"full_name": schema.StringAttribute{
				MarkdownDescription: "The member's full name.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the member was added.",
				Computed:            true,
			},
		},
	}
}

func (r *WorkspaceMemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WorkspaceMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := workspaceMemberCreateRequest{
		UserID: data.UserID.ValueString(),
		RoleID: data.RoleID.ValueString(),
	}

	var createResult workspaceMemberCreateResponse
	err := r.client.Post(ctx, "/api/v1/workspaces/current/members", body, &createResult)
	if err != nil {
		resp.Diagnostics.AddError("Error creating workspace member", err.Error())
		return
	}

	// The create response hands us the member id -- our brand for tracking
	// this cowhand. Now we ride back to the roster for the full picture.
	data.ID = types.StringValue(createResult.ID)

	var listResult workspaceMemberListAPIResponse
	err = r.client.Get(ctx, "/api/v1/workspaces/current/members", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading workspace member after create", err.Error())
		return
	}

	var found *workspaceMemberAPIResponse
	for i := range listResult.Members {
		if listResult.Members[i].ID == createResult.ID {
			found = &listResult.Members[i]
			break
		}
	}

	if found == nil {
		resp.Diagnostics.AddError(
			"Error reading workspace member after create",
			fmt.Sprintf("Member with id %s not found in workspace roster after creation -- vanished like a ghost rider.", createResult.ID),
		)
		return
	}

	mapWorkspaceMemberResponseToState(&data, found)
	tflog.Trace(ctx, "created workspace member resource", map[string]interface{}{"id": createResult.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No single-member endpoint -- we have to call roll on the whole bunkhouse.
	var listResult workspaceMemberListAPIResponse
	err := r.client.Get(ctx, "/api/v1/workspaces/current/members", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading workspace members", err.Error())
		return
	}

	var found *workspaceMemberAPIResponse
	for i := range listResult.Members {
		if listResult.Members[i].ID == data.ID.ValueString() {
			found = &listResult.Members[i]
			break
		}
	}

	if found == nil {
		// This cowhand has ridden off into the sunset.
		resp.State.RemoveResource(ctx)
		return
	}

	mapWorkspaceMemberResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkspaceMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := workspaceMemberUpdateRequest{
		RoleID: data.RoleID.ValueString(),
	}

	var result workspaceMemberAPIResponse
	err := r.client.Patch(ctx, "/api/v1/workspaces/current/members/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating workspace member", err.Error())
		return
	}

	mapWorkspaceMemberResponseToState(&data, &result)
	tflog.Trace(ctx, "updated workspace member resource", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/workspaces/current/members/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting workspace member", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted workspace member resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *WorkspaceMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapWorkspaceMemberResponseToState maps the API response onto Terraform state.
// A good deputy keeps accurate records -- Kitty Russell would expect nothing less
// from anyone working Front Street.
func mapWorkspaceMemberResponseToState(data *WorkspaceMemberResourceModel, result *workspaceMemberAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.UserID = types.StringValue(result.UserID)
	data.RoleID = types.StringValue(result.RoleID)

	if result.Email != "" {
		data.Email = types.StringValue(result.Email)
	} else {
		data.Email = types.StringNull()
	}

	if result.FullName != nil {
		data.FullName = types.StringValue(*result.FullName)
	} else {
		data.FullName = types.StringNull()
	}

	if result.CreatedAt != "" {
		data.CreatedAt = types.StringValue(result.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
}

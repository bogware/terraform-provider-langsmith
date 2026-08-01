// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &PersonalAccessTokenResource{}
	_ resource.ResourceWithImportState = &PersonalAccessTokenResource{}
)

func NewPersonalAccessTokenResource() resource.Resource {
	return &PersonalAccessTokenResource{}
}

type PersonalAccessTokenResource struct {
	client *client.Client
}

type PersonalAccessTokenResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Description        types.String `tfsdk:"description"`
	ShortKey           types.String `tfsdk:"short_key"`
	Key                types.String `tfsdk:"key"`
	CreatedAt          types.String `tfsdk:"created_at"`
	ExpiresAt          types.String `tfsdk:"expires_at"`
	DefaultWorkspaceID types.String `tfsdk:"default_workspace_id"`
	OrgRoleID          types.String `tfsdk:"org_role_id"`
	ReadOnly           types.Bool   `tfsdk:"read_only"`
	RoleID             types.String `tfsdk:"role_id"`
	Workspaces         types.List   `tfsdk:"workspaces"`
}

type patCreateRequest struct {
	Description        string  `json:"description"`
	ExpiresAt          *string `json:"expires_at,omitempty"`
	DefaultWorkspaceID *string `json:"default_workspace_id,omitempty"`
	OrgRoleID          *string `json:"org_role_id,omitempty"`
	// ReadOnly is always sent: it decides whether the token can write, so it must
	// not be left to a server-side default that could change.
	ReadOnly   bool     `json:"read_only"`
	RoleID     *string  `json:"role_id,omitempty"`
	Workspaces []string `json:"workspaces,omitempty"`
}

type patCreateResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ShortKey    string `json:"short_key"`
	Key         string `json:"key"`
	CreatedAt   string `json:"created_at"`
}

type patListItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ShortKey    string `json:"short_key"`
	CreatedAt   string `json:"created_at"`
}

func (r *PersonalAccessTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_personal_access_token"
}

func (r *PersonalAccessTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an organization-scoped personal access token (PAT). PATs cannot be updated — changing any mutable attribute forces recreation. The full token is only returned at creation; after that, only the short key is observable.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("Default API key"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"short_key": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The full PAT. Only available at creation time; empty after import.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ISO 8601 timestamp when the PAT expires.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"default_workspace_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UUID of the default workspace for this PAT.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"org_role_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UUID of the org role to assign. Defaults server-side to `ORG_USER` when omitted.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"read_only": schema.BoolAttribute{
				// Optional-only on purpose -- see the note on langsmith_api_key's
				// read_only: a default plus RequiresReplace would reissue every
				// token that predates this attribute.
				Optional:            true,
				MarkdownDescription: "Whether the token is read-only. Defaults to `false` server-side when omitted. The API does not return this value, so Terraform cannot refresh it and leaves it null unless you set it explicitly; changing it forces a new token.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"role_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UUID of the workspace role to assign to the token. If omitted, the server picks a default role.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"workspaces": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of workspace UUIDs this token may access.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *PersonalAccessTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PersonalAccessTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PersonalAccessTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := patCreateRequest{Description: data.Description.ValueString()}
	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		v := data.ExpiresAt.ValueString()
		body.ExpiresAt = &v
	}
	if !data.DefaultWorkspaceID.IsNull() && !data.DefaultWorkspaceID.IsUnknown() {
		v := data.DefaultWorkspaceID.ValueString()
		body.DefaultWorkspaceID = &v
	}
	if !data.OrgRoleID.IsNull() && !data.OrgRoleID.IsUnknown() {
		v := data.OrgRoleID.ValueString()
		body.OrgRoleID = &v
	}
	if !data.RoleID.IsNull() && !data.RoleID.IsUnknown() {
		v := data.RoleID.ValueString()
		body.RoleID = &v
	}
	if !data.Workspaces.IsNull() && !data.Workspaces.IsUnknown() {
		var ws []string
		resp.Diagnostics.Append(data.Workspaces.ElementsAs(ctx, &ws, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Workspaces = ws
	}
	body.ReadOnly = data.ReadOnly.ValueBool()

	var result patCreateResponse
	if err := r.client.Post(ctx, "/api/v1/orgs/current/personal-access-tokens", body, &result); err != nil {
		resp.Diagnostics.AddError("Error creating personal access token", err.Error())
		return
	}
	data.ID = types.StringValue(result.ID)
	data.Description = types.StringValue(result.Description)
	data.ShortKey = types.StringValue(result.ShortKey)
	data.Key = types.StringValue(result.Key)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	tflog.Trace(ctx, "created personal access token", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PersonalAccessTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PersonalAccessTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list []patListItem
	if err := r.client.Get(ctx, "/api/v1/orgs/current/personal-access-tokens", nil, &list); err != nil {
		resp.Diagnostics.AddError("Error reading personal access tokens", err.Error())
		return
	}
	var found *patListItem
	for i := range list {
		if list[i].ID == data.ID.ValueString() {
			found = &list[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ID = types.StringValue(found.ID)
	data.Description = types.StringValue(found.Description)
	data.ShortKey = types.StringValue(found.ShortKey)
	data.CreatedAt = types.StringValue(found.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PersonalAccessTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported", "Personal access tokens cannot be updated; all mutable attributes are marked RequiresReplace.")
}

func (r *PersonalAccessTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PersonalAccessTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/api/v1/orgs/current/personal-access-tokens/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting personal access token", err.Error())
		return
	}
}

func (r *PersonalAccessTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

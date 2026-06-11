// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &WorkspaceMembersDataSource{}

// NewWorkspaceMembersDataSource returns a data source that lists every member
// of the current workspace -- a full roll call of the bunkhouse.
func NewWorkspaceMembersDataSource() datasource.DataSource {
	return &WorkspaceMembersDataSource{}
}

type WorkspaceMembersDataSource struct {
	client *client.Client
}

type WorkspaceMembersDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Members     types.List   `tfsdk:"members"`
	Pending     types.List   `tfsdk:"pending"`
}

// workspaceMembersAPIResponse mirrors the TenantMembers schema returned by
// GET /api/v1/workspaces/current/members.
type workspaceMembersAPIResponse struct {
	TenantID string                       `json:"tenant_id"`
	Members  []workspaceMemberIdentityAPI `json:"members"`
	Pending  []workspaceMemberPendingAPI  `json:"pending"`
}

// workspaceMemberIdentityAPI mirrors the MemberIdentity schema.
type workspaceMemberIdentityAPI struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	LsUserID    string  `json:"ls_user_id"`
	Email       *string `json:"email"`
	FullName    *string `json:"full_name"`
	RoleID      *string `json:"role_id"`
	RoleName    *string `json:"role_name"`
	AccessScope string  `json:"access_scope"`
	OrgRoleID   *string `json:"org_role_id"`
	OrgRoleName *string `json:"org_role_name"`
	IsDisabled  bool    `json:"is_disabled"`
	CreatedAt   string  `json:"created_at"`
}

// workspaceMemberPendingAPI mirrors the PendingIdentity schema (invited but
// not yet accepted).
type workspaceMemberPendingAPI struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FullName  *string `json:"full_name"`
	RoleID    *string `json:"role_id"`
	RoleName  *string `json:"role_name"`
	CreatedAt string  `json:"created_at"`
}

var workspaceMemberObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":            types.StringType,
	"user_id":       types.StringType,
	"ls_user_id":    types.StringType,
	"email":         types.StringType,
	"full_name":     types.StringType,
	"role_id":       types.StringType,
	"role_name":     types.StringType,
	"access_scope":  types.StringType,
	"org_role_id":   types.StringType,
	"org_role_name": types.StringType,
	"is_disabled":   types.BoolType,
	"created_at":    types.StringType,
}}

var workspaceMemberPendingObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":         types.StringType,
	"email":      types.StringType,
	"full_name":  types.StringType,
	"role_id":    types.StringType,
	"role_name":  types.StringType,
	"created_at": types.StringType,
}}

func (d *WorkspaceMembersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_members"
}

func (d *WorkspaceMembersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all members of the current LangSmith workspace, including pending invitations.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source. Populated from the API response when unset.",
				Optional:            true,
				Computed:            true,
			},
			"members": schema.ListNestedAttribute{
				MarkdownDescription: "The members of the workspace.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace member identity ID."},
						"user_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "The user ID of the member."},
						"ls_user_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "The LangSmith user ID of the member."},
						"email":         schema.StringAttribute{Computed: true, MarkdownDescription: "The member's email address."},
						"full_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "The member's full name."},
						"role_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role ID assigned to the member."},
						"role_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role name assigned to the member."},
						"access_scope":  schema.StringAttribute{Computed: true, MarkdownDescription: "The member's access scope (e.g. `workspace` or `organization`)."},
						"org_role_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "The organization role ID of the member."},
						"org_role_name": schema.StringAttribute{Computed: true, MarkdownDescription: "The organization role name of the member."},
						"is_disabled":   schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the member is disabled."},
						"created_at":    schema.StringAttribute{Computed: true, MarkdownDescription: "When the member was added to the workspace."},
					},
				},
			},
			"pending": schema.ListNestedAttribute{
				MarkdownDescription: "Pending workspace invitations that have not been accepted yet.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true, MarkdownDescription: "The pending invitation ID."},
						"email":      schema.StringAttribute{Computed: true, MarkdownDescription: "The invited email address."},
						"full_name":  schema.StringAttribute{Computed: true, MarkdownDescription: "The invitee's full name."},
						"role_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role ID assigned to the invitee."},
						"role_name":  schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role name assigned to the invitee."},
						"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "When the invitation was created."},
					},
				},
			},
		},
	}
}

func (d *WorkspaceMembersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *WorkspaceMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api workspaceMembersAPIResponse
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/workspaces/current/members", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error listing workspace members", err.Error())
		return
	}

	memberElems := make([]attr.Value, 0, len(api.Members))
	for i := range api.Members {
		m := api.Members[i]
		obj, diags := types.ObjectValue(workspaceMemberObjectType.AttrTypes, map[string]attr.Value{
			"id":            types.StringValue(m.ID),
			"user_id":       types.StringValue(m.UserID),
			"ls_user_id":    types.StringValue(m.LsUserID),
			"email":         types.StringPointerValue(m.Email),
			"full_name":     types.StringPointerValue(m.FullName),
			"role_id":       types.StringPointerValue(m.RoleID),
			"role_name":     types.StringPointerValue(m.RoleName),
			"access_scope":  types.StringValue(m.AccessScope),
			"org_role_id":   types.StringPointerValue(m.OrgRoleID),
			"org_role_name": types.StringPointerValue(m.OrgRoleName),
			"is_disabled":   types.BoolValue(m.IsDisabled),
			"created_at":    types.StringValue(m.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		memberElems = append(memberElems, obj)
	}
	members, diags := types.ListValue(workspaceMemberObjectType, memberElems)
	resp.Diagnostics.Append(diags...)
	data.Members = members

	pendingElems := make([]attr.Value, 0, len(api.Pending))
	for i := range api.Pending {
		p := api.Pending[i]
		obj, objDiags := types.ObjectValue(workspaceMemberPendingObjectType.AttrTypes, map[string]attr.Value{
			"id":         types.StringValue(p.ID),
			"email":      types.StringValue(p.Email),
			"full_name":  types.StringPointerValue(p.FullName),
			"role_id":    types.StringPointerValue(p.RoleID),
			"role_name":  types.StringPointerValue(p.RoleName),
			"created_at": types.StringValue(p.CreatedAt),
		})
		resp.Diagnostics.Append(objDiags...)
		pendingElems = append(pendingElems, obj)
	}
	pending, pendingDiags := types.ListValue(workspaceMemberPendingObjectType, pendingElems)
	resp.Diagnostics.Append(pendingDiags...)
	data.Pending = pending

	reconcileWorkspaceID(&data.WorkspaceID, api.TenantID, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

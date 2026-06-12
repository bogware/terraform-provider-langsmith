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

var _ datasource.DataSource = &OrgMembersDataSource{}

// NewOrgMembersDataSource returns a data source that lists every member of
// the current organization -- the whole territory's census in one ride.
func NewOrgMembersDataSource() datasource.DataSource {
	return &OrgMembersDataSource{}
}

type OrgMembersDataSource struct {
	client *client.Client
}

type OrgMembersDataSourceModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
	Members        types.List   `tfsdk:"members"`
	Pending        types.List   `tfsdk:"pending"`
}

// orgMembersAPIResponse mirrors the OrganizationMembers schema returned by
// GET /api/v1/orgs/current/members.
type orgMembersAPIResponse struct {
	OrganizationID string                 `json:"organization_id"`
	Members        []orgMemberIdentityAPI `json:"members"`
	Pending        []orgMemberPendingAPI  `json:"pending"`
}

// orgMemberIdentityAPI mirrors the OrgMemberIdentity schema.
type orgMemberIdentityAPI struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	LsUserID    string   `json:"ls_user_id"`
	Email       *string  `json:"email"`
	FullName    *string  `json:"full_name"`
	RoleID      *string  `json:"role_id"`
	RoleName    *string  `json:"role_name"`
	AccessScope string   `json:"access_scope"`
	OrgRoleID   *string  `json:"org_role_id"`
	OrgRoleName *string  `json:"org_role_name"`
	IsDisabled  bool     `json:"is_disabled"`
	TenantIDs   []string `json:"tenant_ids"`
	CreatedAt   string   `json:"created_at"`
}

// orgMemberPendingAPI mirrors the OrgPendingIdentity schema (invited but not
// yet accepted).
type orgMemberPendingAPI struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	FullName    *string `json:"full_name"`
	RoleID      *string `json:"role_id"`
	RoleName    *string `json:"role_name"`
	OrgRoleID   *string `json:"org_role_id"`
	OrgRoleName *string `json:"org_role_name"`
	CreatedAt   string  `json:"created_at"`
}

var orgMemberObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
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
	"workspace_ids": types.ListType{ElemType: types.StringType},
	"created_at":    types.StringType,
}}

var orgMemberPendingObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":            types.StringType,
	"email":         types.StringType,
	"full_name":     types.StringType,
	"role_id":       types.StringType,
	"role_name":     types.StringType,
	"org_role_id":   types.StringType,
	"org_role_name": types.StringType,
	"created_at":    types.StringType,
}}

func (d *OrgMembersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_members"
}

func (d *OrgMembersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all members of the current LangSmith organization, including pending invitations.",
		Attributes: map[string]schema.Attribute{
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the organization.",
				Computed:            true,
			},
			"members": schema.ListNestedAttribute{
				MarkdownDescription: "The members of the organization.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "The organization member identity ID."},
						"user_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "The user ID of the member."},
						"ls_user_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "The LangSmith user ID of the member."},
						"email":         schema.StringAttribute{Computed: true, MarkdownDescription: "The member's email address."},
						"full_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "The member's full name."},
						"role_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role ID of the member, if any."},
						"role_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role name of the member, if any."},
						"access_scope":  schema.StringAttribute{Computed: true, MarkdownDescription: "The member's access scope (e.g. `workspace` or `organization`)."},
						"org_role_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "The organization role ID of the member."},
						"org_role_name": schema.StringAttribute{Computed: true, MarkdownDescription: "The organization role name of the member."},
						"is_disabled":   schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the member is disabled."},
						"workspace_ids": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "The IDs of the workspaces this member belongs to (returned by the API as `tenant_ids`).",
						},
						"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "When the member joined the organization."},
					},
				},
			},
			"pending": schema.ListNestedAttribute{
				MarkdownDescription: "Pending organization invitations that have not been accepted yet.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "The pending invitation ID."},
						"email":         schema.StringAttribute{Computed: true, MarkdownDescription: "The invited email address."},
						"full_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "The invitee's full name."},
						"role_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role ID assigned to the invitee, if any."},
						"role_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace role name assigned to the invitee, if any."},
						"org_role_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "The organization role ID assigned to the invitee."},
						"org_role_name": schema.StringAttribute{Computed: true, MarkdownDescription: "The organization role name assigned to the invitee."},
						"created_at":    schema.StringAttribute{Computed: true, MarkdownDescription: "When the invitation was created."},
					},
				},
			},
		},
	}
}

func (d *OrgMembersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrgMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrgMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api orgMembersAPIResponse
	if err := d.client.Get(ctx, "/api/v1/orgs/current/members", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error listing organization members", err.Error())
		return
	}

	memberElems := make([]attr.Value, 0, len(api.Members))
	for i := range api.Members {
		m := api.Members[i]
		workspaceIDElems := make([]attr.Value, 0, len(m.TenantIDs))
		for _, t := range m.TenantIDs {
			workspaceIDElems = append(workspaceIDElems, types.StringValue(t))
		}
		workspaceIDs, wsDiags := types.ListValue(types.StringType, workspaceIDElems)
		resp.Diagnostics.Append(wsDiags...)
		obj, diags := types.ObjectValue(orgMemberObjectType.AttrTypes, map[string]attr.Value{
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
			"workspace_ids": workspaceIDs,
			"created_at":    types.StringValue(m.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		memberElems = append(memberElems, obj)
	}
	members, diags := types.ListValue(orgMemberObjectType, memberElems)
	resp.Diagnostics.Append(diags...)
	data.Members = members

	pendingElems := make([]attr.Value, 0, len(api.Pending))
	for i := range api.Pending {
		p := api.Pending[i]
		obj, objDiags := types.ObjectValue(orgMemberPendingObjectType.AttrTypes, map[string]attr.Value{
			"id":            types.StringValue(p.ID),
			"email":         types.StringValue(p.Email),
			"full_name":     types.StringPointerValue(p.FullName),
			"role_id":       types.StringPointerValue(p.RoleID),
			"role_name":     types.StringPointerValue(p.RoleName),
			"org_role_id":   types.StringPointerValue(p.OrgRoleID),
			"org_role_name": types.StringPointerValue(p.OrgRoleName),
			"created_at":    types.StringValue(p.CreatedAt),
		})
		resp.Diagnostics.Append(objDiags...)
		pendingElems = append(pendingElems, obj)
	}
	pending, pendingDiags := types.ListValue(orgMemberPendingObjectType, pendingElems)
	resp.Diagnostics.Append(pendingDiags...)
	data.Pending = pending

	data.OrganizationID = types.StringValue(api.OrganizationID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

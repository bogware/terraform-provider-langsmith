// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &RepoOwnersDataSource{}

func NewRepoOwnersDataSource() datasource.DataSource {
	return &RepoOwnersDataSource{}
}

type RepoOwnersDataSource struct {
	client *client.Client
}

type RepoOwnersDataSourceModel struct {
	ID          types.String          `tfsdk:"id"`
	RepoHandle  types.String          `tfsdk:"repo_handle"`
	Owner       types.String          `tfsdk:"owner"`
	WorkspaceID types.String          `tfsdk:"workspace_id"`
	Owners      []repoOwnersItemModel `tfsdk:"owners"`
}

type repoOwnersItemModel struct {
	IdentityID types.String `tfsdk:"identity_id"`
	LSUserID   types.String `tfsdk:"ls_user_id"`
	Email      types.String `tfsdk:"email"`
	FullName   types.String `tfsdk:"full_name"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

func (d *RepoOwnersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repo_owners"
}

func (d *RepoOwnersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the collaborators (\"owners\") of a LangSmith prompt repo.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic identifier of the form `<owner>/<repo_handle>`.",
				Computed:            true,
			},
			"repo_handle": schema.StringAttribute{
				MarkdownDescription: "The name of the prompt repo whose owners should be listed.",
				Required:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "The owner handle of the repo. Defaults to `-`, the wildcard for the current workspace, matching how `langsmith_prompt` addresses repos.",
				Optional:            true,
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"owners": schema.ListNestedAttribute{
				MarkdownDescription: "The collaborators of the repo.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"identity_id": schema.StringAttribute{
							MarkdownDescription: "Identity UUID of the owner.",
							Computed:            true,
						},
						"ls_user_id": schema.StringAttribute{
							MarkdownDescription: "LangSmith user ID of the owner.",
							Computed:            true,
						},
						"email": schema.StringAttribute{
							MarkdownDescription: "Email address of the owner.",
							Computed:            true,
						},
						"full_name": schema.StringAttribute{
							MarkdownDescription: "Full name of the owner.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "Timestamp at which the owner was added to the repo.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *RepoOwnersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RepoOwnersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RepoOwnersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner := repoOwnerSegment(data.Owner.ValueString())
	repo := data.RepoHandle.ValueString()

	var list listRepoOwnersResponse
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, fmt.Sprintf("/api/v1/repos/%s/%s/owners", owner, repo), nil, &list); err != nil {
		resp.Diagnostics.AddError("Error reading repo owners", err.Error())
		return
	}

	data.Owner = types.StringValue(owner)
	data.ID = types.StringValue(owner + "/" + repo)
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)

	data.Owners = make([]repoOwnersItemModel, 0, len(list.Owners))
	for _, o := range list.Owners {
		item := repoOwnersItemModel{
			LSUserID:  types.StringValue(o.LSUserID),
			CreatedAt: types.StringValue(o.CreatedAt),
		}
		if o.IdentityID != nil {
			item.IdentityID = types.StringValue(*o.IdentityID)
		} else {
			item.IdentityID = types.StringNull()
		}
		if o.Email != nil {
			item.Email = types.StringValue(*o.Email)
		} else {
			item.Email = types.StringNull()
		}
		if o.FullName != nil {
			item.FullName = types.StringValue(*o.FullName)
		} else {
			item.FullName = types.StringNull()
		}
		data.Owners = append(data.Owners, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

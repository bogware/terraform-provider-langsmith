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

var _ datasource.DataSource = &PromptRepoTagsDataSource{}

// NewPromptRepoTagsDataSource returns a data source that lists the named tags
// on a LangSmith prompt repo.
func NewPromptRepoTagsDataSource() datasource.DataSource {
	return &PromptRepoTagsDataSource{}
}

// PromptRepoTagsDataSource lists the version tags for a prompt repo.
type PromptRepoTagsDataSource struct {
	client *client.Client
}

// PromptRepoTagsDataSourceModel maps the Terraform schema for the data source.
type PromptRepoTagsDataSourceModel struct {
	RepoHandle  types.String                   `tfsdk:"repo_handle"`
	WorkspaceID types.String                   `tfsdk:"workspace_id"`
	Tags        []promptRepoTagsDataSourceItem `tfsdk:"tags"`
}

// promptRepoTagsDataSourceItem mirrors a single tag in the computed list.
type promptRepoTagsDataSourceItem struct {
	ID         types.String `tfsdk:"id"`
	RepoID     types.String `tfsdk:"repo_id"`
	CommitID   types.String `tfsdk:"commit_id"`
	CommitHash types.String `tfsdk:"commit_hash"`
	TagName    types.String `tfsdk:"tag_name"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func (d *PromptRepoTagsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt_repo_tags"
}

func (d *PromptRepoTagsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the named version tags on a LangSmith prompt repo. Tags such as `production` or `staging` point to specific commits.",
		Attributes: map[string]schema.Attribute{
			"repo_handle": schema.StringAttribute{
				MarkdownDescription: "The handle of the prompt repo whose tags are listed.",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "The tags defined on the prompt repo.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the tag.",
							Computed:            true,
						},
						"repo_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the prompt repo the tag belongs to.",
							Computed:            true,
						},
						"commit_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the commit the tag points to.",
							Computed:            true,
						},
						"commit_hash": schema.StringAttribute{
							MarkdownDescription: "The hash of the commit the tag points to.",
							Computed:            true,
						},
						"tag_name": schema.StringAttribute{
							MarkdownDescription: "The name of the tag (e.g., `production`, `staging`).",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "When the tag was created.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "When the tag was last updated.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *PromptRepoTagsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *PromptRepoTagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PromptRepoTagsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	effClient := effectiveClient(d.client, data.WorkspaceID)

	// GET /api/v1/repos/{owner}/{repo}/tags returns a bare JSON array of tags.
	var listResp []promptTagAPIResponse
	err := effClient.Get(ctx, fmt.Sprintf("/api/v1/repos/-/%s/tags", data.RepoHandle.ValueString()), nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Error listing prompt repo tags", err.Error())
		return
	}

	data.Tags = make([]promptRepoTagsDataSourceItem, 0, len(listResp))
	for _, t := range listResp {
		data.Tags = append(data.Tags, promptRepoTagsDataSourceItem{
			ID:         types.StringValue(t.ID),
			RepoID:     types.StringValue(t.RepoID),
			CommitID:   types.StringValue(t.CommitID),
			CommitHash: types.StringValue(t.CommitHash),
			TagName:    types.StringValue(t.TagName),
			CreatedAt:  types.StringValue(t.CreatedAt),
			UpdatedAt:  types.StringValue(t.UpdatedAt),
		})
	}

	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

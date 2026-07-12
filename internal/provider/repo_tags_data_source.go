// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

const (
	// repoTagsPageSize is the page size used when listing repo tags. The API
	// caps `limit` at 100.
	repoTagsPageSize = 100

	// repoTagsMaxPages bounds the paging loop so a misbehaving endpoint can
	// never spin forever.
	repoTagsMaxPages = 100
)

var _ datasource.DataSource = &RepoTagsDataSource{}

// NewRepoTagsDataSource returns a data source that lists the prompt-repo tag
// catalog together with the number of repos carrying each tag.
func NewRepoTagsDataSource() datasource.DataSource {
	return &RepoTagsDataSource{}
}

// RepoTagsDataSource reads the prompt-repo tag catalog from
// GET /api/v1/repos/tags, paging until the endpoint returns a short page.
type RepoTagsDataSource struct {
	client *client.Client
}

// RepoTagsDataSourceModel maps the Terraform schema for the data source.
type RepoTagsDataSourceModel struct {
	Query       types.String             `tfsdk:"query"`
	WorkspaceID types.String             `tfsdk:"workspace_id"`
	Tags        []repoTagsDataSourceItem `tfsdk:"tags"`
}

// repoTagsDataSourceItem mirrors a single tag in the computed list.
type repoTagsDataSourceItem struct {
	Tag   types.String `tfsdk:"tag"`
	Count types.Int64  `tfsdk:"count"`
}

// repoTagsListResponse mirrors the ListTagsResponse envelope returned by
// GET /api/v1/repos/tags. Note the tags are wrapped in a `tags` key.
type repoTagsListResponse struct {
	Tags []repoTagCountAPIResponse `json:"tags"`
}

// repoTagCountAPIResponse mirrors the TagCount wire schema.
type repoTagCountAPIResponse struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

func (d *RepoTagsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repo_tags"
}

func (d *RepoTagsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the prompt-repo tag catalog — every tag applied to a prompt repo, with the number of repos carrying it. " +
			"Use this to discover which tags are in circulation before tagging a repo. " +
			"This is distinct from `langsmith_prompt_repo_tags`, which lists the named version tags (such as `production`) on a single prompt repo.",
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				MarkdownDescription: "If set, only returns tags matching this search string. When omitted, the entire tag catalog is returned.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "The prompt-repo tags, each with its usage count.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tag": schema.StringAttribute{
							MarkdownDescription: "The tag name (for example, `ChatPromptTemplate`).",
							Computed:            true,
						},
						"count": schema.Int64Attribute{
							MarkdownDescription: "The number of prompt repos carrying this tag.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *RepoTagsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RepoTagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RepoTagsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	searchQuery := ""
	if !data.Query.IsNull() && !data.Query.IsUnknown() {
		searchQuery = data.Query.ValueString()
	}

	// The endpoint reports neither a total nor a cursor, so page until it hands
	// back a short (or empty) page.
	var all []repoTagCountAPIResponse
	for page := 0; page < repoTagsMaxPages; page++ {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(repoTagsPageSize))
		query.Set("offset", strconv.Itoa(page*repoTagsPageSize))
		if searchQuery != "" {
			query.Set("query", searchQuery)
		}

		var listResp repoTagsListResponse
		if err := c.Get(ctx, "/api/v1/repos/tags", query, &listResp); err != nil {
			resp.Diagnostics.AddError("Error listing repo tags", err.Error())
			return
		}

		all = append(all, listResp.Tags...)
		if len(listResp.Tags) < repoTagsPageSize {
			break
		}
	}

	data.Tags = make([]repoTagsDataSourceItem, 0, len(all))
	for _, t := range all {
		data.Tags = append(data.Tags, repoTagsDataSourceItem{
			Tag:   types.StringValue(t.Tag),
			Count: types.Int64Value(t.Count),
		})
	}

	// The endpoint does not echo a workspace identifier, so fall back to the
	// workspace the client is operating in.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read repo tags data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

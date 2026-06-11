// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &PromptsDataSource{}

func NewPromptsDataSource() datasource.DataSource {
	return &PromptsDataSource{}
}

type PromptsDataSource struct {
	client *client.Client
}

type PromptsDataSourceModel struct {
	Query       types.String `tfsdk:"query"`
	IsPublic    types.Bool   `tfsdk:"is_public"`
	IsArchived  types.String `tfsdk:"is_archived"`
	Tags        types.List   `tfsdk:"tags"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Prompts     types.List   `tfsdk:"prompts"`
}

// promptsListItemAPI mirrors a single repo in GET /api/v1/repos responses
// (component schema RepoWithLookups). The list endpoint reports the workspace
// as tenant_id.
type promptsListItemAPI struct {
	ID          string   `json:"id"`
	RepoHandle  string   `json:"repo_handle"`
	Description *string  `json:"description"`
	Readme      *string  `json:"readme"`
	IsPublic    bool     `json:"is_public"`
	IsArchived  bool     `json:"is_archived"`
	Tags        []string `json:"tags"`
	TenantID    string   `json:"tenant_id"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type promptsListAPIResponse struct {
	Repos []promptsListItemAPI `json:"repos"`
	Total int                  `json:"total"`
}

var promptObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":           types.StringType,
	"repo_handle":  types.StringType,
	"description":  types.StringType,
	"readme":       types.StringType,
	"is_public":    types.BoolType,
	"is_archived":  types.BoolType,
	"tags":         types.ListType{ElemType: types.StringType},
	"workspace_id": types.StringType,
	"created_at":   types.StringType,
	"updated_at":   types.StringType,
}}

func (d *PromptsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompts"
}

func (d *PromptsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith prompts (repos), with optional filters. All pages are fetched.",
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				MarkdownDescription: "Free-text search over prompt repos.",
				Optional:            true,
			},
			"is_public": schema.BoolAttribute{
				MarkdownDescription: "Filter by public visibility.",
				Optional:            true,
			},
			"is_archived": schema.StringAttribute{
				MarkdownDescription: "Filter by archive status. Valid values: `true`, `false`, `allow` (include both).",
				Optional:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Filter to prompts carrying all the given tags.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"prompts": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true},
						"repo_handle":  schema.StringAttribute{Computed: true},
						"description":  schema.StringAttribute{Computed: true},
						"readme":       schema.StringAttribute{Computed: true},
						"is_public":    schema.BoolAttribute{Computed: true},
						"is_archived":  schema.BoolAttribute{Computed: true},
						"tags":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"workspace_id": schema.StringAttribute{Computed: true},
						"created_at":   schema.StringAttribute{Computed: true},
						"updated_at":   schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *PromptsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PromptsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PromptsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseQuery := url.Values{}
	if !data.Query.IsNull() && !data.Query.IsUnknown() {
		baseQuery.Set("query", data.Query.ValueString())
	}
	if !data.IsPublic.IsNull() && !data.IsPublic.IsUnknown() {
		baseQuery.Set("is_public", strconv.FormatBool(data.IsPublic.ValueBool()))
	}
	if !data.IsArchived.IsNull() && !data.IsArchived.IsUnknown() {
		baseQuery.Set("is_archived", data.IsArchived.ValueString())
	}
	if !data.Tags.IsNull() && !data.Tags.IsUnknown() {
		var tags []string
		resp.Diagnostics.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, tag := range tags {
			baseQuery.Add("tags", tag)
		}
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	const pageSize = 100
	var repos []promptsListItemAPI
	for offset := 0; ; offset += pageSize {
		query := url.Values{}
		for k, vs := range baseQuery {
			for _, v := range vs {
				query.Add(k, v)
			}
		}
		query.Set("limit", strconv.Itoa(pageSize))
		query.Set("offset", strconv.Itoa(offset))

		var page promptsListAPIResponse
		if err := c.Get(ctx, "/api/v1/repos", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing prompts", err.Error())
			return
		}
		repos = append(repos, page.Repos...)
		if len(page.Repos) < pageSize || len(repos) >= page.Total {
			break
		}
	}

	elems := make([]attr.Value, 0, len(repos))
	for _, repo := range repos {
		description := types.StringNull()
		if repo.Description != nil {
			description = types.StringValue(*repo.Description)
		}
		readme := types.StringNull()
		if repo.Readme != nil {
			readme = types.StringValue(*repo.Readme)
		}
		tagElems := make([]attr.Value, 0, len(repo.Tags))
		for _, tag := range repo.Tags {
			tagElems = append(tagElems, types.StringValue(tag))
		}
		tagsList, diags := types.ListValue(types.StringType, tagElems)
		resp.Diagnostics.Append(diags...)

		obj, diags := types.ObjectValue(promptObjectType.AttrTypes, map[string]attr.Value{
			"id":           types.StringValue(repo.ID),
			"repo_handle":  types.StringValue(repo.RepoHandle),
			"description":  description,
			"readme":       readme,
			"is_public":    types.BoolValue(repo.IsPublic),
			"is_archived":  types.BoolValue(repo.IsArchived),
			"tags":         tagsList,
			"workspace_id": types.StringValue(repo.TenantID),
			"created_at":   types.StringValue(repo.CreatedAt),
			"updated_at":   types.StringValue(repo.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(promptObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Prompts = list

	tflog.Trace(ctx, "read prompts data source", map[string]interface{}{"count": len(repos)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

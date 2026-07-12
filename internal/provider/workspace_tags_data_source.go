// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &WorkspaceTagsDataSource{}

// NewWorkspaceTagsDataSource returns a data source that lists the tag taxonomy
// (tag keys and their permitted values) defined in a LangSmith workspace.
func NewWorkspaceTagsDataSource() datasource.DataSource {
	return &WorkspaceTagsDataSource{}
}

// WorkspaceTagsDataSource reads the workspace tag taxonomy from
// GET /api/v1/workspaces/current/tags.
type WorkspaceTagsDataSource struct {
	client *client.Client
}

// WorkspaceTagsDataSourceModel maps the Terraform schema for the data source.
type WorkspaceTagsDataSourceModel struct {
	ResourceType types.String            `tfsdk:"resource_type"`
	WorkspaceID  types.String            `tfsdk:"workspace_id"`
	TagKeys      []workspaceTagsKeyModel `tfsdk:"tag_keys"`
}

// workspaceTagsKeyModel mirrors a single tag key, along with the tag values
// registered under it.
type workspaceTagsKeyModel struct {
	ID          types.String              `tfsdk:"id"`
	Key         types.String              `tfsdk:"key"`
	Description types.String              `tfsdk:"description"`
	CreatedAt   types.String              `tfsdk:"created_at"`
	UpdatedAt   types.String              `tfsdk:"updated_at"`
	Values      []workspaceTagsValueModel `tfsdk:"values"`
}

// workspaceTagsValueModel mirrors a single tag value nested under a tag key.
type workspaceTagsValueModel struct {
	ID          types.String `tfsdk:"id"`
	TagKeyID    types.String `tfsdk:"tag_key_id"`
	Value       types.String `tfsdk:"value"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

// workspaceTagKeyAPIResponse mirrors the TagKeyWithValues wire schema.
type workspaceTagKeyAPIResponse struct {
	ID          string                         `json:"id"`
	Key         string                         `json:"key"`
	Description *string                        `json:"description"`
	CreatedAt   string                         `json:"created_at"`
	UpdatedAt   string                         `json:"updated_at"`
	Values      []workspaceTagValueAPIResponse `json:"values"`
}

// workspaceTagValueAPIResponse mirrors the TagValue wire schema.
type workspaceTagValueAPIResponse struct {
	ID          string  `json:"id"`
	TagKeyID    string  `json:"tag_key_id"`
	Value       string  `json:"value"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func (d *WorkspaceTagsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_tags"
}

func (d *WorkspaceTagsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the tag taxonomy of a LangSmith workspace: every tag key and the tag values registered under it. " +
			"This is the read-only counterpart to the `langsmith_tag_key` and `langsmith_tag_value` resources, and is useful for " +
			"discovering a taxonomy that already exists (for example, one managed outside Terraform).",
		Attributes: map[string]schema.Attribute{
			"resource_type": schema.StringAttribute{
				MarkdownDescription: "If set, only returns tag keys applicable to this resource type. One of `agent`, `dashboard`, `dataset`, `deployment`, `evaluator`, `experiment`, `fleet_integration`, `mcp_server`, `project`, `prompt`, `queue`, `sandbox`, or `skill`. When omitted, the entire workspace taxonomy is returned.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tag_keys": schema.ListNestedAttribute{
				MarkdownDescription: "The tag keys defined in the workspace.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the tag key.",
							Computed:            true,
						},
						"key": schema.StringAttribute{
							MarkdownDescription: "The tag key name (for example, `Application`).",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "The description of the tag key.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "When the tag key was created.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "When the tag key was last updated.",
							Computed:            true,
						},
						"values": schema.ListNestedAttribute{
							MarkdownDescription: "The tag values registered under this tag key.",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										MarkdownDescription: "The unique identifier of the tag value.",
										Computed:            true,
									},
									"tag_key_id": schema.StringAttribute{
										MarkdownDescription: "The ID of the tag key this value belongs to.",
										Computed:            true,
									},
									"value": schema.StringAttribute{
										MarkdownDescription: "The tag value (for example, `checkout-service`).",
										Computed:            true,
									},
									"description": schema.StringAttribute{
										MarkdownDescription: "The description of the tag value.",
										Computed:            true,
									},
									"created_at": schema.StringAttribute{
										MarkdownDescription: "When the tag value was created.",
										Computed:            true,
									},
									"updated_at": schema.StringAttribute{
										MarkdownDescription: "When the tag value was last updated.",
										Computed:            true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *WorkspaceTagsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspaceTagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceTagsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var query url.Values
	if !data.ResourceType.IsNull() && !data.ResourceType.IsUnknown() && data.ResourceType.ValueString() != "" {
		query = url.Values{}
		query.Set("resource_type", data.ResourceType.ValueString())
	}

	// GET /api/v1/workspaces/current/tags returns a bare JSON array of tag keys,
	// each carrying its nested tag values.
	var listResp []workspaceTagKeyAPIResponse
	if err := c.Get(ctx, "/api/v1/workspaces/current/tags", query, &listResp); err != nil {
		resp.Diagnostics.AddError("Error listing workspace tags", err.Error())
		return
	}

	data.TagKeys = make([]workspaceTagsKeyModel, 0, len(listResp))
	for _, k := range listResp {
		values := make([]workspaceTagsValueModel, 0, len(k.Values))
		for _, v := range k.Values {
			values = append(values, workspaceTagsValueModel{
				ID:          types.StringValue(v.ID),
				TagKeyID:    types.StringValue(v.TagKeyID),
				Value:       types.StringValue(v.Value),
				Description: types.StringPointerValue(v.Description),
				CreatedAt:   types.StringValue(v.CreatedAt),
				UpdatedAt:   types.StringValue(v.UpdatedAt),
			})
		}
		data.TagKeys = append(data.TagKeys, workspaceTagsKeyModel{
			ID:          types.StringValue(k.ID),
			Key:         types.StringValue(k.Key),
			Description: types.StringPointerValue(k.Description),
			CreatedAt:   types.StringValue(k.CreatedAt),
			UpdatedAt:   types.StringValue(k.UpdatedAt),
			Values:      values,
		})
	}

	// The endpoint does not echo a workspace identifier, so fall back to the
	// workspace the client is operating in.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read workspace tags data source", map[string]interface{}{"tag_key_count": len(data.TagKeys)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

// sandboxRegistriesPageSize is the page size used when listing registries. The
// list endpoint takes limit/offset and echoes the offset back, but carries no
// total or next cursor, so we page until a short page comes home.
const sandboxRegistriesPageSize = 100

// sandboxRegistriesMaxResults bounds the pagination walk. The endpoint reports
// neither a total nor a next cursor, so the only stop signal is a short page --
// which never arrives if the server ignores limit/offset.
const sandboxRegistriesMaxResults = 10000

var _ datasource.DataSource = &SandboxRegistriesDataSource{}

// NewSandboxRegistriesDataSource returns a data source that lists the private
// container image registries configured for LangSmith sandboxes.
func NewSandboxRegistriesDataSource() datasource.DataSource {
	return &SandboxRegistriesDataSource{}
}

// SandboxRegistriesDataSource lists sandbox registries. It pages through
// GET /v2/sandboxes/registries until a page comes back short.
type SandboxRegistriesDataSource struct {
	client *client.Client
}

// SandboxRegistriesDataSourceModel holds the inputs and the resulting registry list.
type SandboxRegistriesDataSourceModel struct {
	NameContains types.String `tfsdk:"name_contains"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Registries   types.List   `tfsdk:"registries"`
}

// sandboxRegistryListResponse mirrors the envelope returned by
// GET /v2/sandboxes/registries -- the results are wrapped in a "registries" key.
type sandboxRegistryListResponse struct {
	Registries []sandboxRegistryAPI `json:"registries"`
	Offset     int64                `json:"offset"`
}

// sandboxRegistriesObjectType intentionally omits username and password: the
// API never returns them, and a data source has no business advertising
// credentials it cannot know.
var sandboxRegistriesObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"url":        types.StringType,
	"created_at": types.StringType,
	"created_by": types.StringType,
	"updated_at": types.StringType,
	"updated_by": types.StringType,
}}

func (d *SandboxRegistriesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox_registries"
}

func (d *SandboxRegistriesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the private container image registries configured for LangSmith sandboxes.\n\n" +
			"The registry `username` and `password` are write-only and are never returned by the API, so they are not " +
			"exposed by this data source.",
		Attributes: map[string]schema.Attribute{
			"name_contains": schema.StringAttribute{
				MarkdownDescription: "If set, only registries whose name contains this substring are returned.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list registries from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"registries": schema.ListNestedAttribute{
				MarkdownDescription: "The sandbox registries configured in the workspace.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the registry.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the registry. Registries are addressed by name.",
							Computed:            true,
						},
						"url": schema.StringAttribute{
							MarkdownDescription: "The URL of the container image registry.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the registry.",
							Computed:            true,
						},
						"created_by": schema.StringAttribute{
							MarkdownDescription: "The identifier of the user who created the registry.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the registry.",
							Computed:            true,
						},
						"updated_by": schema.StringAttribute{
							MarkdownDescription: "The identifier of the user who last modified the registry.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *SandboxRegistriesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *SandboxRegistriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SandboxRegistriesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var all []sandboxRegistryAPI
	offset := 0
	for {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(sandboxRegistriesPageSize))
		query.Set("offset", strconv.Itoa(offset))
		if v := data.NameContains.ValueString(); v != "" {
			query.Set("name_contains", v)
		}

		var page sandboxRegistryListResponse
		if err := c.Get(ctx, "/v2/sandboxes/registries", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing sandbox registries", err.Error())
			return
		}
		all = append(all, page.Registries...)

		// No total, no next cursor -- a short page means we've reached the end
		// of the trail.
		if len(page.Registries) < sandboxRegistriesPageSize {
			break
		}
		// A server that ignored limit/offset would hand back a full page forever;
		// bound the walk rather than spin, and tell the practitioner we truncated
		// instead of silently returning a partial list as if it were complete.
		offset += len(page.Registries)
		if offset >= sandboxRegistriesMaxResults {
			resp.Diagnostics.AddWarning(
				"Sandbox registry list truncated",
				fmt.Sprintf("Stopped after %d registries. If you genuinely have more than that, please open an issue.", sandboxRegistriesMaxResults),
			)
			break
		}
	}

	elems := make([]attr.Value, 0, len(all))
	for _, reg := range all {
		obj, diags := types.ObjectValue(sandboxRegistriesObjectType.AttrTypes, map[string]attr.Value{
			"id":         types.StringValue(reg.ID),
			"name":       types.StringValue(reg.Name),
			"url":        types.StringValue(reg.URL),
			"created_at": types.StringValue(reg.CreatedAt),
			"created_by": types.StringValue(reg.CreatedBy),
			"updated_at": types.StringValue(reg.UpdatedAt),
			"updated_by": types.StringValue(reg.UpdatedBy),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(sandboxRegistriesObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Registries = list

	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read sandbox registries data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

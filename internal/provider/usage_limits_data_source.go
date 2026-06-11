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

var _ datasource.DataSource = &UsageLimitsDataSource{}

// NewUsageLimitsDataSource returns a data source that lists the usage limits
// configured for the current workspace (or, optionally, the organization) --
// the posted fence lines around how much the herd can graze.
func NewUsageLimitsDataSource() datasource.DataSource {
	return &UsageLimitsDataSource{}
}

type UsageLimitsDataSource struct {
	client *client.Client
}

type UsageLimitsDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	OrgScope    types.Bool   `tfsdk:"org_scope"`
	Limits      types.List   `tfsdk:"limits"`
}

// usageLimitListItemAPI mirrors the UsageLimit schema returned by
// GET /api/v1/usage-limits and GET /api/v1/usage-limits/org (both endpoints
// return the same shape).
type usageLimitListItemAPI struct {
	ID         string `json:"id"`
	LimitType  string `json:"limit_type"`
	LimitValue int64  `json:"limit_value"`
	TenantID   string `json:"tenant_id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

var usageLimitObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":           types.StringType,
	"limit_type":   types.StringType,
	"limit_value":  types.Int64Type,
	"workspace_id": types.StringType,
	"created_at":   types.StringType,
	"updated_at":   types.StringType,
}}

func (d *UsageLimitsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_usage_limits"
}

func (d *UsageLimitsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the LangSmith usage limits configured for the current workspace, or for every workspace in the organization when `org_scope` is set.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source. Only meaningful for the default workspace-scoped listing; ignored by the API when `org_scope` is `true`.",
				Optional:            true,
			},
			"org_scope": schema.BoolAttribute{
				MarkdownDescription: "When `true`, lists usage limits across the whole organization (`/usage-limits/org`) instead of only the current workspace.",
				Optional:            true,
			},
			"limits": schema.ListNestedAttribute{
				MarkdownDescription: "The usage limits.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true, MarkdownDescription: "The unique identifier of the usage limit."},
						"limit_type":   schema.StringAttribute{Computed: true, MarkdownDescription: "The type of usage limit (e.g. `monthly_traces`, `monthly_longlived_traces`)."},
						"limit_value":  schema.Int64Attribute{Computed: true, MarkdownDescription: "The limit value."},
						"workspace_id": schema.StringAttribute{Computed: true, MarkdownDescription: "The workspace the limit applies to (returned by the API as `tenant_id`)."},
						"created_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "The creation timestamp."},
						"updated_at":   schema.StringAttribute{Computed: true, MarkdownDescription: "The last update timestamp."},
					},
				},
			},
		},
	}
}

func (d *UsageLimitsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UsageLimitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UsageLimitsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := "/api/v1/usage-limits"
	if data.OrgScope.ValueBool() {
		path = "/api/v1/usage-limits/org"
	}

	var api []usageLimitListItemAPI
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, path, nil, &api); err != nil {
		resp.Diagnostics.AddError("Error listing usage limits", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(api))
	for i := range api {
		l := api[i]
		id := types.StringNull()
		if l.ID != "" {
			id = types.StringValue(l.ID)
		}
		obj, diags := types.ObjectValue(usageLimitObjectType.AttrTypes, map[string]attr.Value{
			"id":           id,
			"limit_type":   types.StringValue(l.LimitType),
			"limit_value":  types.Int64Value(l.LimitValue),
			"workspace_id": types.StringValue(l.TenantID),
			"created_at":   types.StringValue(l.CreatedAt),
			"updated_at":   types.StringValue(l.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	limits, diags := types.ListValue(usageLimitObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Limits = limits

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &OrganizationsDataSource{}

// NewOrganizationsDataSource returns a data source backed by GET /api/v1/orgs.
func NewOrganizationsDataSource() datasource.DataSource {
	return &OrganizationsDataSource{}
}

// OrganizationsDataSource lists organizations visible to the caller.
type OrganizationsDataSource struct {
	client *client.Client
}

// OrganizationsDataSourceModel is Terraform state for GET /api/v1/orgs.
type OrganizationsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Organizations types.List   `tfsdk:"organizations"`
}

func (d *OrganizationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organizations"
}

func (d *OrganizationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith **organizations** the authenticated identity can access via GET [`/api/v1/orgs`](https://api.smith.langchain.com/openapi.json). " +
			"Each element matches OpenAPI `OrganizationPGSchemaSlim`. " +
			"For only the current organization (GET `/api/v1/orgs/current`), use the `langsmith_organization` data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable placeholder (`organizations`) so the data source can be referenced.",
				Computed:            true,
			},
			"organizations": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: organizationPGSlimNestedAttributes(),
				},
				MarkdownDescription: "Organizations returned by the API, sorted by `id` for stable Terraform state.",
			},
		},
	}
}

func (d *OrganizationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rows []organizationPGSchemaSlimAPI
	if err := d.client.Get(ctx, "/api/v1/orgs", nil, &rows); err != nil {
		resp.Diagnostics.AddError("Error reading organizations", err.Error())
		return
	}

	orgList, diags := organizationPGSlimListValue(rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("organizations")
	data.Organizations = orgList
	tflog.Trace(ctx, "read organizations data source", map[string]any{"count": len(rows)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

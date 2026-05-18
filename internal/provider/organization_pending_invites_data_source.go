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

var _ datasource.DataSource = &OrganizationPendingInvitesDataSource{}

// NewOrganizationPendingInvitesDataSource returns a data source backed by GET /api/v1/orgs/pending.
func NewOrganizationPendingInvitesDataSource() datasource.DataSource {
	return &OrganizationPendingInvitesDataSource{}
}

// OrganizationPendingInvitesDataSource lists pending organization invitations.
type OrganizationPendingInvitesDataSource struct {
	client *client.Client
}

// OrganizationPendingInvitesDataSourceModel is Terraform state for GET /api/v1/orgs/pending.
type OrganizationPendingInvitesDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	PendingInvites types.List   `tfsdk:"pending_invites"`
}

func (d *OrganizationPendingInvitesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_pending_invites"
}

func (d *OrganizationPendingInvitesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists **pending organization invitations** via GET [`/api/v1/orgs/pending`](https://api.smith.langchain.com/openapi.json). " +
			"Each element uses the same `OrganizationPGSchemaSlim` shape as GET `/api/v1/orgs`. " +
			"Accepting or declining invites (`DELETE /api/v1/orgs/pending/{organization_id}`, `POST .../claim`) is not implemented here because those are interactive user flows rather than steady-state infrastructure.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable placeholder (`organization_pending_invites`).",
				Computed:            true,
			},
			"pending_invites": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: organizationPGSlimNestedAttributes(),
				},
				MarkdownDescription: "Pending invites, sorted by organization `id` for stable Terraform state.",
			},
		},
	}
}

func (d *OrganizationPendingInvitesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationPendingInvitesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationPendingInvitesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rows []organizationPGSchemaSlimAPI
	if err := d.client.Get(ctx, "/api/v1/orgs/pending", nil, &rows); err != nil {
		resp.Diagnostics.AddError("Error reading pending organization invites", err.Error())
		return
	}

	pendingList, diags := organizationPGSlimListValue(rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("organization_pending_invites")
	data.PendingInvites = pendingList
	tflog.Trace(ctx, "read organization_pending_invites data source", map[string]any{"count": len(rows)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

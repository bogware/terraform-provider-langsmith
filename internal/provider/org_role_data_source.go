// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &OrgRoleDataSource{}

func NewOrgRoleDataSource() datasource.DataSource {
	return &OrgRoleDataSource{}
}

type OrgRoleDataSource struct {
	client *client.Client
}

type OrgRoleDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	DisplayName    types.String `tfsdk:"display_name"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Permissions    types.String `tfsdk:"permissions"`
	OrganizationID types.String `tfsdk:"organization_id"`
	AccessScope    types.String `tfsdk:"access_scope"`
}

type orgRoleDataSourceAPIResponse struct {
	ID             string          `json:"id"`
	DisplayName    string          `json:"display_name"`
	Name           string          `json:"name"`
	Description    *string         `json:"description"`
	Permissions    json.RawMessage `json:"permissions"`
	OrganizationID string          `json:"organization_id"`
	AccessScope    *string         `json:"access_scope"`
}

func (d *OrgRoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_role"
}

func (d *OrgRoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith organization role by ID or display name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the role. Either `id` or `display_name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the role. Either `id` or `display_name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The internal name of the role.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the role.",
				Computed:            true,
			},
			"permissions": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of permissions.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID.",
				Computed:            true,
			},
			"access_scope": schema.StringAttribute{
				MarkdownDescription: "The access scope of the role.",
				Computed:            true,
			},
		},
	}
}

func (d *OrgRoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrgRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrgRoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.DisplayName.IsNull() && !data.DisplayName.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"display_name\" must be specified.")
		return
	}

	var roles []orgRoleDataSourceAPIResponse
	err := d.client.Get(ctx, "/api/v1/orgs/current/roles", nil, &roles)
	if err != nil {
		resp.Diagnostics.AddError("Error reading org roles", err.Error())
		return
	}

	var found *orgRoleDataSourceAPIResponse
	for i := range roles {
		if idSet && roles[i].ID == data.ID.ValueString() {
			found = &roles[i]
			break
		}
		if nameSet && roles[i].DisplayName == data.DisplayName.ValueString() {
			found = &roles[i]
			break
		}
	}

	if found == nil {
		if idSet {
			resp.Diagnostics.AddError("Org Role Not Found", fmt.Sprintf("No role found with ID %q.", data.ID.ValueString()))
		} else {
			resp.Diagnostics.AddError("Org Role Not Found", fmt.Sprintf("No role found with display name %q.", data.DisplayName.ValueString()))
		}
		return
	}

	data.ID = types.StringValue(found.ID)
	data.DisplayName = types.StringValue(found.DisplayName)
	data.Name = types.StringValue(found.Name)
	data.OrganizationID = types.StringValue(found.OrganizationID)
	data.Permissions = jsonStringValue(found.Permissions)

	if found.Description != nil {
		data.Description = types.StringValue(*found.Description)
	} else {
		data.Description = types.StringNull()
	}
	if found.AccessScope != nil {
		data.AccessScope = types.StringValue(*found.AccessScope)
	} else {
		data.AccessScope = types.StringNull()
	}

	tflog.Trace(ctx, "read org role data source", map[string]interface{}{"id": found.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

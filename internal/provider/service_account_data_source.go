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

var _ datasource.DataSource = &ServiceAccountDataSource{}

func NewServiceAccountDataSource() datasource.DataSource {
	return &ServiceAccountDataSource{}
}

type ServiceAccountDataSource struct {
	client *client.Client
}

type ServiceAccountDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	DefaultWorkspaceID types.String `tfsdk:"default_workspace_id"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func (d *ServiceAccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (d *ServiceAccountDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith service account by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier. Either `id` or `name` must be specified.",
				Optional: true, Computed: true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The service account name. Either `id` or `name` must be specified.",
				Optional: true, Computed: true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID.", Computed: true,
			},
			"default_workspace_id": schema.StringAttribute{
				MarkdownDescription: "The default workspace ID.", Computed: true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.", Computed: true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.", Computed: true,
			},
		},
	}
}

func (d *ServiceAccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ServiceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ServiceAccountDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"name\" must be specified.")
		return
	}

	var listResult serviceAccountListAPIResponse
	err := d.client.Get(ctx, "/api/v1/service-accounts", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service accounts", err.Error())
		return
	}

	var found *serviceAccountAPIResponse
	for i := range listResult {
		if idSet && listResult[i].ID == data.ID.ValueString() {
			found = &listResult[i]
			break
		}
		if nameSet && listResult[i].Name == data.Name.ValueString() {
			found = &listResult[i]
			break
		}
	}

	if found == nil {
		if idSet {
			resp.Diagnostics.AddError("Service Account Not Found", fmt.Sprintf("No service account found with ID %q.", data.ID.ValueString()))
		} else {
			resp.Diagnostics.AddError("Service Account Not Found", fmt.Sprintf("No service account found with name %q.", data.Name.ValueString()))
		}
		return
	}

	data.ID = types.StringValue(found.ID)
	data.Name = types.StringValue(found.Name)
	data.OrganizationID = types.StringValue(found.OrganizationID)
	data.DefaultWorkspaceID = types.StringValue(found.DefaultWorkspaceID)
	data.CreatedAt = types.StringValue(found.CreatedAt)
	data.UpdatedAt = types.StringValue(found.UpdatedAt)

	tflog.Trace(ctx, "read service account data source", map[string]interface{}{"id": found.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &SecretNamesDataSource{}

// NewSecretNamesDataSource returns a data source that lists the key names of
// the secrets stored in the current workspace. Only the names ride out --
// the values stay locked in the strongbox.
func NewSecretNamesDataSource() datasource.DataSource {
	return &SecretNamesDataSource{}
}

type SecretNamesDataSource struct {
	client *client.Client
}

type SecretNamesDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Names       types.List   `tfsdk:"names"`
}

func (d *SecretNamesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_names"
}

func (d *SecretNamesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the key names of the secrets configured in the current LangSmith workspace. " +
			"Secret values are write-only and are **never** returned by the API; this data source only exposes the names.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"names": schema.ListAttribute{
				MarkdownDescription: "The secret key names, sorted alphabetically.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *SecretNamesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SecretNamesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SecretNamesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API returns a bare list of {"key": "..."} objects -- names only,
	// never values.
	var api []secretKeyResponse
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/workspaces/current/secrets", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error listing workspace secrets", err.Error())
		return
	}

	names := make([]string, 0, len(api))
	for _, s := range api {
		names = append(names, s.Key)
	}
	// Sort for a deterministic ordering regardless of API response order.
	sort.Strings(names)

	elems := make([]attr.Value, 0, len(names))
	for _, n := range names {
		elems = append(elems, types.StringValue(n))
	}
	list, diags := types.ListValue(types.StringType, elems)
	resp.Diagnostics.Append(diags...)
	data.Names = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

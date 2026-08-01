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

var _ datasource.DataSource = &SharedTokensDataSource{}

// NewSharedTokensDataSource returns a data source listing everything in the
// workspace that is currently shared through a public link.
func NewSharedTokensDataSource() datasource.DataSource {
	return &SharedTokensDataSource{}
}

// SharedTokensDataSource reads GET /api/v1/workspaces/current/shared.
type SharedTokensDataSource struct {
	client *client.Client
}

// SharedTokensDataSourceModel maps the Terraform schema for the data source.
type SharedTokensDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Entities    types.String `tfsdk:"entities"`
	Count       types.Int64  `tfsdk:"entity_count"`
}

// sharedTokensAPIResponse mirrors the wire format. The entity shape varies by
// what was shared (a run, a dataset, ...) and the API does not pin it down, so
// it is surfaced as raw JSON rather than guessed at.
type sharedTokensAPIResponse struct {
	Entities []json.RawMessage `json:"entities"`
}

func (d *SharedTokensDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_tokens"
}

func (d *SharedTokensDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the entities in a LangSmith workspace that are currently shared through a public link — the runs and datasets published by `langsmith_run_share` and `langsmith_dataset_share`, including any shared outside Terraform.\n\n" +
			"**A share token is an unauthenticated capability**: anyone holding the link can read the shared content, which may include end-user data captured in traces. Use this data source to audit what is exposed.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"entities": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of the shared entities. The shape of each entry depends on what was shared and is not fixed by the API, so it is returned verbatim; decode it with `jsondecode()`.",
				Computed:            true,
			},
			"entity_count": schema.Int64Attribute{
				MarkdownDescription: "Number of shared entities returned.",
				Computed:            true,
			},
		},
	}
}

func (d *SharedTokensDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SharedTokensDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SharedTokensDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var apiResp sharedTokensAPIResponse
	if err := c.Get(ctx, "/api/v1/workspaces/current/shared", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing shared tokens", err.Error())
		return
	}

	encoded, err := json.Marshal(apiResp.Entities)
	if err != nil {
		resp.Diagnostics.AddError("Error encoding shared entities", err.Error())
		return
	}
	data.Entities = types.StringValue(normalizeJSON(string(encoded)))
	data.Count = types.Int64Value(int64(len(apiResp.Entities)))

	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read shared tokens data source", map[string]interface{}{"count": len(apiResp.Entities)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

var _ datasource.DataSource = &EvaluatorSpendDataSource{}

func NewEvaluatorSpendDataSource() datasource.DataSource {
	return &EvaluatorSpendDataSource{}
}

type EvaluatorSpendDataSource struct {
	client *client.Client
}

type EvaluatorSpendDataSourceModel struct {
	SpendJSON   types.String `tfsdk:"spend_json"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

func (d *EvaluatorSpendDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator_spend"
}

func (d *EvaluatorSpendDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns evaluator spend information for the current LangSmith workspace. The response shape is not yet stable, so it is exposed as a JSON-encoded string.",
		Attributes: map[string]schema.Attribute{
			"spend_json": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded evaluator spend report as returned by the API.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *EvaluatorSpendDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EvaluatorSpendDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EvaluatorSpendDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var raw json.RawMessage
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/platform/evaluators/spend", nil, &raw); err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Evaluator Spend Not Found", "The evaluator spend endpoint returned 404. It may not be available on this LangSmith deployment.")
			return
		}
		resp.Diagnostics.AddError("Error reading evaluator spend", err.Error())
		return
	}

	data.SpendJSON = jsonStringValue(raw)

	tflog.Trace(ctx, "read evaluator spend data source", nil)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

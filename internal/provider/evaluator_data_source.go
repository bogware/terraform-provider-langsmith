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

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &EvaluatorDataSource{}

func NewEvaluatorDataSource() datasource.DataSource {
	return &EvaluatorDataSource{}
}

type EvaluatorDataSource struct {
	client *client.Client
}

type EvaluatorDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Type              types.String `tfsdk:"type"`
	TenantID          types.String `tfsdk:"tenant_id"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	CodeEvaluatorJSON types.String `tfsdk:"code_evaluator_json"`
	LLMEvaluatorJSON  types.String `tfsdk:"llm_evaluator_json"`
}

func (d *EvaluatorDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator"
}

func (d *EvaluatorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a LangSmith evaluator by ID. The nested `code_evaluator` / `llm_evaluator` payloads are surfaced as JSON-encoded strings for downstream consumption.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Required: true},
			"name":                schema.StringAttribute{Computed: true},
			"type":                schema.StringAttribute{Computed: true},
			"tenant_id":           schema.StringAttribute{Computed: true},
			"created_at":          schema.StringAttribute{Computed: true},
			"updated_at":          schema.StringAttribute{Computed: true},
			"code_evaluator_json": schema.StringAttribute{Computed: true},
			"llm_evaluator_json":  schema.StringAttribute{Computed: true},
		},
	}
}

func (d *EvaluatorDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EvaluatorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EvaluatorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api evaluatorAPI
	if err := d.client.Get(ctx, "/v1/platform/evaluators/"+data.ID.ValueString(), nil, &api); err != nil {
		resp.Diagnostics.AddError("Error reading evaluator", err.Error())
		return
	}
	data.Name = types.StringValue(api.Name)
	data.Type = types.StringValue(api.Type)
	data.TenantID = types.StringValue(api.TenantID)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	if api.CodeEvaluator != nil {
		b, _ := json.Marshal(api.CodeEvaluator)
		data.CodeEvaluatorJSON = jsonStringValue(b)
	} else {
		data.CodeEvaluatorJSON = types.StringNull()
	}
	if api.LLMEvaluator != nil {
		b, _ := json.Marshal(api.LLMEvaluator)
		data.LLMEvaluatorJSON = jsonStringValue(b)
	} else {
		data.LLMEvaluatorJSON = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

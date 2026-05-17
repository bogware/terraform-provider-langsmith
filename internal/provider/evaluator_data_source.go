// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &EvaluatorDataSource{}

// NewEvaluatorDataSource returns a data source that reads one evaluator by ID.
func NewEvaluatorDataSource() datasource.DataSource {
	return &EvaluatorDataSource{}
}

// EvaluatorDataSource reads `GET /v1/platform/evaluators/{evaluator_id}`.
type EvaluatorDataSource struct {
	client *client.Client
}

// EvaluatorDataSourceModel is the Terraform model for the evaluator data source.
type EvaluatorDataSourceModel struct {
	EvaluatorID   types.String `tfsdk:"evaluator_id"`
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	LLMEvaluator  types.String `tfsdk:"llm_evaluator"`
	CodeEvaluator types.String `tfsdk:"code_evaluator"`
	TenantID      types.String `tfsdk:"tenant_id"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
	CreatedBy     types.String `tfsdk:"created_by"`
	FeedbackKeys  types.List   `tfsdk:"feedback_keys"`
	RunRules      types.String `tfsdk:"run_rules"`
}

func (d *EvaluatorDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator"
}

func (d *EvaluatorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single LangSmith hosted evaluator by `evaluator_id` (`GET /v1/platform/evaluators/{evaluator_id}`).",
		Attributes: map[string]schema.Attribute{
			"evaluator_id": schema.StringAttribute{
				MarkdownDescription: "Evaluator ID (OpenAPI path parameter `evaluator_id`).",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Same as `evaluator_id` (stable identifier from the API).",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Evaluator display name.",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "`llm` or `code`.",
				Computed:            true,
			},
			"llm_evaluator": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded `evaluators.LLMEvaluator` when `type` is `llm`.",
				Computed:            true,
			},
			"code_evaluator": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded `evaluators.CodeEvaluator` when `type` is `code`.",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Workspace tenant ID.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "Creator user ID.",
				Computed:            true,
			},
			"feedback_keys": schema.ListAttribute{
				MarkdownDescription: "Associated feedback keys.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"run_rules": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded `run_rules` array.",
				Computed:            true,
			},
		},
	}
}

func (d *EvaluatorDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EvaluatorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EvaluatorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	evalID := data.EvaluatorID.ValueString()
	var ev evaluatorAPI
	if err := d.client.Get(ctx, evaluatorDetailPath(evalID), nil, &ev); err != nil {
		resp.Diagnostics.AddError("Error reading evaluator", err.Error())
		return
	}

	data.ID = types.StringValue(ev.ID)
	data.EvaluatorID = types.StringValue(ev.ID)
	data.Name = types.StringValue(ev.Name)
	data.Type = types.StringValue(ev.Type)
	data.TenantID = types.StringValue(ev.TenantID)
	data.CreatedAt = types.StringValue(ev.CreatedAt)
	data.UpdatedAt = types.StringValue(ev.UpdatedAt)
	data.CreatedBy = types.StringValue(ev.CreatedBy)

	listVals, ldiags := types.ListValue(types.StringType, stringSliceToAttrValues(ev.FeedbackKeys))
	resp.Diagnostics.Append(ldiags...)
	data.FeedbackKeys = listVals

	data.RunRules = jsonStringValue(ev.RunRules)

	switch ev.Type {
	case "code":
		data.CodeEvaluator = jsonStringValue(ev.CodeEvaluator)
		data.LLMEvaluator = types.StringNull()
	case "llm":
		data.LLMEvaluator = jsonStringValue(ev.LLMEvaluator)
		data.CodeEvaluator = types.StringNull()
	default:
		data.LLMEvaluator = jsonStringValue(ev.LLMEvaluator)
		data.CodeEvaluator = jsonStringValue(ev.CodeEvaluator)
	}

	tflog.Trace(ctx, "read evaluator data source", map[string]any{"evaluator_id": evalID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

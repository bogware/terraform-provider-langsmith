// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &EvaluatorsDataSource{}

func NewEvaluatorsDataSource() datasource.DataSource {
	return &EvaluatorsDataSource{}
}

type EvaluatorsDataSource struct {
	client *client.Client
}

type EvaluatorsDataSourceModel struct {
	Type         types.String `tfsdk:"type"`
	NameContains types.String `tfsdk:"name_contains"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Evaluators   types.List   `tfsdk:"evaluators"`
}

// evaluatorsListItemAPI mirrors a single evaluator in
// GET /v1/platform/evaluators responses. The OpenAPI spec reports the
// workspace as tenant_id while the singular endpoint uses workspace_id, so
// both keys are decoded and reconciled.
type evaluatorsListItemAPI struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	WorkspaceID   string          `json:"workspace_id"`
	TenantID      string          `json:"tenant_id"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	CodeEvaluator json.RawMessage `json:"code_evaluator"`
	LLMEvaluator  json.RawMessage `json:"llm_evaluator"`
}

type evaluatorsListAPIResponse struct {
	Evaluators []evaluatorsListItemAPI `json:"evaluators"`
	Total      int                     `json:"total"`
}

var evaluatorObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                  types.StringType,
	"name":                types.StringType,
	"type":                types.StringType,
	"workspace_id":        types.StringType,
	"created_at":          types.StringType,
	"updated_at":          types.StringType,
	"code_evaluator_json": types.StringType,
	"llm_evaluator_json":  types.StringType,
}}

func (d *EvaluatorsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluators"
}

func (d *EvaluatorsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith evaluators, with optional filters. The nested `code_evaluator` / `llm_evaluator` payloads are surfaced as JSON-encoded strings. All pages are fetched.",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Filter by evaluator type (e.g. `code`, `llm`).",
				Optional:            true,
			},
			"name_contains": schema.StringAttribute{
				MarkdownDescription: "Filter to evaluators whose name contains this substring.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"evaluators": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                  schema.StringAttribute{Computed: true},
						"name":                schema.StringAttribute{Computed: true},
						"type":                schema.StringAttribute{Computed: true},
						"workspace_id":        schema.StringAttribute{Computed: true},
						"created_at":          schema.StringAttribute{Computed: true},
						"updated_at":          schema.StringAttribute{Computed: true},
						"code_evaluator_json": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded code evaluator payload."},
						"llm_evaluator_json":  schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded LLM evaluator payload."},
					},
				},
			},
		},
	}
}

func (d *EvaluatorsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EvaluatorsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EvaluatorsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	const pageSize = 100
	var evaluators []evaluatorsListItemAPI
	for offset := 0; ; offset += pageSize {
		query := url.Values{}
		if !data.Type.IsNull() && !data.Type.IsUnknown() {
			query.Set("type", data.Type.ValueString())
		}
		if !data.NameContains.IsNull() && !data.NameContains.IsUnknown() {
			query.Set("name_contains", data.NameContains.ValueString())
		}
		query.Set("limit", strconv.Itoa(pageSize))
		query.Set("offset", strconv.Itoa(offset))

		var page evaluatorsListAPIResponse
		if err := c.Get(ctx, "/v1/platform/evaluators", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing evaluators", err.Error())
			return
		}
		evaluators = append(evaluators, page.Evaluators...)
		if len(page.Evaluators) < pageSize || len(evaluators) >= page.Total {
			break
		}
	}

	elems := make([]attr.Value, 0, len(evaluators))
	for _, ev := range evaluators {
		workspaceID := ev.WorkspaceID
		if workspaceID == "" {
			workspaceID = ev.TenantID
		}
		obj, diags := types.ObjectValue(evaluatorObjectType.AttrTypes, map[string]attr.Value{
			"id":                  types.StringValue(ev.ID),
			"name":                types.StringValue(ev.Name),
			"type":                types.StringValue(ev.Type),
			"workspace_id":        types.StringValue(workspaceID),
			"created_at":          types.StringValue(ev.CreatedAt),
			"updated_at":          types.StringValue(ev.UpdatedAt),
			"code_evaluator_json": jsonStringValue(ev.CodeEvaluator),
			"llm_evaluator_json":  jsonStringValue(ev.LLMEvaluator),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(evaluatorObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Evaluators = list

	tflog.Trace(ctx, "read evaluators data source", map[string]interface{}{"count": len(evaluators)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

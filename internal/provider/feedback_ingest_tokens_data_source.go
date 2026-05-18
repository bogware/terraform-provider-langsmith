// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &FeedbackIngestTokensDataSource{}

// feedbackIngestTokenListObjectAttrTypes matches each element of `tokens`
// (OpenAPI `FeedbackIngestTokenSchema`).
var feedbackIngestTokenListObjectAttrTypes = map[string]attr.Type{
	"id":           types.StringType,
	"url":          types.StringType,
	"expires_at":   types.StringType,
	"feedback_key": types.StringType,
}

// NewFeedbackIngestTokensDataSource returns a data source for GET /api/v1/feedback/tokens.
func NewFeedbackIngestTokensDataSource() datasource.DataSource {
	return &FeedbackIngestTokensDataSource{}
}

// FeedbackIngestTokensDataSource lists feedback ingest tokens for a run.
type FeedbackIngestTokensDataSource struct {
	client *client.Client
}

// FeedbackIngestTokensDataSourceModel is Terraform state for the listing.
type FeedbackIngestTokensDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	RunID  types.String `tfsdk:"run_id"`
	Tokens types.List   `tfsdk:"tokens"`
}

func (d *FeedbackIngestTokensDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_ingest_tokens"
}

func (d *FeedbackIngestTokensDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists **feedback ingest tokens** for a run via GET `/api/v1/feedback/tokens?run_id=...` (OpenAPI `FeedbackIngestTokenSchema` items). " +
			"Use this data source to inspect tokens minted outside Terraform or to verify tokens created by `langsmith_feedback_ingest_token`. " +
			"Submitting feedback with a token (`GET`/`POST /api/v1/feedback/tokens/{token}`) is not exposed here because it is an operational API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable identifier for this data source, set to `run_id`.",
				Computed:            true,
			},
			"run_id": schema.StringAttribute{
				MarkdownDescription: "Run UUID whose ingest tokens are listed (required query parameter on the LangSmith API).",
				Required:            true,
			},
			"tokens": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Token UUID (path parameter for `/api/v1/feedback/tokens/{token}`).",
						},
						"url": schema.StringAttribute{
							Computed:            true,
							Sensitive:           true,
							MarkdownDescription: "Full ingest URL (secret; authorizes feedback submission).",
						},
						"expires_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Expiration timestamp in RFC3339 form.",
						},
						"feedback_key": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Feedback key for this ingest token.",
						},
					},
				},
				MarkdownDescription: "Ingest tokens returned by the API.",
			},
		},
	}
}

func (d *FeedbackIngestTokensDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FeedbackIngestTokensDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FeedbackIngestTokensDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runID := strings.TrimSpace(data.RunID.ValueString())
	if runID == "" {
		resp.Diagnostics.AddError("Invalid run_id", "run_id cannot be empty.")
		return
	}

	q := url.Values{}
	q.Set("run_id", runID)
	var results []feedbackTokenAPI
	if err := d.client.Get(ctx, "/api/v1/feedback/tokens", q, &results); err != nil {
		resp.Diagnostics.AddError("Error reading feedback ingest tokens", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for i := range results {
		t := &results[i]
		obj := types.ObjectValueMust(feedbackIngestTokenListObjectAttrTypes, map[string]attr.Value{
			"id":           types.StringValue(t.ID),
			"url":          types.StringValue(t.URL),
			"expires_at":   types.StringValue(t.ExpiresAt),
			"feedback_key": types.StringValue(t.FeedbackKey),
		})
		elems = append(elems, obj)
	}

	tokenList, listDiags := types.ListValue(types.ObjectType{AttrTypes: feedbackIngestTokenListObjectAttrTypes}, elems)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(runID)
	data.RunID = types.StringValue(runID)
	data.Tokens = tokenList

	tflog.Trace(ctx, "read feedback ingest tokens data source", map[string]interface{}{"run_id": runID, "count": len(results)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// feedbackFormulasPageSize is the page size used when listing feedback
// formulas. The API defaults `limit` to 20; we request full pages and stop when
// a short page signals the end.
const feedbackFormulasPageSize = 100

var (
	_ datasource.DataSource                     = &FeedbackFormulasDataSource{}
	_ datasource.DataSourceWithConfigValidators = &FeedbackFormulasDataSource{}
)

// NewFeedbackFormulasDataSource returns a new FeedbackFormulasDataSource that
// lists the feedback formulas scoped to a dataset or a project (session).
func NewFeedbackFormulasDataSource() datasource.DataSource {
	return &FeedbackFormulasDataSource{}
}

// FeedbackFormulasDataSource lists LangSmith feedback formulas via
// GET /api/v1/feedback/formulas. The endpoint requires a scope: an unfiltered
// read is rejected with an HTTP 400 ("You must either provide a dataset_id or a
// session_id"), so a ConfigValidator enforces exactly one of them at plan time.
type FeedbackFormulasDataSource struct {
	client *client.Client
}

// FeedbackFormulasDataSourceModel holds the scope inputs and the resulting
// formulas list.
type FeedbackFormulasDataSourceModel struct {
	DatasetID   types.String `tfsdk:"dataset_id"`
	SessionID   types.String `tfsdk:"session_id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Formulas    types.List   `tfsdk:"formulas"`
}

// feedbackFormulaListAPI mirrors the FeedbackFormula schema returned by
// GET /api/v1/feedback/formulas. The endpoint returns no workspace field, so
// there is nothing to decode for workspace_id / tenant_id here.
type feedbackFormulaListAPI struct {
	ID              string          `json:"id"`
	FeedbackKey     string          `json:"feedback_key"`
	AggregationType string          `json:"aggregation_type"`
	FormulaParts    json.RawMessage `json:"formula_parts"`
	DatasetID       *string         `json:"dataset_id"`
	SessionID       *string         `json:"session_id"`
	CreatedAt       string          `json:"created_at"`
	ModifiedAt      string          `json:"modified_at"`
}

var feedbackFormulaObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":               types.StringType,
	"feedback_key":     types.StringType,
	"aggregation_type": types.StringType,
	"formula_parts":    types.StringType,
	"dataset_id":       types.StringType,
	"session_id":       types.StringType,
	"created_at":       types.StringType,
	"modified_at":      types.StringType,
}}

func (d *FeedbackFormulasDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_formulas"
}

// ConfigValidators enforces the API's scoping requirement at plan time so a
// misconfiguration surfaces as a clean Terraform error instead of an HTTP 400
// during apply.
func (d *FeedbackFormulasDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("dataset_id"),
			path.MatchRoot("session_id"),
		),
	}
}

func (d *FeedbackFormulasDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith feedback formulas scoped to a dataset or a project (session).\n\n" +
			"Exactly one of `dataset_id` or `session_id` must be set: the LangSmith API rejects an unscoped listing with an HTTP 400.",
		Attributes: map[string]schema.Attribute{
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "List the feedback formulas scoped to this dataset. Exactly one of `dataset_id` or `session_id` must be set.",
				Optional:            true,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "List the feedback formulas scoped to this project (session). Exactly one of `dataset_id` or `session_id` must be set.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list feedback formulas from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"formulas": schema.ListNestedAttribute{
				MarkdownDescription: "The matching feedback formulas.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the feedback formula.",
							Computed:            true,
						},
						"feedback_key": schema.StringAttribute{
							MarkdownDescription: "The feedback key the formula computes.",
							Computed:            true,
						},
						"aggregation_type": schema.StringAttribute{
							MarkdownDescription: "How the formula parts are aggregated (`sum` or `avg`).",
							Computed:            true,
						},
						"formula_parts": schema.StringAttribute{
							MarkdownDescription: "JSON string of the formula parts. Each part is `{\"part_type\": \"weighted_key\", \"weight\": 1.0, \"key\": \"feedback_key\"}`.",
							Computed:            true,
						},
						"dataset_id": schema.StringAttribute{
							MarkdownDescription: "The dataset the formula is scoped to, if any.",
							Computed:            true,
						},
						"session_id": schema.StringAttribute{
							MarkdownDescription: "The project (session) the formula is scoped to, if any.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the feedback formula.",
							Computed:            true,
						},
						"modified_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the feedback formula.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *FeedbackFormulasDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FeedbackFormulasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FeedbackFormulasDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var all []feedbackFormulaListAPI
	for offset := 0; ; offset += feedbackFormulasPageSize {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(feedbackFormulasPageSize))
		query.Set("offset", strconv.Itoa(offset))
		if !data.DatasetID.IsNull() && !data.DatasetID.IsUnknown() {
			query.Set("dataset_id", data.DatasetID.ValueString())
		}
		if !data.SessionID.IsNull() && !data.SessionID.IsUnknown() {
			query.Set("session_id", data.SessionID.ValueString())
		}

		var page []feedbackFormulaListAPI
		if err := c.Get(ctx, "/api/v1/feedback/formulas", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing feedback formulas", err.Error())
			return
		}
		all = append(all, page...)
		if len(page) < feedbackFormulasPageSize {
			break
		}
	}

	elems := make([]attr.Value, 0, len(all))
	for _, f := range all {
		obj, diags := types.ObjectValue(feedbackFormulaObjectType.AttrTypes, map[string]attr.Value{
			"id":               types.StringValue(f.ID),
			"feedback_key":     types.StringValue(f.FeedbackKey),
			"aggregation_type": types.StringValue(f.AggregationType),
			"formula_parts":    jsonStringValue(f.FormulaParts),
			"dataset_id":       types.StringPointerValue(f.DatasetID),
			"session_id":       types.StringPointerValue(f.SessionID),
			"created_at":       types.StringValue(f.CreatedAt),
			"modified_at":      types.StringValue(f.ModifiedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(feedbackFormulaObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Formulas = list

	// GET /api/v1/feedback/formulas returns no workspace field, so there is no
	// API value to reconcile against; fall back to the client's workspace.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read feedback formulas data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

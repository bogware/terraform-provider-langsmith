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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// examplesPageSize is the page size used when listing examples. The API caps
// `limit` at 100, so we page until a short page signals the end.
const examplesPageSize = 100

var _ datasource.DataSource = &ExamplesDataSource{}

// NewExamplesDataSource returns a new ExamplesDataSource that lists the
// examples in a dataset.
func NewExamplesDataSource() datasource.DataSource {
	return &ExamplesDataSource{}
}

// ExamplesDataSource lists LangSmith dataset examples via
// GET /api/v1/examples. The endpoint rejects unfiltered reads with a 400
// ("Either dataset_id or id is required..."), so `dataset_id` is required here.
type ExamplesDataSource struct {
	client *client.Client
}

// ExamplesDataSourceModel holds the filter inputs and the resulting examples list.
type ExamplesDataSourceModel struct {
	DatasetID        types.String `tfsdk:"dataset_id"`
	AsOf             types.String `tfsdk:"as_of"`
	Splits           types.List   `tfsdk:"splits"`
	Metadata         types.String `tfsdk:"metadata"`
	FullTextContains types.List   `tfsdk:"full_text_contains"`
	Filter           types.String `tfsdk:"filter"`
	Offset           types.Int64  `tfsdk:"offset"`
	Limit            types.Int64  `tfsdk:"limit"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	Examples         types.List   `tfsdk:"examples"`
}

// exampleListAPI mirrors the Example schema returned by GET /api/v1/examples.
// The list endpoint returns no workspace field at all, so there is nothing to
// decode for workspace_id / tenant_id here.
type exampleListAPI struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	DatasetID          string          `json:"dataset_id"`
	Inputs             json.RawMessage `json:"inputs"`
	Outputs            json.RawMessage `json:"outputs"`
	Metadata           json.RawMessage `json:"metadata"`
	SourceRunID        *string         `json:"source_run_id"`
	SourceSessionID    *string         `json:"source_session_id"`
	SourceRunStartTime *string         `json:"source_run_start_time"`
	SourceTraceID      *string         `json:"source_trace_id"`
	AttachmentURLs     json.RawMessage `json:"attachment_urls"`
	CreatedAt          string          `json:"created_at"`
	ModifiedAt         *string         `json:"modified_at"`
}

var exampleObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                    types.StringType,
	"name":                  types.StringType,
	"dataset_id":            types.StringType,
	"inputs":                types.StringType,
	"outputs":               types.StringType,
	"metadata":              types.StringType,
	"source_run_id":         types.StringType,
	"source_session_id":     types.StringType,
	"source_run_start_time": types.StringType,
	"source_trace_id":       types.StringType,
	"attachment_urls":       types.StringType,
	"created_at":            types.StringType,
	"modified_at":           types.StringType,
}}

func (d *ExamplesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_examples"
}

func (d *ExamplesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the examples in a LangSmith dataset, optionally filtered by split, metadata, or full-text search.\n\n" +
			"`dataset_id` is required: the LangSmith API rejects an unfiltered example listing with an HTTP 400.",
		Attributes: map[string]schema.Attribute{
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the dataset whose examples should be listed. Required — the API returns an error if no dataset is supplied.",
				Required:            true,
			},
			"as_of": schema.StringAttribute{
				MarkdownDescription: "Read the dataset as of a specific version: either a dataset version tag (for example `latest`) or an RFC 3339 timestamp. Only modifications made on or before this point are included. Defaults to the latest version.",
				Optional:            true,
			},
			"splits": schema.ListAttribute{
				MarkdownDescription: "Return only examples belonging to one of these dataset splits (for example `base`, `train`, `test`).",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON string of metadata key/value pairs to filter examples by (for example `jsonencode({ source = \"prod\" })`). Only examples whose metadata contains every supplied pair are returned.",
				Optional:            true,
			},
			"full_text_contains": schema.ListAttribute{
				MarkdownDescription: "Return only examples whose inputs or outputs contain all of these substrings.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "A LangSmith filter-query string applied server-side to the examples (for example `has(metadata, '{\"source\": \"prod\"}')`).",
				Optional:            true,
			},
			"offset": schema.Int64Attribute{
				MarkdownDescription: "Number of examples to skip before returning results. Defaults to `0`.",
				Optional:            true,
				Validators:          []validator.Int64{int64validator.AtLeast(0)},
			},
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of examples to return (1-100). When omitted, the data source pages through the API and returns every matching example.",
				Optional:            true,
				Validators:          []validator.Int64{int64validator.Between(1, examplesPageSize)},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list examples from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"examples": schema.ListNestedAttribute{
				MarkdownDescription: "The matching examples.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the example.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The display name the API derives for the example.",
							Computed:            true,
						},
						"dataset_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the dataset this example belongs to.",
							Computed:            true,
						},
						"inputs": schema.StringAttribute{
							MarkdownDescription: "JSON string containing the input data for the example.",
							Computed:            true,
						},
						"outputs": schema.StringAttribute{
							MarkdownDescription: "JSON string containing the output data for the example.",
							Computed:            true,
						},
						"metadata": schema.StringAttribute{
							MarkdownDescription: "JSON string containing the example metadata. The API stores the example's split under the `dataset_split` key here.",
							Computed:            true,
						},
						"source_run_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the run this example was created from, if any.",
							Computed:            true,
						},
						"source_session_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the project (session) the source run belongs to, if any.",
							Computed:            true,
						},
						"source_run_start_time": schema.StringAttribute{
							MarkdownDescription: "The start time of the source run, if any.",
							Computed:            true,
						},
						"source_trace_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the trace the source run belongs to, if any.",
							Computed:            true,
						},
						"attachment_urls": schema.StringAttribute{
							MarkdownDescription: "JSON string of the example's attachment URLs, keyed by attachment name.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp of the example.",
							Computed:            true,
						},
						"modified_at": schema.StringAttribute{
							MarkdownDescription: "The last modification timestamp of the example.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ExamplesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ExamplesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ExamplesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	// Build the filter half of the query once; only limit/offset change per page.
	base := url.Values{}
	// The wire parameter is `dataset`, NOT `dataset_id` — despite the API's own
	// 400 message ("Either dataset_id or id is required...") saying otherwise.
	// Sending `dataset_id` is silently ignored and the request 400s.
	base.Set("dataset", data.DatasetID.ValueString())
	if !data.AsOf.IsNull() && !data.AsOf.IsUnknown() {
		base.Set("as_of", data.AsOf.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		base.Set("metadata", data.Metadata.ValueString())
	}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		base.Set("filter", data.Filter.ValueString())
	}

	// splits and full_text_contains are repeated query parameters.
	for _, item := range []struct {
		key  string
		list types.List
	}{
		{"splits", data.Splits},
		{"full_text_contains", data.FullTextContains},
	} {
		if item.list.IsNull() || item.list.IsUnknown() {
			continue
		}
		var values []string
		resp.Diagnostics.Append(item.list.ElementsAs(ctx, &values, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, v := range values {
			base.Add(item.key, v)
		}
	}

	offset := 0
	if !data.Offset.IsNull() && !data.Offset.IsUnknown() {
		offset = int(data.Offset.ValueInt64())
	}

	// An explicit `limit` means "give me exactly this window": issue a single
	// request. Without it, page through the dataset until a short page ends it.
	singlePage := !data.Limit.IsNull() && !data.Limit.IsUnknown()
	pageSize := examplesPageSize
	if singlePage {
		pageSize = int(data.Limit.ValueInt64())
	}

	var all []exampleListAPI
	for {
		query := url.Values{}
		for k, vs := range base {
			for _, v := range vs {
				query.Add(k, v)
			}
		}
		query.Set("limit", strconv.Itoa(pageSize))
		query.Set("offset", strconv.Itoa(offset))

		var page []exampleListAPI
		if err := c.Get(ctx, "/api/v1/examples", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing examples", err.Error())
			return
		}
		all = append(all, page...)
		if singlePage || len(page) < pageSize {
			break
		}
		offset += pageSize
	}

	elems := make([]attr.Value, 0, len(all))
	for _, ex := range all {
		obj, diags := types.ObjectValue(exampleObjectType.AttrTypes, map[string]attr.Value{
			"id":                    types.StringValue(ex.ID),
			"name":                  types.StringValue(ex.Name),
			"dataset_id":            types.StringValue(ex.DatasetID),
			"inputs":                jsonStringValue(ex.Inputs),
			"outputs":               jsonStringValue(ex.Outputs),
			"metadata":              jsonStringValue(ex.Metadata),
			"source_run_id":         types.StringPointerValue(ex.SourceRunID),
			"source_session_id":     types.StringPointerValue(ex.SourceSessionID),
			"source_run_start_time": types.StringPointerValue(ex.SourceRunStartTime),
			"source_trace_id":       types.StringPointerValue(ex.SourceTraceID),
			"attachment_urls":       jsonStringValue(ex.AttachmentURLs),
			"created_at":            types.StringValue(ex.CreatedAt),
			"modified_at":           types.StringPointerValue(ex.ModifiedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(exampleObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Examples = list

	// GET /api/v1/examples returns no workspace field, so there is no API value
	// to reconcile against; fall back to the client's workspace.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read examples data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
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

// bulkExportsPageSize is the page size used when listing bulk exports. The API
// caps `limit` at 1000, but we page in smaller chunks and stop as soon as a
// short page signals the end of the list.
const bulkExportsPageSize = 100

var _ datasource.DataSource = &BulkExportsDataSource{}

// NewBulkExportsDataSource returns a new BulkExportsDataSource that lists all
// bulk export jobs in a workspace.
func NewBulkExportsDataSource() datasource.DataSource {
	return &BulkExportsDataSource{}
}

// BulkExportsDataSource lists LangSmith bulk exports. It pages through
// GET /api/v1/bulk-exports until every export has been collected.
type BulkExportsDataSource struct {
	client *client.Client
}

// BulkExportsDataSourceModel holds the workspace override and the resulting
// bulk exports list.
type BulkExportsDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	BulkExports types.List   `tfsdk:"bulk_exports"`
}

// bulkExportListAPI mirrors the BulkExport schema returned by
// GET /api/v1/bulk-exports. The API documents tenant_id; workspace_id is also
// decoded in case the server returns it as an alias.
type bulkExportListAPI struct {
	ID                      string   `json:"id"`
	BulkExportDestinationID string   `json:"bulk_export_destination_id"`
	SessionID               *string  `json:"session_id"`
	AllExperiments          *bool    `json:"all_experiments"`
	SourceBulkExportID      *string  `json:"source_bulk_export_id"`
	StartTime               string   `json:"start_time"`
	EndTime                 *string  `json:"end_time"`
	Filter                  *string  `json:"filter"`
	Format                  string   `json:"format"`
	FormatVersion           string   `json:"format_version"`
	Compression             string   `json:"compression"`
	IntervalHours           *int64   `json:"interval_hours"`
	ExportFields            []string `json:"export_fields"`
	Status                  string   `json:"status"`
	WorkspaceID             string   `json:"workspace_id"`
	TenantID                string   `json:"tenant_id"`
	CreatedAt               string   `json:"created_at"`
	UpdatedAt               string   `json:"updated_at"`
	FinishedAt              *string  `json:"finished_at"`
}

var bulkExportObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                         types.StringType,
	"bulk_export_destination_id": types.StringType,
	"session_id":                 types.StringType,
	"all_experiments":            types.BoolType,
	"source_bulk_export_id":      types.StringType,
	"start_time":                 types.StringType,
	"end_time":                   types.StringType,
	"filter":                     types.StringType,
	"format":                     types.StringType,
	"format_version":             types.StringType,
	"compression":                types.StringType,
	"interval_hours":             types.Int64Type,
	"export_fields":              types.ListType{ElemType: types.StringType},
	"status":                     types.StringType,
	"workspace_id":               types.StringType,
	"created_at":                 types.StringType,
	"updated_at":                 types.StringType,
	"finished_at":                types.StringType,
}}

func (d *BulkExportsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bulk_exports"
}

func (d *BulkExportsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all LangSmith bulk export jobs in a workspace.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list bulk exports from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"bulk_exports": schema.ListNestedAttribute{
				MarkdownDescription: "The bulk exports in the workspace.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the bulk export.",
							Computed:            true,
						},
						"bulk_export_destination_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the bulk export destination the export writes to.",
							Computed:            true,
						},
						"session_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the project/session being exported.",
							Computed:            true,
						},
						"all_experiments": schema.BoolAttribute{
							MarkdownDescription: "Whether the export covers all experiments rather than a single session.",
							Computed:            true,
						},
						"source_bulk_export_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the bulk export this export was derived from, for scheduled/interval exports.",
							Computed:            true,
						},
						"start_time": schema.StringAttribute{
							MarkdownDescription: "The start time of the exported window, in RFC3339 format.",
							Computed:            true,
						},
						"end_time": schema.StringAttribute{
							MarkdownDescription: "The end time of the exported window, in RFC3339 format.",
							Computed:            true,
						},
						"filter": schema.StringAttribute{
							MarkdownDescription: "The filter expression applied to the exported runs.",
							Computed:            true,
						},
						"format": schema.StringAttribute{
							MarkdownDescription: "The export format (e.g. `Parquet`).",
							Computed:            true,
						},
						"format_version": schema.StringAttribute{
							MarkdownDescription: "The format version (`v1` or `v2_beta`).",
							Computed:            true,
						},
						"compression": schema.StringAttribute{
							MarkdownDescription: "The compression type (`none`, `gzip`, `snappy`, or `zstandard`).",
							Computed:            true,
						},
						"interval_hours": schema.Int64Attribute{
							MarkdownDescription: "The interval in hours for recurring exports.",
							Computed:            true,
						},
						"export_fields": schema.ListAttribute{
							MarkdownDescription: "The list of run fields included in the export.",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "The status of the bulk export (`Created`, `Running`, `Completed`, `Cancelled`, `Failed`, `TimedOut`, or `IntervalScheduled`).",
							Computed:            true,
						},
						"workspace_id": schema.StringAttribute{
							MarkdownDescription: "The workspace ID that owns the bulk export.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "The creation timestamp.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "The last update timestamp.",
							Computed:            true,
						},
						"finished_at": schema.StringAttribute{
							MarkdownDescription: "The timestamp when the export finished.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *BulkExportsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BulkExportsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BulkExportsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var all []bulkExportListAPI
	for offset := 0; ; offset += bulkExportsPageSize {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(bulkExportsPageSize))
		query.Set("offset", strconv.Itoa(offset))

		var page []bulkExportListAPI
		if err := c.Get(ctx, "/api/v1/bulk-exports", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing bulk exports", err.Error())
			return
		}
		all = append(all, page...)
		if len(page) < bulkExportsPageSize {
			break
		}
	}

	elems := make([]attr.Value, 0, len(all))
	for _, e := range all {
		exportFields := types.ListNull(types.StringType)
		if len(e.ExportFields) > 0 {
			fieldElems := make([]attr.Value, 0, len(e.ExportFields))
			for _, f := range e.ExportFields {
				fieldElems = append(fieldElems, types.StringValue(f))
			}
			list, diags := types.ListValue(types.StringType, fieldElems)
			resp.Diagnostics.Append(diags...)
			exportFields = list
		}

		allExperiments := types.BoolNull()
		if e.AllExperiments != nil {
			allExperiments = types.BoolValue(*e.AllExperiments)
		}
		intervalHours := types.Int64Null()
		if e.IntervalHours != nil {
			intervalHours = types.Int64Value(*e.IntervalHours)
		}

		obj, diags := types.ObjectValue(bulkExportObjectType.AttrTypes, map[string]attr.Value{
			"id":                         types.StringValue(e.ID),
			"bulk_export_destination_id": types.StringValue(e.BulkExportDestinationID),
			"session_id":                 bulkExportStringValue(e.SessionID),
			"all_experiments":            allExperiments,
			"source_bulk_export_id":      bulkExportStringValue(e.SourceBulkExportID),
			"start_time":                 types.StringValue(e.StartTime),
			"end_time":                   bulkExportStringValue(e.EndTime),
			"filter":                     bulkExportStringValue(e.Filter),
			"format":                     types.StringValue(e.Format),
			"format_version":             types.StringValue(e.FormatVersion),
			"compression":                types.StringValue(e.Compression),
			"interval_hours":             intervalHours,
			"export_fields":              exportFields,
			"status":                     types.StringValue(e.Status),
			"workspace_id":               types.StringValue(firstNonEmpty(e.WorkspaceID, e.TenantID)),
			"created_at":                 types.StringValue(e.CreatedAt),
			"updated_at":                 types.StringValue(e.UpdatedAt),
			"finished_at":                bulkExportStringValue(e.FinishedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(bulkExportObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.BulkExports = list

	// The listing has no single workspace field of its own; fall back to the
	// workspace the client is operating in so workspace_id is never unknown.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read bulk exports data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// bulkExportStringValue converts an optional API string into a types.String,
// mapping nil to null.
func bulkExportStringValue(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

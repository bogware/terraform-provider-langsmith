// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &BulkExportDataSource{}

// NewBulkExportDataSource returns a new BulkExportDataSource for looking up
// an existing bulk export job by ID.
func NewBulkExportDataSource() datasource.DataSource {
	return &BulkExportDataSource{}
}

// BulkExportDataSource reads a LangSmith bulk export job by ID.
type BulkExportDataSource struct {
	client *client.Client
}

// BulkExportDataSourceModel holds the read-only attributes for a bulk export lookup.
type BulkExportDataSourceModel struct {
	ID                      types.String `tfsdk:"id"`
	BulkExportDestinationID types.String `tfsdk:"bulk_export_destination_id"`
	SessionID               types.String `tfsdk:"session_id"`
	StartTime               types.String `tfsdk:"start_time"`
	EndTime                 types.String `tfsdk:"end_time"`
	Format                  types.String `tfsdk:"format"`
	Compression             types.String `tfsdk:"compression"`
	IntervalHours           types.Int64  `tfsdk:"interval_hours"`
	Filter                  types.String `tfsdk:"filter"`
	Status                  types.String `tfsdk:"status"`
	WorkspaceID             types.String `tfsdk:"workspace_id"`
	TenantID                types.String `tfsdk:"tenant_id"`
	CreatedAt               types.String `tfsdk:"created_at"`
	UpdatedAt               types.String `tfsdk:"updated_at"`
	FormatVersion           types.String `tfsdk:"format_version"`
	ExportFields            types.List   `tfsdk:"export_fields"`
	FinishedAt              types.String `tfsdk:"finished_at"`
}

// bulkExportDataSourceAPIResponse is the wire format for a bulk export. The
// API documents tenant_id; workspace_id is decoded as well in case the server
// returns it as an alias.
type bulkExportDataSourceAPIResponse struct {
	ID                      string   `json:"id"`
	BulkExportDestinationID string   `json:"bulk_export_destination_id"`
	SessionID               string   `json:"session_id"`
	StartTime               string   `json:"start_time"`
	EndTime                 *string  `json:"end_time"`
	Format                  string   `json:"format"`
	Compression             string   `json:"compression"`
	IntervalHours           *int64   `json:"interval_hours"`
	Filter                  *string  `json:"filter"`
	Status                  string   `json:"status"`
	TenantID                string   `json:"tenant_id"`
	WorkspaceID             string   `json:"workspace_id"`
	CreatedAt               string   `json:"created_at"`
	UpdatedAt               string   `json:"updated_at"`
	FormatVersion           string   `json:"format_version"`
	ExportFields            []string `json:"export_fields"`
	FinishedAt              *string  `json:"finished_at"`
}

func (d *BulkExportDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bulk_export"
}

func (d *BulkExportDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith bulk export by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the bulk export.",
				Required:            true,
			},
			"bulk_export_destination_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the bulk export destination.",
				Computed:            true,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the project/session being exported.",
				Computed:            true,
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The start time for the export in RFC3339 format.",
				Computed:            true,
			},
			"end_time": schema.StringAttribute{
				MarkdownDescription: "The end time for the export in RFC3339 format.",
				Computed:            true,
			},
			"format": schema.StringAttribute{
				MarkdownDescription: "The export format.",
				Computed:            true,
			},
			"compression": schema.StringAttribute{
				MarkdownDescription: "The compression type.",
				Computed:            true,
			},
			"interval_hours": schema.Int64Attribute{
				MarkdownDescription: "The interval in hours for recurring exports.",
				Computed:            true,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "A filter expression for the export.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The status of the bulk export.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Deprecated: use `workspace_id` instead.",
				DeprecationMessage:  "Use workspace_id instead. This attribute will be removed in a future release.",
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
			"format_version": schema.StringAttribute{
				MarkdownDescription: "The format version (`v1` or `v2_beta`).",
				Computed:            true,
			},
			"export_fields": schema.ListAttribute{
				MarkdownDescription: "List of fields included in the export.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"finished_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the export finished.",
				Computed:            true,
			},
		},
	}
}

func (d *BulkExportDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BulkExportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BulkExportDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result bulkExportDataSourceAPIResponse
	err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/bulk-exports/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"Bulk Export Not Found",
				fmt.Sprintf("No bulk export found with ID %q.", data.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading bulk export", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.BulkExportDestinationID = types.StringValue(result.BulkExportDestinationID)
	data.SessionID = types.StringValue(result.SessionID)
	data.StartTime = types.StringValue(result.StartTime)

	if result.EndTime != nil {
		data.EndTime = types.StringValue(*result.EndTime)
	} else {
		data.EndTime = types.StringNull()
	}

	data.Format = types.StringValue(result.Format)
	data.Compression = types.StringValue(result.Compression)

	if result.IntervalHours != nil {
		data.IntervalHours = types.Int64Value(*result.IntervalHours)
	} else {
		data.IntervalHours = types.Int64Null()
	}

	if result.Filter != nil {
		data.Filter = types.StringValue(*result.Filter)
	} else {
		data.Filter = types.StringNull()
	}

	data.Status = types.StringValue(result.Status)

	apiWorkspaceID := result.WorkspaceID
	if apiWorkspaceID == "" {
		apiWorkspaceID = result.TenantID
	}
	reconcileWorkspaceID(&data.WorkspaceID, apiWorkspaceID, &resp.Diagnostics)
	data.TenantID = data.WorkspaceID

	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
	data.FormatVersion = types.StringValue(result.FormatVersion)

	if len(result.ExportFields) > 0 {
		elems := make([]attr.Value, 0, len(result.ExportFields))
		for _, s := range result.ExportFields {
			elems = append(elems, types.StringValue(s))
		}
		list, diags := types.ListValue(types.StringType, elems)
		resp.Diagnostics.Append(diags...)
		data.ExportFields = list
	} else {
		data.ExportFields = types.ListNull(types.StringType)
	}

	if result.FinishedAt != nil {
		data.FinishedAt = types.StringValue(*result.FinishedAt)
	} else {
		data.FinishedAt = types.StringNull()
	}

	tflog.Trace(ctx, "read bulk export data source", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

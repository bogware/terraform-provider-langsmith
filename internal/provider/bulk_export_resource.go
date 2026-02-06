// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &BulkExportResource{}
	_ resource.ResourceWithImportState = &BulkExportResource{}
)

// NewBulkExportResource returns a new BulkExportResource.
func NewBulkExportResource() resource.Resource {
	return &BulkExportResource{}
}

// BulkExportResource defines the resource implementation.
type BulkExportResource struct {
	client *client.Client
}

// BulkExportResourceModel describes the resource data model.
type BulkExportResourceModel struct {
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
	TenantID                types.String `tfsdk:"tenant_id"`
	CreatedAt               types.String `tfsdk:"created_at"`
	UpdatedAt               types.String `tfsdk:"updated_at"`
}

// bulkExportAPICreateRequest is the request body for creating a bulk export.
type bulkExportAPICreateRequest struct {
	BulkExportDestinationID string  `json:"bulk_export_destination_id"`
	SessionID               string  `json:"session_id"`
	StartTime               string  `json:"start_time"`
	EndTime                 *string `json:"end_time,omitempty"`
	Format                  string  `json:"format,omitempty"`
	Compression             string  `json:"compression,omitempty"`
	IntervalHours           *int64  `json:"interval_hours,omitempty"`
	Filter                  *string `json:"filter,omitempty"`
}

// bulkExportAPIUpdateRequest is the request body for updating a bulk export.
type bulkExportAPIUpdateRequest struct {
	Status string `json:"status"`
}

// bulkExportAPIResponse is the API response for a bulk export.
type bulkExportAPIResponse struct {
	ID                      string  `json:"id"`
	BulkExportDestinationID string  `json:"bulk_export_destination_id"`
	SessionID               string  `json:"session_id"`
	StartTime               string  `json:"start_time"`
	EndTime                 *string `json:"end_time"`
	Format                  string  `json:"format"`
	Compression             string  `json:"compression"`
	IntervalHours           *int64  `json:"interval_hours"`
	Filter                  *string `json:"filter"`
	Status                  string  `json:"status"`
	TenantID                string  `json:"tenant_id"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

func (r *BulkExportResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bulk_export"
}

func (r *BulkExportResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith bulk export. Deleting this resource cancels the bulk export.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the bulk export.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bulk_export_destination_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the bulk export destination.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the project/session to export.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The start time for the export in RFC3339 format.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"end_time": schema.StringAttribute{
				MarkdownDescription: "The end time for the export in RFC3339 format.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"format": schema.StringAttribute{
				MarkdownDescription: "The export format. Defaults to `Parquet`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Parquet"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"compression": schema.StringAttribute{
				MarkdownDescription: "The compression type. Defaults to `gzip`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("gzip"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"interval_hours": schema.Int64Attribute{
				MarkdownDescription: "The interval in hours for recurring exports.",
				Optional:            true,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "A filter expression for the export.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The status of the bulk export.",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID.",
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
		},
	}
}

func (r *BulkExportResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *BulkExportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BulkExportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := bulkExportAPICreateRequest{
		BulkExportDestinationID: data.BulkExportDestinationID.ValueString(),
		SessionID:               data.SessionID.ValueString(),
		StartTime:               data.StartTime.ValueString(),
		Format:                  data.Format.ValueString(),
		Compression:             data.Compression.ValueString(),
	}

	if !data.EndTime.IsNull() && !data.EndTime.IsUnknown() {
		v := data.EndTime.ValueString()
		body.EndTime = &v
	}
	if !data.IntervalHours.IsNull() && !data.IntervalHours.IsUnknown() {
		v := data.IntervalHours.ValueInt64()
		body.IntervalHours = &v
	}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		v := data.Filter.ValueString()
		body.Filter = &v
	}

	var result bulkExportAPIResponse
	err := r.client.Post(ctx, "/api/v1/bulk-exports", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating bulk export", err.Error())
		return
	}

	mapBulkExportResponseToState(&data, &result)
	tflog.Trace(ctx, "created bulk export resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BulkExportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BulkExportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result bulkExportAPIResponse
	err := r.client.Get(ctx, "/api/v1/bulk-exports/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading bulk export", err.Error())
		return
	}

	mapBulkExportResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BulkExportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BulkExportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := bulkExportAPIUpdateRequest{
		Status: "Cancelled",
	}

	var result bulkExportAPIResponse
	err := r.client.Patch(ctx, "/api/v1/bulk-exports/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating bulk export", err.Error())
		return
	}

	mapBulkExportResponseToState(&data, &result)
	tflog.Trace(ctx, "updated bulk export resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BulkExportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BulkExportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Cancel the bulk export instead of deleting it.
	body := bulkExportAPIUpdateRequest{
		Status: "Cancelled",
	}

	err := r.client.Patch(ctx, "/api/v1/bulk-exports/"+data.ID.ValueString(), body, nil)
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error cancelling bulk export", err.Error())
		return
	}

	tflog.Trace(ctx, "cancelled (deleted) bulk export resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *BulkExportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapBulkExportResponseToState maps an API response to the Terraform state model.
func mapBulkExportResponseToState(data *BulkExportResourceModel, result *bulkExportAPIResponse) {
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
	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

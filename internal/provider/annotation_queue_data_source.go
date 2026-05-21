// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &AnnotationQueueDataSource{}

func NewAnnotationQueueDataSource() datasource.DataSource {
	return &AnnotationQueueDataSource{}
}

type AnnotationQueueDataSource struct {
	client *client.Client
}

type AnnotationQueueDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	NumReviewersPerItem types.Int64  `tfsdk:"num_reviewers_per_item"`
	EnableReservations  types.Bool   `tfsdk:"enable_reservations"`
	ReservationMinutes  types.Int64  `tfsdk:"reservation_minutes"`
	WorkspaceID         types.String `tfsdk:"workspace_id"`
	TenantID            types.String `tfsdk:"tenant_id"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

type annotationQueueDataSourceAPIResponse struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	NumReviewersPerItem *int64  `json:"num_reviewers_per_item"`
	EnableReservations  bool    `json:"enable_reservations"`
	ReservationMinutes  *int64  `json:"reservation_minutes"`
	WorkspaceID         string  `json:"workspace_id"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func (d *AnnotationQueueDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation_queue"
}

func (d *AnnotationQueueDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith annotation queue by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the queue. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the queue. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the queue.",
				Computed:            true,
			},
			"num_reviewers_per_item": schema.Int64Attribute{
				MarkdownDescription: "The number of reviewers per item.",
				Computed:            true,
			},
			"enable_reservations": schema.BoolAttribute{
				MarkdownDescription: "Whether reservations are enabled.",
				Computed:            true,
			},
			"reservation_minutes": schema.Int64Attribute{
				MarkdownDescription: "The number of minutes a reservation is held.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Deprecated: use `workspace_id` instead. The workspace ID.",
				Computed:            true,
				DeprecationMessage:  "Use 'workspace_id' instead. This attribute will be removed in a future version.",
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

func (d *AnnotationQueueDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AnnotationQueueDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AnnotationQueueDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError("Missing Required Attribute", "Either \"id\" or \"name\" must be specified.")
		return
	}

	if idSet {
		var result annotationQueueDataSourceAPIResponse
		err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/annotation-queues/"+data.ID.ValueString(), nil, &result)
		if err != nil {
			resp.Diagnostics.AddError("Error reading annotation queue", err.Error())
			return
		}
		mapAnnotationQueueDSResponse(&data, &result, &resp.Diagnostics)
	} else {
		query := url.Values{}
		query.Set("name", data.Name.ValueString())
		var results []annotationQueueDataSourceAPIResponse
		err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/annotation-queues", query, &results)
		if err != nil {
			resp.Diagnostics.AddError("Error reading annotation queue", err.Error())
			return
		}
		if len(results) == 0 {
			resp.Diagnostics.AddError("Annotation Queue Not Found", fmt.Sprintf("No queue found with name %q.", data.Name.ValueString()))
			return
		}
		mapAnnotationQueueDSResponse(&data, &results[0], &resp.Diagnostics)
	}

	tflog.Trace(ctx, "read annotation queue data source", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapAnnotationQueueDSResponse(data *AnnotationQueueDataSourceModel, r *annotationQueueDataSourceAPIResponse, diags *diag.Diagnostics) {
	data.ID = types.StringValue(r.ID)
	data.Name = types.StringValue(r.Name)
	data.EnableReservations = types.BoolValue(r.EnableReservations)
	reconcileWorkspaceID(&data.WorkspaceID, r.WorkspaceID, diags)
	data.TenantID = data.WorkspaceID
	data.CreatedAt = types.StringValue(r.CreatedAt)
	data.UpdatedAt = types.StringValue(r.UpdatedAt)

	if r.Description != nil {
		data.Description = types.StringValue(*r.Description)
	} else {
		data.Description = types.StringNull()
	}
	if r.NumReviewersPerItem != nil {
		data.NumReviewersPerItem = types.Int64Value(*r.NumReviewersPerItem)
	} else {
		data.NumReviewersPerItem = types.Int64Null()
	}
	if r.ReservationMinutes != nil {
		data.ReservationMinutes = types.Int64Value(*r.ReservationMinutes)
	} else {
		data.ReservationMinutes = types.Int64Null()
	}
}

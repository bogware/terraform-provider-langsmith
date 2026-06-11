// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &WorkspaceStatsDataSource{}

func NewWorkspaceStatsDataSource() datasource.DataSource {
	return &WorkspaceStatsDataSource{}
}

type WorkspaceStatsDataSource struct {
	client *client.Client
}

type WorkspaceStatsDataSourceModel struct {
	TenantID             types.String `tfsdk:"tenant_id"`
	TagValueIDs          types.List   `tfsdk:"tag_value_ids"`
	DatasetCount         types.Int64  `tfsdk:"dataset_count"`
	TracerSessionCount   types.Int64  `tfsdk:"tracer_session_count"`
	RepoCount            types.Int64  `tfsdk:"repo_count"`
	AnnotationQueueCount types.Int64  `tfsdk:"annotation_queue_count"`
	DeploymentCount      types.Int64  `tfsdk:"deployment_count"`
	DashboardsCount      types.Int64  `tfsdk:"dashboards_count"`
	EvaluatorCount       types.Int64  `tfsdk:"evaluator_count"`
	WorkspaceID          types.String `tfsdk:"workspace_id"`
}

// workspaceStatsAPIResponse mirrors the TenantStats schema.
type workspaceStatsAPIResponse struct {
	TenantID             string `json:"tenant_id"`
	DatasetCount         int64  `json:"dataset_count"`
	TracerSessionCount   int64  `json:"tracer_session_count"`
	RepoCount            int64  `json:"repo_count"`
	AnnotationQueueCount int64  `json:"annotation_queue_count"`
	DeploymentCount      int64  `json:"deployment_count"`
	DashboardsCount      int64  `json:"dashboards_count"`
	EvaluatorCount       int64  `json:"evaluator_count"`
}

func (d *WorkspaceStatsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_stats"
}

func (d *WorkspaceStatsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns object counts (datasets, tracing projects, prompt repos, etc.) for the current LangSmith workspace.",
		Attributes: map[string]schema.Attribute{
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the workspace (tenant) the stats belong to.",
				Computed:            true,
			},
			"tag_value_ids": schema.ListAttribute{
				MarkdownDescription: "Optional list of tag value UUIDs to filter the counts to resources carrying those tags.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"dataset_count": schema.Int64Attribute{
				MarkdownDescription: "Number of datasets in the workspace.",
				Computed:            true,
			},
			"tracer_session_count": schema.Int64Attribute{
				MarkdownDescription: "Number of tracing projects (tracer sessions) in the workspace.",
				Computed:            true,
			},
			"repo_count": schema.Int64Attribute{
				MarkdownDescription: "Number of prompt repos in the workspace.",
				Computed:            true,
			},
			"annotation_queue_count": schema.Int64Attribute{
				MarkdownDescription: "Number of annotation queues in the workspace.",
				Computed:            true,
			},
			"deployment_count": schema.Int64Attribute{
				MarkdownDescription: "Number of deployments in the workspace.",
				Computed:            true,
			},
			"dashboards_count": schema.Int64Attribute{
				MarkdownDescription: "Number of dashboards in the workspace.",
				Computed:            true,
			},
			"evaluator_count": schema.Int64Attribute{
				MarkdownDescription: "Number of evaluators in the workspace.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *WorkspaceStatsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspaceStatsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceStatsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if !data.TagValueIDs.IsNull() && !data.TagValueIDs.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.TagValueIDs.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, id := range ids {
			query.Add("tag_value_id", id)
		}
	}

	var result workspaceStatsAPIResponse
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/workspaces/current/stats", query, &result); err != nil {
		resp.Diagnostics.AddError("Error reading workspace stats", err.Error())
		return
	}

	data.TenantID = types.StringValue(result.TenantID)
	data.DatasetCount = types.Int64Value(result.DatasetCount)
	data.TracerSessionCount = types.Int64Value(result.TracerSessionCount)
	data.RepoCount = types.Int64Value(result.RepoCount)
	data.AnnotationQueueCount = types.Int64Value(result.AnnotationQueueCount)
	data.DeploymentCount = types.Int64Value(result.DeploymentCount)
	data.DashboardsCount = types.Int64Value(result.DashboardsCount)
	data.EvaluatorCount = types.Int64Value(result.EvaluatorCount)

	tflog.Trace(ctx, "read workspace stats data source", map[string]interface{}{"tenant_id": result.TenantID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

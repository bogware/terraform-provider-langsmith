// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &OptimizationJobLogsDataSource{}

// NewOptimizationJobLogsDataSource returns a data source reading the log stream
// of a prompt optimization job.
func NewOptimizationJobLogsDataSource() datasource.DataSource {
	return &OptimizationJobLogsDataSource{}
}

// OptimizationJobLogsDataSource reads
// GET /api/v1/repos/{owner}/{repo}/optimization-jobs/{job_id}/logs.
type OptimizationJobLogsDataSource struct {
	client *client.Client
}

// OptimizationJobLogsDataSourceModel maps the Terraform schema.
type OptimizationJobLogsDataSourceModel struct {
	Owner       types.String            `tfsdk:"owner"`
	Repo        types.String            `tfsdk:"repo"`
	JobID       types.String            `tfsdk:"job_id"`
	WorkspaceID types.String            `tfsdk:"workspace_id"`
	Logs        []optimizationJobLogRow `tfsdk:"logs"`
}

type optimizationJobLogRow struct {
	ID        types.String `tfsdk:"id"`
	LogType   types.String `tfsdk:"log_type"`
	Message   types.String `tfsdk:"message"`
	Data      types.String `tfsdk:"data"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type optimizationJobLogAPI struct {
	ID        string          `json:"id"`
	JobID     string          `json:"job_id"`
	LogType   string          `json:"log_type"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	CreatedAt string          `json:"created_at"`
}

func (d *OptimizationJobLogsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_optimization_job_logs"
}

func (d *OptimizationJobLogsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the log stream of a prompt optimization job — progress messages, results and errors as the job runs.\n\n" +
			"This is a snapshot taken when Terraform reads the data source, not a live tail: a job still running will report more later.",
		Attributes: map[string]schema.Attribute{
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the hub repo (`-` for the current workspace).",
				Required:            true,
			},
			"repo": schema.StringAttribute{
				MarkdownDescription: "Handle of the hub repo.",
				Required:            true,
			},
			"job_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the optimization job.",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"logs": schema.ListNestedAttribute{
				MarkdownDescription: "The job's log entries.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "UUID of the log entry.",
							Computed:            true,
						},
						"log_type": schema.StringAttribute{
							MarkdownDescription: "Entry type: `info`, `result`, `error` or `link`.",
							Computed:            true,
						},
						"message": schema.StringAttribute{
							MarkdownDescription: "Log message.",
							Computed:            true,
						},
						"data": schema.StringAttribute{
							MarkdownDescription: "JSON-encoded structured payload attached to the entry, when present.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "When the entry was written.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *OptimizationJobLogsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *OptimizationJobLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OptimizationJobLogsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)
	logPath := fmt.Sprintf("/api/v1/repos/%s/%s/optimization-jobs/%s/logs",
		data.Owner.ValueString(), data.Repo.ValueString(), data.JobID.ValueString())

	var listResp []optimizationJobLogAPI
	if err := c.Get(ctx, logPath, nil, &listResp); err != nil {
		resp.Diagnostics.AddError("Error reading optimization job logs", err.Error())
		return
	}

	data.Logs = make([]optimizationJobLogRow, 0, len(listResp))
	for _, l := range listResp {
		data.Logs = append(data.Logs, optimizationJobLogRow{
			ID:        types.StringValue(l.ID),
			LogType:   types.StringValue(l.LogType),
			Message:   types.StringValue(l.Message),
			Data:      jsonStringValue(l.Data),
			CreatedAt: types.StringValue(l.CreatedAt),
		})
	}

	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read optimization job logs", map[string]interface{}{"count": len(data.Logs)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &AuditLogDataSource{}

func NewAuditLogDataSource() datasource.DataSource {
	return &AuditLogDataSource{}
}

type AuditLogDataSource struct {
	client *client.Client
}

type AuditLogDataSourceModel struct {
	StartTime   types.String `tfsdk:"start_time"`
	EndTime     types.String `tfsdk:"end_time"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Operations  types.List   `tfsdk:"operations"`
	Limit       types.Int64  `tfsdk:"limit"`
	Cursor      types.String `tfsdk:"cursor"`
	NextCursor  types.String `tfsdk:"next_cursor"`
	Items       types.List   `tfsdk:"items"`
}

type auditLogResponse struct {
	Cursor *string           `json:"cursor"`
	Items  []json.RawMessage `json:"items"`
}

func (d *AuditLogDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_audit_log"
}

func (d *AuditLogDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves a page of LangSmith audit log entries in OCSF format. `start_time` and `end_time` are required (ISO 8601). Each entry is surfaced as a JSON-encoded string in `items` because the OCSF payload is large and heterogeneous.",
		Attributes: map[string]schema.Attribute{
			"start_time":   schema.StringAttribute{Required: true, MarkdownDescription: "ISO 8601 start timestamp (inclusive)."},
			"end_time":     schema.StringAttribute{Required: true, MarkdownDescription: "ISO 8601 end timestamp (exclusive)."},
			"workspace_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter to a single workspace UUID."},
			"operations": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter to specific operation names.",
			},
			"limit":       schema.Int64Attribute{Optional: true},
			"cursor":      schema.StringAttribute{Optional: true, MarkdownDescription: "Pagination cursor returned from a prior call."},
			"next_cursor": schema.StringAttribute{Computed: true, MarkdownDescription: "Cursor for the next page, or null when there are no more results."},
			"items": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "JSON-encoded audit log entries (one OCSF activity per element).",
			},
		},
	}
}

func (d *AuditLogDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AuditLogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AuditLogDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	q.Set("start_time", data.StartTime.ValueString())
	q.Set("end_time", data.EndTime.ValueString())
	if !data.WorkspaceID.IsNull() && !data.WorkspaceID.IsUnknown() {
		q.Set("workspace_id", data.WorkspaceID.ValueString())
	}
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		q.Set("limit", fmt.Sprintf("%d", data.Limit.ValueInt64()))
	}
	if !data.Cursor.IsNull() && !data.Cursor.IsUnknown() {
		q.Set("cursor", data.Cursor.ValueString())
	}
	if !data.Operations.IsNull() && !data.Operations.IsUnknown() {
		var ops []string
		resp.Diagnostics.Append(data.Operations.ElementsAs(ctx, &ops, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, op := range ops {
			q.Add("operations", op)
		}
	}

	var api auditLogResponse
	if err := d.client.Get(ctx, "/api/v1/audit-logs", q, &api); err != nil {
		resp.Diagnostics.AddError("Error reading audit logs", err.Error())
		return
	}
	if api.Cursor != nil {
		data.NextCursor = types.StringValue(*api.Cursor)
	} else {
		data.NextCursor = types.StringNull()
	}
	elems := make([]attr.Value, 0, len(api.Items))
	for _, raw := range api.Items {
		elems = append(elems, types.StringValue(string(raw)))
	}
	list, diags := types.ListValue(types.StringType, elems)
	resp.Diagnostics.Append(diags...)
	data.Items = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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
		MarkdownDescription: "Retrieves a page of LangSmith audit log entries via GET `/api/v1/audit-logs` (OCSF API Activity). `start_time` and `end_time` are required (ISO 8601). Each entry is a normalized JSON string in `items`. " +
			"Set `organization_id` on the provider or `LANGSMITH_ORGANIZATION_ID` when using an organization-scoped API key.",
		Attributes: map[string]schema.Attribute{
			"start_time":   schema.StringAttribute{Required: true, MarkdownDescription: "ISO 8601 start timestamp (inclusive)."},
			"end_time":     schema.StringAttribute{Required: true, MarkdownDescription: "ISO 8601 end timestamp (exclusive)."},
			"workspace_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter to a single workspace UUID."},
			"operations": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "When non-empty, each element is sent as a repeated `operations` query parameter (OpenAPI `AuditLogOperation` values), for example `create_api_key`.",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of events to return (1–100). When unset, the API default applies.",
			},
			"cursor": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Opaque pagination cursor from a previous response (`next_cursor`) to fetch the next page.",
			},
			"next_cursor": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cursor returned by the API for the following page, if any.",
			},
			"items": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Audit events from the `items` array; each string is normalized JSON (OCSF API Activity). " +
					"Parse in your stack or use `jsondecode()` in Terraform expressions.",
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

	if data.StartTime.IsNull() || data.StartTime.IsUnknown() || data.EndTime.IsNull() || data.EndTime.IsUnknown() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"The attributes \"start_time\" and \"end_time\" must be specified.",
		)
		return
	}

	start := strings.TrimSpace(data.StartTime.ValueString())
	end := strings.TrimSpace(data.EndTime.ValueString())
	if start == "" || end == "" {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"The attributes \"start_time\" and \"end_time\" must be non-empty ISO 8601 timestamps.",
		)
		return
	}

	q := url.Values{}
	q.Set("start_time", start)
	q.Set("end_time", end)

	workspaceID := ""
	if !data.WorkspaceID.IsNull() && !data.WorkspaceID.IsUnknown() {
		workspaceID = strings.TrimSpace(data.WorkspaceID.ValueString())
	}
	if workspaceID != "" {
		q.Set("workspace_id", workspaceID)
	}

	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		limit := data.Limit.ValueInt64()
		if limit < 1 || limit > 100 {
			resp.Diagnostics.AddError("Invalid limit", "limit must be between 1 and 100 when set.")
			return
		}
		q.Set("limit", fmt.Sprintf("%d", limit))
	}

	if !data.Cursor.IsNull() && !data.Cursor.IsUnknown() {
		if cursor := strings.TrimSpace(data.Cursor.ValueString()); cursor != "" {
			q.Set("cursor", cursor)
		}
	}

	if !data.Operations.IsNull() && !data.Operations.IsUnknown() {
		var ops []string
		resp.Diagnostics.Append(data.Operations.ElementsAs(ctx, &ops, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, op := range ops {
			op = strings.TrimSpace(op)
			if op == "" {
				resp.Diagnostics.AddError("Invalid operations", "operations entries must be non-empty strings.")
				return
			}
			q.Add("operations", op)
		}
	}

	var api auditLogResponse
	if err := d.client.Get(ctx, "/api/v1/audit-logs", q, &api); err != nil {
		resp.Diagnostics.AddError("Error reading audit logs", err.Error())
		return
	}
	data.NextCursor = types.StringPointerValue(api.Cursor)

	elems := make([]attr.Value, 0, len(api.Items))
	for _, raw := range api.Items {
		elems = append(elems, jsonStringValue(raw))
	}
	list, diags := types.ListValue(types.StringType, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Items = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

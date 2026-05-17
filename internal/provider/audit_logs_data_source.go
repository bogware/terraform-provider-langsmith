// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &AuditLogsDataSource{}

// NewAuditLogsDataSource returns a data source backed by GET /api/v1/audit-logs.
func NewAuditLogsDataSource() datasource.DataSource {
	return &AuditLogsDataSource{}
}

// AuditLogsDataSource lists LangSmith audit log events (OCSF API Activity).
type AuditLogsDataSource struct {
	client *client.Client
}

// AuditLogsDataSourceModel is Terraform state for the audit log listing.
type AuditLogsDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	StartTime   types.String `tfsdk:"start_time"`
	EndTime     types.String `tfsdk:"end_time"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Operations  types.List   `tfsdk:"operations"`
	Limit       types.Int64  `tfsdk:"limit"`
	Cursor      types.String `tfsdk:"cursor"`
	NextCursor  types.String `tfsdk:"next_cursor"`
	Items       types.List   `tfsdk:"items"`
}

type listAuditLogsAPIResponse struct {
	Cursor *string           `json:"cursor"`
	Items  []json.RawMessage `json:"items"`
}

func (d *AuditLogsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_audit_logs"
}

func (d *AuditLogsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists **audit log events** for the authenticated organization via GET `/api/v1/audit-logs`. " +
			"Each returned element is a JSON object in [OCSF v1.7.0 API Activity](https://schema.ocsf.io/1.7.0/classes/api_activity) form (Class UID 6003), as described in LangSmith docs. " +
			"Use `operations` to filter by LangSmith operation names (for example `create_api_key`); actor details live inside each event (for example `actor.user.uid`, `api.operation`).\n\n" +
			"**Access requirements** (see [LangSmith audit logs](https://docs.langchain.com/langsmith/audit-logs)): Enterprise plan, and Organization Admin or Organization Operator role (`organization:manage`). " +
			"The API example sends `X-Organization-Id`; set `organization_id` on the provider or `LANGSMITH_ORGANIZATION_ID` when your API key is organization-scoped.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable hash of the query inputs so Terraform can track this data source instance.",
			},
			"start_time": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Start of the window (inclusive), ISO 8601 / RFC3339 (required query parameter on the LangSmith API).",
			},
			"end_time": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "End of the window (inclusive), ISO 8601 / RFC3339 (required query parameter on the LangSmith API).",
			},
			"workspace_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "When set, sent as `workspace_id` to restrict results to a workspace UUID.",
			},
			"operations": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "When non-empty, each element is sent as a repeated `operations` query parameter (OpenAPI `AuditLogOperation` values) to filter by action type.",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of events to return (1–100). When unset, the API default applies (OpenAPI default 10).",
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

func (d *AuditLogsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func auditLogsDataSourceID(start, end, workspaceID string, operations []string, limit *int64, cursor string) string {
	var b strings.Builder
	b.WriteString(start)
	b.WriteByte('\n')
	b.WriteString(end)
	b.WriteByte('\n')
	b.WriteString(workspaceID)
	b.WriteByte('\n')
	sort.Strings(operations)
	b.WriteString(strings.Join(operations, ","))
	b.WriteByte('\n')
	if limit != nil {
		fmt.Fprintf(&b, "%d", *limit)
	} else {
		b.WriteString("default")
	}
	b.WriteByte('\n')
	b.WriteString(cursor)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func (d *AuditLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AuditLogsDataSourceModel
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
		resp.Diagnostics.AddError("Missing Required Attribute", "The attributes \"start_time\" and \"end_time\" must be non-empty ISO 8601 timestamps.")
		return
	}

	q := url.Values{}
	q.Set("start_time", start)
	q.Set("end_time", end)

	var ops []string
	if !data.Operations.IsNull() && !data.Operations.IsUnknown() {
		resp.Diagnostics.Append(data.Operations.ElementsAs(ctx, &ops, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	for _, op := range ops {
		op = strings.TrimSpace(op)
		if op == "" {
			resp.Diagnostics.AddError("Invalid operations", "operations entries must be non-empty strings.")
			return
		}
		q.Add("operations", op)
	}

	var limitPtr *int64
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		limit := data.Limit.ValueInt64()
		if limit < 1 || limit > 100 {
			resp.Diagnostics.AddError("Invalid limit", "limit must be between 1 and 100 when set.")
			return
		}
		q.Set("limit", fmt.Sprintf("%d", limit))
		limitPtr = &limit
	}

	workspaceID := ""
	if !data.WorkspaceID.IsNull() && !data.WorkspaceID.IsUnknown() {
		workspaceID = strings.TrimSpace(data.WorkspaceID.ValueString())
	}
	if workspaceID != "" {
		q.Set("workspace_id", workspaceID)
	}

	cursorIn := ""
	if !data.Cursor.IsNull() && !data.Cursor.IsUnknown() {
		cursorIn = strings.TrimSpace(data.Cursor.ValueString())
	}
	if cursorIn != "" {
		q.Set("cursor", cursorIn)
	}

	var apiResp listAuditLogsAPIResponse
	if err := d.client.Get(ctx, "/api/v1/audit-logs", q, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error reading audit logs", err.Error())
		return
	}

	itemElems := make([]attr.Value, 0, len(apiResp.Items))
	for _, raw := range apiResp.Items {
		itemElems = append(itemElems, jsonStringValue(raw))
	}
	itemsList, listDiags := types.ListValue(types.StringType, itemElems)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Items = itemsList
	data.NextCursor = types.StringPointerValue(apiResp.Cursor)
	data.ID = types.StringValue(auditLogsDataSourceID(start, end, workspaceID, ops, limitPtr, cursorIn))

	tflog.Trace(ctx, "read audit logs data source", map[string]interface{}{
		"count": len(apiResp.Items),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

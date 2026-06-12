// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &IssuesDataSource{}

func NewIssuesDataSource() datasource.DataSource {
	return &IssuesDataSource{}
}

type IssuesDataSource struct {
	client *client.Client
}

type IssuesDataSourceModel struct {
	SessionID   types.String `tfsdk:"session_id"`
	Issues      types.List   `tfsdk:"issues"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

func (d *IssuesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_issues"
}

func (d *IssuesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "**Beta:** Lists issues detected in the current LangSmith workspace. The issue object shape is not yet stable, so each issue is returned as a JSON-encoded string.",
		Attributes: map[string]schema.Attribute{
			"session_id": schema.StringAttribute{
				MarkdownDescription: "**Beta:** If set, filters issues to the given tracing project (session) UUID.",
				Optional:            true,
			},
			"issues": schema.ListAttribute{
				MarkdownDescription: "**Beta:** List of issues; each element is a JSON-encoded issue object as returned by the API.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "**Beta:** If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
		},
	}
}

func (d *IssuesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IssuesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IssuesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if !data.SessionID.IsNull() && !data.SessionID.IsUnknown() {
		query.Set("session_id", data.SessionID.ValueString())
	}

	var raw json.RawMessage
	if err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/v1/platform/issues", query, &raw); err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Issues Not Found", "The issues endpoint returned 404. It is a beta endpoint and may not be available on this LangSmith deployment.")
			return
		}
		resp.Diagnostics.AddError("Error listing issues", err.Error())
		return
	}

	issues, err := decodeIssuesList(raw)
	if err != nil {
		resp.Diagnostics.AddError("Error decoding issues response", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(issues))
	for _, issue := range issues {
		elems = append(elems, types.StringValue(normalizeJSON(string(issue))))
	}
	list, diags := types.ListValue(types.StringType, elems)
	resp.Diagnostics.Append(diags...)
	data.Issues = list

	tflog.Trace(ctx, "read issues data source", map[string]interface{}{"count": len(issues)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// decodeIssuesList accepts either a bare JSON array of issues or an object
// wrapping the array under an "issues" key, since the beta endpoint's
// envelope is not yet stable.
func decodeIssuesList(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var issues []json.RawMessage
		if err := json.Unmarshal(trimmed, &issues); err != nil {
			return nil, err
		}
		return issues, nil
	}
	var wrapper struct {
		Issues []json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Issues, nil
}

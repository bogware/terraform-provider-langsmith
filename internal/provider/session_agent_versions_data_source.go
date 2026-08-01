// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &SessionAgentVersionsDataSource{}

// NewSessionAgentVersionsDataSource returns a data source listing the agent
// versions observed in a tracing project.
func NewSessionAgentVersionsDataSource() datasource.DataSource {
	return &SessionAgentVersionsDataSource{}
}

// SessionAgentVersionsDataSource reads
// GET /api/v1/platform/sessions/{session_id}/agent-versions.
type SessionAgentVersionsDataSource struct {
	client *client.Client
}

// SessionAgentVersionsDataSourceModel maps the Terraform schema.
type SessionAgentVersionsDataSourceModel struct {
	SessionID   types.String              `tfsdk:"session_id"`
	WorkspaceID types.String              `tfsdk:"workspace_id"`
	Versions    []sessionAgentVersionItem `tfsdk:"versions"`
}

type sessionAgentVersionItem struct {
	CommitSHA   types.String `tfsdk:"commit_sha"`
	FirstSeenAt types.String `tfsdk:"first_seen_at"`
}

type sessionAgentVersionAPI struct {
	CommitSHA   string `json:"commit_sha"`
	FirstSeenAt string `json:"first_seen_at"`
}

func (d *SessionAgentVersionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_session_agent_versions"
}

func (d *SessionAgentVersionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the agent versions LangSmith has observed sending traces to a project, identified by the commit each version was built from. Useful for correlating a change in behaviour with the deploy that introduced it.",
		Attributes: map[string]schema.Attribute{
			"session_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the tracing project (session) to inspect.",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"versions": schema.ListNestedAttribute{
				MarkdownDescription: "The agent versions seen in this project.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"commit_sha": schema.StringAttribute{
							MarkdownDescription: "Commit the agent version was built from.",
							Computed:            true,
						},
						"first_seen_at": schema.StringAttribute{
							MarkdownDescription: "When this version first sent a trace to the project.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *SessionAgentVersionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SessionAgentVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SessionAgentVersionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var listResp []sessionAgentVersionAPI
	if err := c.Get(ctx, "/api/v1/platform/sessions/"+data.SessionID.ValueString()+"/agent-versions", nil, &listResp); err != nil {
		resp.Diagnostics.AddError("Error listing agent versions", err.Error())
		return
	}

	data.Versions = make([]sessionAgentVersionItem, 0, len(listResp))
	for _, v := range listResp {
		data.Versions = append(data.Versions, sessionAgentVersionItem{
			CommitSHA:   types.StringValue(v.CommitSHA),
			FirstSeenAt: types.StringValue(v.FirstSeenAt),
		})
	}

	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read session agent versions", map[string]interface{}{"count": len(data.Versions)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

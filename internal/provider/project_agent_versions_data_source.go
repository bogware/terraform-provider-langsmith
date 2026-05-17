// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &ProjectAgentVersionsDataSource{}

// NewProjectAgentVersionsDataSource returns a data source that lists agent
// versions for a LangSmith project (tracer session) via the platform API.
func NewProjectAgentVersionsDataSource() datasource.DataSource {
	return &ProjectAgentVersionsDataSource{}
}

// ProjectAgentVersionsDataSource reads GET /v1/platform/sessions/{sessionID}/agent-versions.
type ProjectAgentVersionsDataSource struct {
	client *client.Client
}

// ProjectAgentVersionsDataSourceModel is Terraform state for this data source.
type ProjectAgentVersionsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	SessionID     types.String `tfsdk:"session_id"`
	AgentVersions types.List   `tfsdk:"agent_versions"`
}

// agentVersionAPIItem matches components/schemas/tracer_sessions.AgentVersionResponse in OpenAPI.
type agentVersionAPIItem struct {
	CommitSHA   *string `json:"commit_sha"`
	FirstSeenAt *string `json:"first_seen_at"`
}

func agentVersionObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"commit_sha":    types.StringType,
		"first_seen_at": types.StringType,
	}
}

func (d *ProjectAgentVersionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_agent_versions"
}

func (d *ProjectAgentVersionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists **agent deployment versions** recorded for a LangSmith project (tracer session) using the platform API " +
			"(`GET /v1/platform/sessions/{sessionID}/agent-versions`). " +
			"Use the project UUID from `langsmith_project` as `session_id` for auditing or pinning agent revisions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The stable identifier for this data source, set to `session_id`.",
				Computed:            true,
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project (tracer session) UUID — the same value as `langsmith_project.id` and other `session_id` arguments in this provider.",
				Required:            true,
			},
			"agent_versions": schema.ListNestedAttribute{
				MarkdownDescription: "Versions observed for this session (`tracer_sessions.AgentVersionResponse` items from the API).",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"commit_sha": schema.StringAttribute{
							MarkdownDescription: "Commit SHA associated with a deployed agent revision.",
							Computed:            true,
						},
						"first_seen_at": schema.StringAttribute{
							MarkdownDescription: "RFC3339-style timestamp from the API for when this revision was first seen.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ProjectAgentVersionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProjectAgentVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectAgentVersionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.SessionID.IsNull() || data.SessionID.IsUnknown() {
		resp.Diagnostics.AddError("Missing Required Attribute", "The attribute \"session_id\" must be specified.")
		return
	}
	sessionID := data.SessionID.ValueString()
	apiPath := "/v1/platform/sessions/" + url.PathEscape(sessionID) + "/agent-versions"

	var apiItems []agentVersionAPIItem
	if err := d.client.Get(ctx, apiPath, nil, &apiItems); err != nil {
		resp.Diagnostics.AddError("Error reading session agent versions", err.Error())
		return
	}

	attrTypes := agentVersionObjectAttrTypes()
	objType := types.ObjectType{AttrTypes: attrTypes}
	elems := make([]attr.Value, 0, len(apiItems))
	for _, item := range apiItems {
		var commit, firstSeen attr.Value
		if item.CommitSHA != nil && *item.CommitSHA != "" {
			commit = types.StringValue(*item.CommitSHA)
		} else {
			commit = types.StringNull()
		}
		if item.FirstSeenAt != nil && *item.FirstSeenAt != "" {
			firstSeen = types.StringValue(*item.FirstSeenAt)
		} else {
			firstSeen = types.StringNull()
		}
		obj, diags := types.ObjectValue(attrTypes, map[string]attr.Value{
			"commit_sha":    commit,
			"first_seen_at": firstSeen,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		elems = append(elems, obj)
	}

	listVal, diags := types.ListValue(objType, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(sessionID)
	data.AgentVersions = listVal

	tflog.Trace(ctx, "read project agent versions data source", map[string]interface{}{"session_id": sessionID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

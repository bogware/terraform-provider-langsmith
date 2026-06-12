// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// projectsPageSize is the page size used when listing tracer sessions; the
// API caps `limit` at 100, so we page until a short page signals the end.
const projectsPageSize = 100

var _ datasource.DataSource = &ProjectsDataSource{}

// NewProjectsDataSource returns a new ProjectsDataSource that lists tracer
// projects (sessions) in a workspace, with optional name filters.
func NewProjectsDataSource() datasource.DataSource {
	return &ProjectsDataSource{}
}

// ProjectsDataSource lists LangSmith projects (TracerSessions). It pages
// through GET /api/v1/sessions until every matching project is collected.
type ProjectsDataSource struct {
	client *client.Client
}

// ProjectsDataSourceModel holds the filter inputs and the resulting projects list.
type ProjectsDataSourceModel struct {
	Name         types.String `tfsdk:"name"`
	NameContains types.String `tfsdk:"name_contains"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Projects     types.List   `tfsdk:"projects"`
}

// projectListAPI mirrors the TracerSession schema returned by
// GET /api/v1/sessions. The list response carries tenant_id; workspace_id is
// also decoded in case the API supplies it.
type projectListAPI struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	DefaultDatasetID   *string         `json:"default_dataset_id"`
	ReferenceDatasetID *string         `json:"reference_dataset_id"`
	Extra              json.RawMessage `json:"extra"`
	TraceTier          *string         `json:"trace_tier"`
	WorkspaceID        string          `json:"workspace_id"`
	TenantID           string          `json:"tenant_id"`
	StartTime          string          `json:"start_time"`
	RunCount           *int64          `json:"run_count"`
}

var projectObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                   types.StringType,
	"name":                 types.StringType,
	"description":          types.StringType,
	"default_dataset_id":   types.StringType,
	"reference_dataset_id": types.StringType,
	"extra":                types.StringType,
	"trace_tier":           types.StringType,
	"workspace_id":         types.StringType,
	"start_time":           types.StringType,
	"run_count":            types.Int64Type,
}}

func (d *ProjectsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *ProjectsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists LangSmith projects (tracer sessions) in a workspace, optionally filtered by name.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Return only the project with this exact name.",
				Optional:            true,
			},
			"name_contains": schema.StringAttribute{
				MarkdownDescription: "Return only projects whose name contains this substring.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list projects from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
			},
			"projects": schema.ListNestedAttribute{
				MarkdownDescription: "The matching projects.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the project.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the project.",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "A description of the project.",
							Computed:            true,
						},
						"default_dataset_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the default dataset for this project.",
							Computed:            true,
						},
						"reference_dataset_id": schema.StringAttribute{
							MarkdownDescription: "The UUID of the reference dataset for this project.",
							Computed:            true,
						},
						"extra": schema.StringAttribute{
							MarkdownDescription: "JSON string containing extra metadata for the project.",
							Computed:            true,
						},
						"trace_tier": schema.StringAttribute{
							MarkdownDescription: "The trace retention tier (`longlived` or `shortlived`).",
							Computed:            true,
						},
						"workspace_id": schema.StringAttribute{
							MarkdownDescription: "The workspace ID that owns the project.",
							Computed:            true,
						},
						"start_time": schema.StringAttribute{
							MarkdownDescription: "The start time of the project.",
							Computed:            true,
						},
						"run_count": schema.Int64Attribute{
							MarkdownDescription: "The number of runs in the project.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ProjectsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProjectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var all []projectListAPI
	for offset := 0; ; offset += projectsPageSize {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(projectsPageSize))
		query.Set("offset", strconv.Itoa(offset))
		if !data.Name.IsNull() && !data.Name.IsUnknown() {
			query.Set("name", data.Name.ValueString())
		}
		if !data.NameContains.IsNull() && !data.NameContains.IsUnknown() {
			query.Set("name_contains", data.NameContains.ValueString())
		}

		var page []projectListAPI
		if err := c.Get(ctx, "/api/v1/sessions", query, &page); err != nil {
			resp.Diagnostics.AddError("Error listing projects", err.Error())
			return
		}
		all = append(all, page...)
		if len(page) < projectsPageSize {
			break
		}
	}

	elems := make([]attr.Value, 0, len(all))
	for _, p := range all {
		description := types.StringNull()
		if p.Description != nil {
			description = types.StringValue(*p.Description)
		}
		defaultDatasetID := types.StringNull()
		if p.DefaultDatasetID != nil {
			defaultDatasetID = types.StringValue(*p.DefaultDatasetID)
		}
		referenceDatasetID := types.StringNull()
		if p.ReferenceDatasetID != nil {
			referenceDatasetID = types.StringValue(*p.ReferenceDatasetID)
		}
		traceTier := types.StringNull()
		if p.TraceTier != nil {
			traceTier = types.StringValue(*p.TraceTier)
		}
		workspaceID := p.WorkspaceID
		if workspaceID == "" {
			workspaceID = p.TenantID
		}
		runCount := types.Int64Null()
		if p.RunCount != nil {
			runCount = types.Int64Value(*p.RunCount)
		}
		obj, diags := types.ObjectValue(projectObjectType.AttrTypes, map[string]attr.Value{
			"id":                   types.StringValue(p.ID),
			"name":                 types.StringValue(p.Name),
			"description":          description,
			"default_dataset_id":   defaultDatasetID,
			"reference_dataset_id": referenceDatasetID,
			"extra":                jsonStringValue(p.Extra),
			"trace_tier":           traceTier,
			"workspace_id":         types.StringValue(workspaceID),
			"start_time":           types.StringValue(p.StartTime),
			"run_count":            runCount,
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(projectObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Projects = list

	tflog.Trace(ctx, "read projects data source", map[string]interface{}{"count": len(all)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

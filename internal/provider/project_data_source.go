// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &ProjectDataSource{}

// NewProjectDataSource returns a new ProjectDataSource for scouting out
// an existing LangSmith project by ID or name.
func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

// ProjectDataSource reads a LangSmith project (TracerSession) by ID or name.
// It is read-only -- purely a reconnaissance mission, no cattle get moved.
type ProjectDataSource struct {
	client *client.Client
}

// ProjectDataSourceModel holds the read-only attributes returned for a project:
// name, description, tenant, start time, and run count.
type ProjectDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	DefaultDatasetID   types.String `tfsdk:"default_dataset_id"`
	ReferenceDatasetID types.String `tfsdk:"reference_dataset_id"`
	Extra              types.String `tfsdk:"extra"`
	TraceTier          types.String `tfsdk:"trace_tier"`
	TenantID           types.String `tfsdk:"tenant_id"`
	StartTime          types.String `tfsdk:"start_time"`
	RunCount           types.Int64  `tfsdk:"run_count"`
}

// projectDataSourceAPIResponse is the API response for a project lookup.
type projectDataSourceAPIResponse struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	DefaultDatasetID   *string         `json:"default_dataset_id"`
	ReferenceDatasetID *string         `json:"reference_dataset_id"`
	Extra              json.RawMessage `json:"extra"`
	TraceTier          *string         `json:"trace_tier"`
	TenantID           string          `json:"tenant_id"`
	StartTime          string          `json:"start_time"`
	RunCount           int64           `json:"run_count"`
}

func (d *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith project by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the project. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the project. Either `id` or `name` must be specified.",
				Optional:            true,
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
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID of the project.",
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
	}
}

func (d *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either \"id\" or \"name\" must be specified to look up a project.",
		)
		return
	}

	var result projectDataSourceAPIResponse

	if idSet {
		err := d.client.Get(ctx, "/api/v1/sessions/"+data.ID.ValueString(), nil, &result)
		if err != nil {
			resp.Diagnostics.AddError("Error reading project", err.Error())
			return
		}
	} else {
		query := url.Values{}
		query.Set("name", data.Name.ValueString())

		var results []projectDataSourceAPIResponse
		err := d.client.Get(ctx, "/api/v1/sessions", query, &results)
		if err != nil {
			resp.Diagnostics.AddError("Error reading project", err.Error())
			return
		}

		if len(results) == 0 {
			resp.Diagnostics.AddError(
				"Project Not Found",
				fmt.Sprintf("No project found with name %q.", data.Name.ValueString()),
			)
			return
		}

		result = results[0]
	}

	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	if result.DefaultDatasetID != nil {
		data.DefaultDatasetID = types.StringValue(*result.DefaultDatasetID)
	} else {
		data.DefaultDatasetID = types.StringNull()
	}

	if result.ReferenceDatasetID != nil {
		data.ReferenceDatasetID = types.StringValue(*result.ReferenceDatasetID)
	} else {
		data.ReferenceDatasetID = types.StringNull()
	}

	if len(result.Extra) > 0 && string(result.Extra) != "null" {
		data.Extra = types.StringValue(string(result.Extra))
	} else {
		data.Extra = types.StringNull()
	}

	if result.TraceTier != nil {
		data.TraceTier = types.StringValue(*result.TraceTier)
	} else {
		data.TraceTier = types.StringNull()
	}

	data.TenantID = types.StringValue(result.TenantID)
	data.StartTime = types.StringValue(result.StartTime)
	data.RunCount = types.Int64Value(result.RunCount)

	tflog.Trace(ctx, "read project data source", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

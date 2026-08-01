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

var _ datasource.DataSource = &DatasetVersionsDataSource{}

// NewDatasetVersionsDataSource returns a data source listing the version history
// of a dataset.
func NewDatasetVersionsDataSource() datasource.DataSource {
	return &DatasetVersionsDataSource{}
}

// DatasetVersionsDataSource reads GET /api/v1/datasets/{dataset_id}/versions.
type DatasetVersionsDataSource struct {
	client *client.Client
}

// DatasetVersionsDataSourceModel maps the Terraform schema for the data source.
type DatasetVersionsDataSourceModel struct {
	DatasetID   types.String               `tfsdk:"dataset_id"`
	WorkspaceID types.String               `tfsdk:"workspace_id"`
	Versions    []datasetVersionEntryModel `tfsdk:"versions"`
}

type datasetVersionEntryModel struct {
	AsOf types.String `tfsdk:"as_of"`
	Tags types.List   `tfsdk:"tags"`
}

// datasetVersionListEntry mirrors a single entry of the version list.
type datasetVersionListEntry struct {
	AsOf string   `json:"as_of"`
	Tags []string `json:"tags"`
}

func (d *DatasetVersionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_versions"
}

func (d *DatasetVersionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the version history of a LangSmith dataset. Every write to a dataset produces a version, identified by the timestamp it was taken `as_of`; a version can also carry tags applied with `langsmith_dataset_version_tag`. " +
			"Use this to discover the versions available before pinning an experiment or a tag to one.",
		Attributes: map[string]schema.Attribute{
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the dataset whose versions are listed.",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"versions": schema.ListNestedAttribute{
				MarkdownDescription: "The dataset's versions, newest first.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"as_of": schema.StringAttribute{
							MarkdownDescription: "Timestamp identifying the version. This is the value other endpoints accept as `as_of`.",
							Computed:            true,
						},
						"tags": schema.ListAttribute{
							MarkdownDescription: "Tags pointing at this version.",
							ElementType:         types.StringType,
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *DatasetVersionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatasetVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DatasetVersionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	var listResp []datasetVersionListEntry
	if err := c.Get(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/versions", nil, &listResp); err != nil {
		resp.Diagnostics.AddError("Error listing dataset versions", err.Error())
		return
	}

	data.Versions = make([]datasetVersionEntryModel, 0, len(listResp))
	for _, v := range listResp {
		tags, diags := types.ListValueFrom(ctx, types.StringType, v.Tags)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Versions = append(data.Versions, datasetVersionEntryModel{
			AsOf: types.StringValue(v.AsOf),
			Tags: tags,
		})
	}

	// The endpoint does not echo a workspace identifier.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read dataset versions data source", map[string]interface{}{"version_count": len(data.Versions)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

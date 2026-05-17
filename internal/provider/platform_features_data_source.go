// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &PlatformFeaturesDataSource{}

// platformFeatureRowAttrTypes matches each element of `features` (OpenAPI
// features.FeatureConfig).
var platformFeatureRowAttrTypes = map[string]attr.Type{
	"feature":         types.StringType,
	"default_model":   types.StringType,
	"disabled_models": types.ListType{ElemType: types.StringType},
}

// NewPlatformFeaturesDataSource returns a data source backed by GET
// /v1/platform/features.
func NewPlatformFeaturesDataSource() datasource.DataSource {
	return &PlatformFeaturesDataSource{}
}

// PlatformFeaturesDataSource lists consolidated feature model configuration.
type PlatformFeaturesDataSource struct {
	client *client.Client
}

// PlatformFeaturesDataSourceModel is Terraform state for the listing.
type PlatformFeaturesDataSourceModel struct {
	Features types.List `tfsdk:"features"`
}

func (d *PlatformFeaturesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_features"
}

func (d *PlatformFeaturesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the consolidated LangSmith **platform features** view (`GET /v1/platform/features`): default model and disabled models per feature key. " +
			"Use this data source to discover valid `feature` values and current settings before managing a `langsmith_platform_feature` resource.",
		Attributes: map[string]schema.Attribute{
			"features": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"feature": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Feature key (matches the `{feature}` path segment on `/v1/platform/features/{feature}/...`).",
						},
						"default_model": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Configured default model for this feature, if any.",
						},
						"disabled_models": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Models disabled for this feature (sorted in state for stable plans).",
						},
					},
				},
			},
		},
	}
}

func (d *PlatformFeaturesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PlatformFeaturesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PlatformFeaturesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rows []platformFeatureSnapshot
	if err := d.client.Get(ctx, platformFeaturesAPIPath, nil, &rows); err != nil {
		resp.Diagnostics.AddError("Error reading platform features", err.Error())
		return
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Feature < rows[j].Feature })

	elems := make([]attr.Value, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		dm := types.StringNull()
		if row.DefaultModel != nil && *row.DefaultModel != "" {
			dm = types.StringValue(*row.DefaultModel)
		}
		disabled := sortedStrings(row.DisabledModels)
		disElems := stringSliceToAttrValues(disabled)
		disList, ddiags := types.ListValue(types.StringType, disElems)
		resp.Diagnostics.Append(ddiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		obj := types.ObjectValueMust(platformFeatureRowAttrTypes, map[string]attr.Value{
			"feature":         types.StringValue(row.Feature),
			"default_model":   dm,
			"disabled_models": disList,
		})
		elems = append(elems, obj)
	}

	featureList, ldiags := types.ListValue(types.ObjectType{AttrTypes: platformFeatureRowAttrTypes}, elems)
	resp.Diagnostics.Append(ldiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Features = featureList
	tflog.Trace(ctx, "read platform_features data source", map[string]any{"count": len(rows)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

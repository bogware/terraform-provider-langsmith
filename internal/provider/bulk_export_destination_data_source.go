// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &BulkExportDestinationDataSource{}

// NewBulkExportDestinationDataSource returns a new bulk export destination data source.
func NewBulkExportDestinationDataSource() datasource.DataSource {
	return &BulkExportDestinationDataSource{}
}

// BulkExportDestinationDataSource looks up a LangSmith bulk export destination by ID.
type BulkExportDestinationDataSource struct {
	client *client.Client
}

// bulkExportDestinationDataSourceModel holds the Terraform state for a bulk export
// destination data source. Secrets are never returned by the API; only the configured
// credential key names are surfaced via credentials_keys.
type bulkExportDestinationDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	DisplayName           types.String `tfsdk:"display_name"`
	DestinationType       types.String `tfsdk:"destination_type"`
	BucketName            types.String `tfsdk:"bucket_name"`
	Prefix                types.String `tfsdk:"prefix"`
	Region                types.String `tfsdk:"region"`
	EndpointURL           types.String `tfsdk:"endpoint_url"`
	IncludeBucketInPrefix types.Bool   `tfsdk:"include_bucket_in_prefix"`
	CredentialsKeys       types.List   `tfsdk:"credentials_keys"`
	WorkspaceID           types.String `tfsdk:"workspace_id"`
	TenantID              types.String `tfsdk:"tenant_id"`
	CreatedAt             types.String `tfsdk:"created_at"`
	UpdatedAt             types.String `tfsdk:"updated_at"`
}

func (d *BulkExportDestinationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bulk_export_destination"
}

func (d *BulkExportDestinationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a LangSmith bulk export destination by ID. Credential secrets are never returned by the API; only the configured credential key names are surfaced via `credentials_keys`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the bulk export destination.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the bulk export destination.",
				Computed:            true,
			},
			"destination_type": schema.StringAttribute{
				MarkdownDescription: "The type of the destination (e.g. `s3`).",
				Computed:            true,
			},
			"bucket_name": schema.StringAttribute{
				MarkdownDescription: "The S3 bucket name.",
				Computed:            true,
			},
			"prefix": schema.StringAttribute{
				MarkdownDescription: "The S3 key prefix.",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The AWS region of the S3 bucket.",
				Computed:            true,
			},
			"endpoint_url": schema.StringAttribute{
				MarkdownDescription: "The S3-compatible endpoint URL.",
				Computed:            true,
			},
			"include_bucket_in_prefix": schema.BoolAttribute{
				MarkdownDescription: "Whether to include the bucket name in the S3 key prefix.",
				Computed:            true,
			},
			"credentials_keys": schema.ListAttribute{
				MarkdownDescription: "The keys of configured credentials. Secret values are never returned by the API.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Deprecated: use `workspace_id` instead. The workspace ID.",
				Computed:            true,
				DeprecationMessage:  "Use 'workspace_id' instead. This attribute will be removed in a future version.",
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (d *BulkExportDestinationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BulkExportDestinationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data bulkExportDestinationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result bulkExportDestinationAPIResponse
	err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/bulk-exports/destinations/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading bulk export destination", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.DestinationType = types.StringValue(result.DestinationType)
	data.BucketName = types.StringValue(result.Config.BucketName)

	if result.Config.Prefix != "" {
		data.Prefix = types.StringValue(result.Config.Prefix)
	} else {
		data.Prefix = types.StringNull()
	}
	if result.Config.Region != "" {
		data.Region = types.StringValue(result.Config.Region)
	} else {
		data.Region = types.StringNull()
	}
	if result.Config.EndpointURL != "" {
		data.EndpointURL = types.StringValue(result.Config.EndpointURL)
	} else {
		data.EndpointURL = types.StringNull()
	}
	if result.Config.IncludeBucketInPrefix != nil {
		data.IncludeBucketInPrefix = types.BoolValue(*result.Config.IncludeBucketInPrefix)
	} else {
		data.IncludeBucketInPrefix = types.BoolNull()
	}

	if len(result.CredentialsKeys) > 0 {
		var elems []attr.Value
		for _, s := range result.CredentialsKeys {
			elems = append(elems, types.StringValue(s))
		}
		data.CredentialsKeys, _ = types.ListValue(types.StringType, elems)
	} else {
		data.CredentialsKeys = types.ListNull(types.StringType)
	}

	reconcileWorkspaceID(&data.WorkspaceID, result.TenantID, &resp.Diagnostics)
	data.TenantID = data.WorkspaceID
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

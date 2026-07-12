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
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &BulkExportDestinationsDataSource{}

// NewBulkExportDestinationsDataSource returns a new BulkExportDestinationsDataSource
// that lists every bulk export destination in a workspace.
func NewBulkExportDestinationsDataSource() datasource.DataSource {
	return &BulkExportDestinationsDataSource{}
}

// BulkExportDestinationsDataSource lists LangSmith bulk export destinations via
// GET /api/v1/bulk-exports/destinations. Destination credentials (AWS access key
// ID, secret access key, session token) are write-only in the LangSmith API and
// are never returned by this endpoint; only the names of the configured
// credential keys are surfaced, via `credentials_keys`.
type BulkExportDestinationsDataSource struct {
	client *client.Client
}

// BulkExportDestinationsDataSourceModel holds the workspace override and the
// resulting destinations list.
type BulkExportDestinationsDataSourceModel struct {
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Destinations types.List   `tfsdk:"destinations"`
}

// bulkExportDestinationListAPI mirrors the BulkExportDestination schema returned
// by GET /api/v1/bulk-exports/destinations. The API documents tenant_id;
// workspace_id is also decoded in case the server returns it as an alias.
//
// Deliberately omitted from the config decode: `s3_additional_kwargs` and
// `config_kwargs_s3`, which are free-form botocore keyword bags that could carry
// credential material. Credentials themselves are never returned by the API.
type bulkExportDestinationListAPI struct {
	ID              string                          `json:"id"`
	DisplayName     string                          `json:"display_name"`
	DestinationType string                          `json:"destination_type"`
	Config          bulkExportDestinationListConfig `json:"config"`
	CredentialsKeys []string                        `json:"credentials_keys"`
	WorkspaceID     string                          `json:"workspace_id"`
	TenantID        string                          `json:"tenant_id"`
	CreatedAt       string                          `json:"created_at"`
	UpdatedAt       string                          `json:"updated_at"`
}

type bulkExportDestinationListConfig struct {
	BucketName            *string `json:"bucket_name"`
	Prefix                *string `json:"prefix"`
	Region                *string `json:"region"`
	EndpointURL           *string `json:"endpoint_url"`
	IncludeBucketInPrefix *bool   `json:"include_bucket_in_prefix"`
	AWSRoleARN            *string `json:"aws_role_arn"`
}

var bulkExportDestinationObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                       types.StringType,
	"display_name":             types.StringType,
	"destination_type":         types.StringType,
	"bucket_name":              types.StringType,
	"prefix":                   types.StringType,
	"region":                   types.StringType,
	"endpoint_url":             types.StringType,
	"include_bucket_in_prefix": types.BoolType,
	"aws_role_arn":             types.StringType,
	"credentials_keys":         types.ListType{ElemType: types.StringType},
	"workspace_id":             types.StringType,
	"created_at":               types.StringType,
	"updated_at":               types.StringType,
}}

func (d *BulkExportDestinationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bulk_export_destinations"
}

func (d *BulkExportDestinationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all LangSmith bulk export destinations in a workspace. " +
			"Credential values (AWS access key ID, secret access key, session token) are never returned by the LangSmith API and are never exposed by this data source; only the names of the configured credential keys are surfaced, via `credentials_keys`.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID to list bulk export destinations from. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"destinations": schema.ListNestedAttribute{
				MarkdownDescription: "The bulk export destinations in the workspace.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the bulk export destination.",
							Computed:            true,
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
							MarkdownDescription: "Whether the bucket name is prepended to the S3 key prefix.",
							Computed:            true,
						},
						"aws_role_arn": schema.StringAttribute{
							MarkdownDescription: "The AWS IAM role ARN that LangSmith assumes instead of using static credentials, if configured.",
							Computed:            true,
						},
						"credentials_keys": schema.ListAttribute{
							MarkdownDescription: "The names of the credential keys configured on the destination. Credential values are never returned by the LangSmith API.",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"workspace_id": schema.StringAttribute{
							MarkdownDescription: "The workspace ID that owns the destination.",
							Computed:            true,
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
				},
			},
		},
	}
}

func (d *BulkExportDestinationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BulkExportDestinationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BulkExportDestinationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(d.client, data.WorkspaceID)

	// The destinations endpoint returns the full list in a single response; it
	// exposes no pagination parameters.
	var result []bulkExportDestinationListAPI
	if err := c.Get(ctx, "/api/v1/bulk-exports/destinations", nil, &result); err != nil {
		resp.Diagnostics.AddError("Error listing bulk export destinations", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(result))
	for _, dest := range result {
		credentialsKeys := types.ListNull(types.StringType)
		if len(dest.CredentialsKeys) > 0 {
			keyElems := make([]attr.Value, 0, len(dest.CredentialsKeys))
			for _, k := range dest.CredentialsKeys {
				keyElems = append(keyElems, types.StringValue(k))
			}
			list, diags := types.ListValue(types.StringType, keyElems)
			resp.Diagnostics.Append(diags...)
			credentialsKeys = list
		}

		includeBucketInPrefix := types.BoolNull()
		if dest.Config.IncludeBucketInPrefix != nil {
			includeBucketInPrefix = types.BoolValue(*dest.Config.IncludeBucketInPrefix)
		}

		obj, diags := types.ObjectValue(bulkExportDestinationObjectType.AttrTypes, map[string]attr.Value{
			"id":                       types.StringValue(dest.ID),
			"display_name":             types.StringValue(dest.DisplayName),
			"destination_type":         types.StringValue(dest.DestinationType),
			"bucket_name":              bulkExportStringValue(dest.Config.BucketName),
			"prefix":                   bulkExportStringValue(dest.Config.Prefix),
			"region":                   bulkExportStringValue(dest.Config.Region),
			"endpoint_url":             bulkExportStringValue(dest.Config.EndpointURL),
			"include_bucket_in_prefix": includeBucketInPrefix,
			"aws_role_arn":             bulkExportStringValue(dest.Config.AWSRoleARN),
			"credentials_keys":         credentialsKeys,
			"workspace_id":             types.StringValue(firstNonEmpty(dest.WorkspaceID, dest.TenantID)),
			"created_at":               types.StringValue(dest.CreatedAt),
			"updated_at":               types.StringValue(dest.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}

	list, diags := types.ListValue(bulkExportDestinationObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Destinations = list

	// The listing has no single workspace field of its own; fall back to the
	// workspace the client is operating in so workspace_id is never unknown.
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "read bulk export destinations data source", map[string]interface{}{"count": len(result)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

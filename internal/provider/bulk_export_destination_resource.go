// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &BulkExportDestinationResource{}
	_ resource.ResourceWithImportState = &BulkExportDestinationResource{}
)

// NewBulkExportDestinationResource sets up a fresh outpost for shipping data out of Dodge.
func NewBulkExportDestinationResource() resource.Resource {
	return &BulkExportDestinationResource{}
}

// BulkExportDestinationResource manages an S3-compatible destination where LangSmith
// ships its bulk export cargo. Once created, the API offers no way to tear it down --
// like a building on Front Street, it stays standing whether you want it or not.
type BulkExportDestinationResource struct {
	client *client.Client
}

// BulkExportDestinationResourceModel holds the Terraform state for a bulk export destination,
// including S3 bucket coordinates, credentials, and timestamps.
type BulkExportDestinationResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	DisplayName           types.String `tfsdk:"display_name"`
	DestinationType       types.String `tfsdk:"destination_type"`
	BucketName            types.String `tfsdk:"bucket_name"`
	Prefix                types.String `tfsdk:"prefix"`
	Region                types.String `tfsdk:"region"`
	EndpointURL           types.String `tfsdk:"endpoint_url"`
	IncludeBucketInPrefix types.Bool   `tfsdk:"include_bucket_in_prefix"`
	AccessKeyID           types.String `tfsdk:"access_key_id"`
	SecretAccessKey       types.String `tfsdk:"secret_access_key"`
	SessionToken          types.String `tfsdk:"session_token"`
	WorkspaceID           types.String `tfsdk:"workspace_id"`
	CreatedAt             types.String `tfsdk:"created_at"`
	UpdatedAt             types.String `tfsdk:"updated_at"`
	CredentialsKeys       types.List   `tfsdk:"credentials_keys"`
}

// bulkExportDestinationAPICreateRequest is the request body for creating a bulk export destination.
type bulkExportDestinationAPICreateRequest struct {
	DisplayName     string                            `json:"display_name"`
	DestinationType string                            `json:"destination_type"`
	Config          bulkExportDestinationConfig       `json:"config"`
	Credentials     *bulkExportDestinationCredentials `json:"credentials,omitempty"`
}

type bulkExportDestinationConfig struct {
	BucketName            string `json:"bucket_name"`
	Prefix                string `json:"prefix,omitempty"`
	Region                string `json:"region,omitempty"`
	EndpointURL           string `json:"endpoint_url,omitempty"`
	IncludeBucketInPrefix *bool  `json:"include_bucket_in_prefix,omitempty"`
}

type bulkExportDestinationCredentials struct {
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken    string `json:"session_token,omitempty"`
}

// bulkExportDestinationAPIUpdateRequest is the request body for updating a bulk export destination.
type bulkExportDestinationAPIUpdateRequest struct {
	Credentials *bulkExportDestinationCredentials `json:"credentials,omitempty"`
}

// bulkExportDestinationAPIResponse is the API response for a bulk export destination.
type bulkExportDestinationAPIResponse struct {
	ID              string                      `json:"id"`
	DisplayName     string                      `json:"display_name"`
	DestinationType string                      `json:"destination_type"`
	Config          bulkExportDestinationConfig `json:"config"`
	WorkspaceID     string                      `json:"workspace_id"`
	CreatedAt       string                      `json:"created_at"`
	UpdatedAt       string                      `json:"updated_at"`
	CredentialsKeys []string                    `json:"credentials_keys"`
}

func (r *BulkExportDestinationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bulk_export_destination"
}

func (r *BulkExportDestinationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith bulk export destination. **Note:** The LangSmith API does not support deleting bulk export destinations. Destroying this resource will only remove it from Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the bulk export destination.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the bulk export destination.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"destination_type": schema.StringAttribute{
				MarkdownDescription: "The type of the destination. Defaults to `s3`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("s3"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bucket_name": schema.StringAttribute{
				MarkdownDescription: "The S3 bucket name.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prefix": schema.StringAttribute{
				MarkdownDescription: "The S3 key prefix.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The AWS region of the S3 bucket.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint_url": schema.StringAttribute{
				MarkdownDescription: "The S3-compatible endpoint URL.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"include_bucket_in_prefix": schema.BoolAttribute{
				MarkdownDescription: "Whether to include the bucket name in the S3 key prefix.",
				Optional:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"access_key_id": schema.StringAttribute{
				MarkdownDescription: "The AWS access key ID for the destination.",
				Optional:            true,
				Sensitive:           true,
			},
			"secret_access_key": schema.StringAttribute{
				MarkdownDescription: "The AWS secret access key for the destination.",
				Optional:            true,
				Sensitive:           true,
			},
			"session_token": schema.StringAttribute{
				MarkdownDescription: "The AWS session token for temporary credentials (e.g., from STS AssumeRole).",
				Optional:            true,
				Sensitive:           true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID of the resource. If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last update timestamp.",
				Computed:            true,
			},
			"credentials_keys": schema.ListAttribute{
				MarkdownDescription: "The keys of configured credentials.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *BulkExportDestinationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *BulkExportDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BulkExportDestinationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := bulkExportDestinationAPICreateRequest{
		DisplayName:     data.DisplayName.ValueString(),
		DestinationType: data.DestinationType.ValueString(),
		Config: bulkExportDestinationConfig{
			BucketName: data.BucketName.ValueString(),
		},
	}

	if !data.Prefix.IsNull() && !data.Prefix.IsUnknown() {
		body.Config.Prefix = data.Prefix.ValueString()
	}
	if !data.Region.IsNull() && !data.Region.IsUnknown() {
		body.Config.Region = data.Region.ValueString()
	}
	if !data.EndpointURL.IsNull() && !data.EndpointURL.IsUnknown() {
		body.Config.EndpointURL = data.EndpointURL.ValueString()
	}
	if !data.IncludeBucketInPrefix.IsNull() && !data.IncludeBucketInPrefix.IsUnknown() {
		v := data.IncludeBucketInPrefix.ValueBool()
		body.Config.IncludeBucketInPrefix = &v
	}

	creds := &bulkExportDestinationCredentials{}
	hasCreds := false
	if !data.AccessKeyID.IsNull() && !data.AccessKeyID.IsUnknown() {
		creds.AccessKeyID = data.AccessKeyID.ValueString()
		hasCreds = true
	}
	if !data.SecretAccessKey.IsNull() && !data.SecretAccessKey.IsUnknown() {
		creds.SecretAccessKey = data.SecretAccessKey.ValueString()
		hasCreds = true
	}
	if !data.SessionToken.IsNull() && !data.SessionToken.IsUnknown() {
		creds.SessionToken = data.SessionToken.ValueString()
		hasCreds = true
	}
	if hasCreds {
		body.Credentials = creds
	}

	var result bulkExportDestinationAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Post(ctx, "/api/v1/bulk-exports/destinations", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating bulk export destination", err.Error())
		return
	}

	mapBulkExportDestinationResponseToState(&data, &result, &resp.Diagnostics)
	tflog.Trace(ctx, "created bulk export destination resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BulkExportDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BulkExportDestinationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result bulkExportDestinationAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/api/v1/bulk-exports/destinations/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading bulk export destination", err.Error())
		return
	}

	mapBulkExportDestinationResponseToState(&data, &result, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BulkExportDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BulkExportDestinationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := bulkExportDestinationAPIUpdateRequest{}
	creds := &bulkExportDestinationCredentials{}
	hasCreds := false
	if !data.AccessKeyID.IsNull() && !data.AccessKeyID.IsUnknown() {
		creds.AccessKeyID = data.AccessKeyID.ValueString()
		hasCreds = true
	}
	if !data.SecretAccessKey.IsNull() && !data.SecretAccessKey.IsUnknown() {
		creds.SecretAccessKey = data.SecretAccessKey.ValueString()
		hasCreds = true
	}
	if !data.SessionToken.IsNull() && !data.SessionToken.IsUnknown() {
		creds.SessionToken = data.SessionToken.ValueString()
		hasCreds = true
	}
	if hasCreds {
		body.Credentials = creds
	}

	var result bulkExportDestinationAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Patch(ctx, "/api/v1/bulk-exports/destinations/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating bulk export destination", err.Error())
		return
	}

	mapBulkExportDestinationResponseToState(&data, &result, &resp.Diagnostics)
	tflog.Trace(ctx, "updated bulk export destination resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BulkExportDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// The LangSmith API has no delete endpoint for destinations -- some things
	// in this town you just can't get rid of. We tip our hat and walk away.
	tflog.Trace(ctx, "bulk export destination delete is a no-op (API does not support deletion)")
}

func (r *BulkExportDestinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapBulkExportDestinationResponseToState rounds up the API response fields and
// brands them onto the Terraform state model. Absent optional fields get null values
// to keep Terraform from reporting phantom drift.
func mapBulkExportDestinationResponseToState(data *BulkExportDestinationResourceModel, result *bulkExportDestinationAPIResponse, diags *diag.Diagnostics) {
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

	reconcileWorkspaceID(&data.WorkspaceID, result.WorkspaceID, diags)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	// Round up the credential keys -- like counting heads after a cattle drive,
	// you want to know exactly which ones made it through.
	if len(result.CredentialsKeys) > 0 {
		var elems []attr.Value
		for _, s := range result.CredentialsKeys {
			elems = append(elems, types.StringValue(s))
		}
		data.CredentialsKeys, _ = types.ListValue(types.StringType, elems)
	} else {
		data.CredentialsKeys = types.ListNull(types.StringType)
	}
}

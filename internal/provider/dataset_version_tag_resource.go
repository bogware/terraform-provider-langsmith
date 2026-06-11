// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &DatasetVersionTagResource{}
	_ resource.ResourceWithImportState = &DatasetVersionTagResource{}
)

// NewDatasetVersionTagResource returns a resource for managing named tags on
// dataset versions, e.g. pinning `prod` to a known-good snapshot.
func NewDatasetVersionTagResource() resource.Resource {
	return &DatasetVersionTagResource{}
}

// DatasetVersionTagResource manages a named tag pointing at a dataset version.
type DatasetVersionTagResource struct {
	client *client.Client
}

// DatasetVersionTagResourceModel maps the Terraform schema for a dataset version tag.
type DatasetVersionTagResourceModel struct {
	DatasetID   types.String `tfsdk:"dataset_id"`
	Tag         types.String `tfsdk:"tag"`
	AsOf        types.String `tfsdk:"as_of"`
	VersionAsOf types.String `tfsdk:"version_as_of"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

// datasetVersionTagPutRequest is sent to PUT /api/v1/datasets/{dataset_id}/tags.
type datasetVersionTagPutRequest struct {
	AsOf string `json:"as_of"`
	Tag  string `json:"tag"`
}

// datasetVersionAPIResponse is the DatasetVersion shape from the API.
type datasetVersionAPIResponse struct {
	Tags []string `json:"tags"`
	AsOf string   `json:"as_of"`
}

func (r *DatasetVersionTagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_version_tag"
}

func (r *DatasetVersionTagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a named tag on a dataset version. Tags like `prod` point to a specific dataset snapshot (resolved from `as_of`), letting you pin evaluations to a known-good version. " +
			"The LangSmith API only supports upserting tags, so destroying this resource removes it from Terraform state without deleting the tag in LangSmith; re-tag a different version to move it.",
		Attributes: map[string]schema.Attribute{
			"dataset_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the dataset.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"tag": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the tag (e.g., `prod`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"as_of": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The dataset version to tag: the exact RFC 3339 timestamp of an existing dataset version, or `latest` to tag the most recent version. " +
					"Update this value to move the tag to a different version. The canonical version timestamp the server resolved this to is exposed as `version_as_of`.",
			},
			"version_as_of": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The canonical timestamp of the dataset version the tag currently points to, as resolved by the server.",
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *DatasetVersionTagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

// upsertTag issues the PUT that creates or moves the tag, then verifies the
// tag resolves to a dataset version. The PUT endpoint returns 200 even when
// as_of matches no version (the tag is simply not applied), so the follow-up
// lookup is required to fail loudly instead of silently drifting.
func (r *DatasetVersionTagResource) upsertTag(ctx context.Context, c *client.Client, data *DatasetVersionTagResourceModel) error {
	body := datasetVersionTagPutRequest{
		AsOf: data.AsOf.ValueString(),
		Tag:  data.Tag.ValueString(),
	}
	if err := c.Put(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/tags", body, nil); err != nil {
		return err
	}
	q := url.Values{}
	q.Set("tag", data.Tag.ValueString())
	var verified datasetVersionAPIResponse
	if err := c.Get(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/version", q, &verified); err != nil {
		if client.IsNotFound(err) {
			return fmt.Errorf("tag %q was not applied: no dataset version exists at as_of %q. Use the exact timestamp of an existing dataset version, or \"latest\" to tag the most recent version",
				data.Tag.ValueString(), data.AsOf.ValueString())
		}
		return err
	}
	data.VersionAsOf = types.StringValue(verified.AsOf)
	return nil
}

func (r *DatasetVersionTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasetVersionTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient := effectiveClient(r.client, data.WorkspaceID)
	if err := r.upsertTag(ctx, apiClient, &data); err != nil {
		resp.Diagnostics.AddError("Error creating dataset version tag", err.Error())
		return
	}
	// The dataset version API does not return workspace information.
	finalizeWorkspaceID(&data.WorkspaceID, apiClient, "", &resp.Diagnostics)

	tflog.Trace(ctx, "created dataset version tag", map[string]interface{}{
		"dataset_id": data.DatasetID.ValueString(),
		"tag":        data.Tag.ValueString(),
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetVersionTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasetVersionTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient := effectiveClient(r.client, data.WorkspaceID)
	q := url.Values{}
	q.Set("tag", data.Tag.ValueString())
	var result datasetVersionAPIResponse
	err := apiClient.Get(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/version", q, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading dataset version tag", err.Error())
		return
	}
	if result.AsOf == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	data.VersionAsOf = types.StringValue(result.AsOf)
	// On import as_of is not known; seed it with the canonical version
	// timestamp so the configuration can adopt the current pointer.
	if data.AsOf.IsNull() || data.AsOf.IsUnknown() {
		data.AsOf = types.StringValue(result.AsOf)
	}
	// The dataset version API does not return workspace information.
	finalizeWorkspaceID(&data.WorkspaceID, apiClient, "", &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetVersionTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasetVersionTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient := effectiveClient(r.client, data.WorkspaceID)
	if err := r.upsertTag(ctx, apiClient, &data); err != nil {
		resp.Diagnostics.AddError("Error updating dataset version tag", err.Error())
		return
	}
	// The dataset version API does not return workspace information.
	finalizeWorkspaceID(&data.WorkspaceID, apiClient, "", &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetVersionTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// The LangSmith API only exposes an upsert (PUT) for dataset version
	// tags; there is no endpoint to delete one. Removing the resource only
	// drops it from Terraform state.
	resp.Diagnostics.AddWarning(
		"Dataset version tag not deleted in LangSmith",
		"The LangSmith API does not support deleting dataset version tags; the tag was removed from Terraform state only and still points at its last version in LangSmith.",
	)
	tflog.Warn(ctx, "dataset version tags cannot be deleted via the API; removing from Terraform state only")
}

func (r *DatasetVersionTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: dataset_id:tag
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: dataset_id:tag")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dataset_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag"), parts[1])...)
}

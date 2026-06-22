// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

const workspaceTTLSettingsPath = "/workspaces/current/ttl-settings"

var (
	_ resource.Resource                = &WorkspaceTTLSettingsResource{}
	_ resource.ResourceWithImportState = &WorkspaceTTLSettingsResource{}
)

// NewWorkspaceTTLSettingsResource returns a new WorkspaceTTLSettingsResource for
// managing per-workspace longlived trace retention.
func NewWorkspaceTTLSettingsResource() resource.Resource {
	return &WorkspaceTTLSettingsResource{}
}

// WorkspaceTTLSettingsResource manages the longlived trace retention (TTL) for a
// single workspace. This is a singleton resource: one per workspace, always
// present, never truly created or destroyed. It is distinct from the org-level
// langsmith_ttl_settings resource.
type WorkspaceTTLSettingsResource struct {
	client *client.Client
}

// workspaceTTLSettingsResourceModel holds the Terraform state for workspace TTL
// settings.
type workspaceTTLSettingsResourceModel struct {
	ID               types.String `tfsdk:"id"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	TenantID         types.String `tfsdk:"tenant_id"`
	LonglivedTTLDays types.Int64  `tfsdk:"longlived_ttl_days"`
	IsCustom         types.Bool   `tfsdk:"is_custom"`
}

// workspaceTTLSettingsUpdateRequest is the PUT request body for updating the
// longlived trace TTL of a workspace.
type workspaceTTLSettingsUpdateRequest struct {
	LonglivedTTLDays int64 `json:"longlived_ttl_days"`
}

// workspaceTTLSettingsAPIResponse is what the API returns for workspace TTL
// settings. LangSmith returns the workspace identifier under tenant_id; we also
// decode workspace_id defensively.
type workspaceTTLSettingsAPIResponse struct {
	TenantID         string `json:"tenant_id"`
	WorkspaceID      string `json:"workspace_id"`
	LonglivedTTLDays int64  `json:"longlived_ttl_days"`
	IsCustom         bool   `json:"is_custom"`
}

func (r *WorkspaceTTLSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_ttl_settings"
}

func (r *WorkspaceTTLSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the longlived trace retention (TTL) for a single LangSmith workspace. This is a singleton resource: there is exactly one TTL configuration per workspace, and it cannot be created or destroyed, only updated. This is distinct from the organization-level `langsmith_ttl_settings` resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the workspace TTL settings (the workspace/tenant ID).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID whose TTL settings are managed. Overrides the provider-level workspace for this resource's API calls. If omitted, the provider-level workspace (`current`) is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Deprecated: use `workspace_id` instead.",
				DeprecationMessage:  "Use workspace_id instead. This attribute will be removed in a future release.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"longlived_ttl_days": schema.Int64Attribute{
				MarkdownDescription: "The number of days longlived traces are retained for this workspace.",
				Required:            true,
			},
			"is_custom": schema.BoolAttribute{
				MarkdownDescription: "Whether the workspace has a custom TTL configured (as opposed to inheriting a default).",
				Computed:            true,
			},
		},
	}
}

func (r *WorkspaceTTLSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WorkspaceTTLSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data workspaceTTLSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.upsert(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created workspace TTL settings resource", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceTTLSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data workspaceTTLSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)

	var result workspaceTTLSettingsAPIResponse
	err := effClient.Get(ctx, workspaceTTLSettingsPath, nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading workspace TTL settings", err.Error())
		return
	}

	r.mapResponseToState(&data, &result, effClient, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceTTLSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data workspaceTTLSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.upsert(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "updated workspace TTL settings resource", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceTTLSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Workspace TTL settings are a singleton and cannot be deleted. Removing the
	// resource from Terraform state leaves the workspace configuration untouched.
	tflog.Warn(ctx, "Workspace TTL settings are a singleton resource and cannot be deleted. Removing from Terraform state only.")
}

func (r *WorkspaceTTLSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// upsert performs the PUT that sets the longlived TTL for the workspace and maps
// the response into Terraform state.
func (r *WorkspaceTTLSettingsResource) upsert(ctx context.Context, data *workspaceTTLSettingsResourceModel, diags *diag.Diagnostics) {
	effClient := effectiveClient(r.client, data.WorkspaceID)

	body := workspaceTTLSettingsUpdateRequest{
		LonglivedTTLDays: data.LonglivedTTLDays.ValueInt64(),
	}

	var result workspaceTTLSettingsAPIResponse
	err := effClient.Put(ctx, workspaceTTLSettingsPath, body, &result)
	if err != nil {
		diags.AddError("Error updating workspace TTL settings", err.Error())
		return
	}

	r.mapResponseToState(data, &result, effClient, diags)
}

// mapResponseToState writes the API response into Terraform state, reconciling
// the workspace identifier and guaranteeing workspace_id is never left unknown.
func (r *WorkspaceTTLSettingsResource) mapResponseToState(data *workspaceTTLSettingsResourceModel, result *workspaceTTLSettingsAPIResponse, effClient *client.Client, diags *diag.Diagnostics) {
	apiWorkspaceID := firstNonEmpty(result.WorkspaceID, result.TenantID)

	finalizeWorkspaceID(&data.WorkspaceID, effClient, apiWorkspaceID, diags)
	data.TenantID = data.WorkspaceID

	if !data.WorkspaceID.IsNull() {
		data.ID = data.WorkspaceID
	} else {
		data.ID = types.StringValue(apiWorkspaceID)
	}

	data.LonglivedTTLDays = types.Int64Value(result.LonglivedTTLDays)
	data.IsCustom = types.BoolValue(result.IsCustom)
}

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

var (
	_ resource.Resource                = &WorkspaceHandleResource{}
	_ resource.ResourceWithImportState = &WorkspaceHandleResource{}
)

// NewWorkspaceHandleResource returns a resource for managing the unique
// public handle of a workspace (tenant).
func NewWorkspaceHandleResource() resource.Resource {
	return &WorkspaceHandleResource{}
}

// WorkspaceHandleResource manages the workspace (tenant) handle. This is a
// singleton per workspace: a handle can be set and changed but never unset.
type WorkspaceHandleResource struct {
	client *client.Client
}

// WorkspaceHandleResourceModel maps the Terraform schema for a workspace handle.
type WorkspaceHandleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Handle      types.String `tfsdk:"handle"`
	DisplayName types.String `tfsdk:"display_name"`
	CreatedAt   types.String `tfsdk:"created_at"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

// setTenantHandleRequest is sent to POST /api/v1/settings/handle.
type setTenantHandleRequest struct {
	TenantHandle string `json:"tenant_handle"`
}

// tenantSettingsAPIResponse is the Tenant shape returned by the settings endpoints.
type tenantSettingsAPIResponse struct {
	ID           string  `json:"id"`
	DisplayName  string  `json:"display_name"`
	CreatedAt    string  `json:"created_at"`
	TenantHandle *string `json:"tenant_handle"`
}

func (r *WorkspaceHandleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_handle"
}

func (r *WorkspaceHandleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the public handle of a LangSmith workspace (used e.g. as the owner segment of public prompt URLs). This is a singleton resource per workspace. " +
			"Handles cannot be unset once assigned, so destroying this resource only removes it from Terraform state; the workspace keeps its handle.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "UUID of the workspace (tenant) the handle belongs to.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"handle": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The handle to assign to the workspace. Must be globally unique across LangSmith.",
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the workspace.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the workspace was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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

func (r *WorkspaceHandleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// setHandle POSTs the handle and maps the resulting tenant onto state.
func (r *WorkspaceHandleResource) setHandle(ctx context.Context, data *WorkspaceHandleResourceModel, diags *diag.Diagnostics) {
	body := setTenantHandleRequest{TenantHandle: data.Handle.ValueString()}
	var result tenantSettingsAPIResponse
	if err := effectiveClient(r.client, data.WorkspaceID).Post(ctx, "/api/v1/settings/handle", body, &result); err != nil {
		diags.AddError("Error setting workspace handle", err.Error())
		return
	}
	mapTenantSettingsToHandleState(data, &result, diags)
}

func (r *WorkspaceHandleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceHandleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.setHandle(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "set workspace handle", map[string]interface{}{"id": data.ID.ValueString(), "handle": data.Handle.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceHandleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceHandleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result tenantSettingsAPIResponse
	if err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/api/v1/settings", nil, &result); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading workspace settings", err.Error())
		return
	}
	if result.TenantHandle == nil || *result.TenantHandle == "" {
		// No handle assigned to this workspace.
		resp.State.RemoveResource(ctx)
		return
	}

	mapTenantSettingsToHandleState(&data, &result, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceHandleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkspaceHandleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.setHandle(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceHandleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// The LangSmith API has no way to unset a workspace handle once it has
	// been assigned, so deletion only removes the resource from state.
	resp.Diagnostics.AddWarning(
		"Workspace handle not removed in LangSmith",
		"Workspace handles cannot be unset via the LangSmith API; the resource was removed from Terraform state only and the workspace keeps its current handle.",
	)
	tflog.Warn(ctx, "workspace handles cannot be unset via the API; removing from Terraform state only")
}

func (r *WorkspaceHandleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The handle is a per-workspace singleton; import using the current
	// handle value (it is re-read from the API on refresh).
	resource.ImportStatePassthroughID(ctx, path.Root("handle"), req, resp)
}

// mapTenantSettingsToHandleState copies the API tenant response onto the
// Terraform state model.
func mapTenantSettingsToHandleState(data *WorkspaceHandleResourceModel, result *tenantSettingsAPIResponse, diags *diag.Diagnostics) {
	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	if result.TenantHandle != nil && *result.TenantHandle != "" {
		data.Handle = types.StringValue(*result.TenantHandle)
	}
	reconcileWorkspaceID(&data.WorkspaceID, result.ID, diags)
}

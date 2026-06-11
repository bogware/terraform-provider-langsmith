// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &MCPVendorSettingsResource{}
	_ resource.ResourceWithImportState = &MCPVendorSettingsResource{}
)

func NewMCPVendorSettingsResource() resource.Resource {
	return &MCPVendorSettingsResource{}
}

type MCPVendorSettingsResource struct {
	client *client.Client
}

type MCPVendorSettingsResourceModel struct {
	ID             types.String `tfsdk:"id"`
	VendorSlug     types.String `tfsdk:"vendor_slug"`
	OrganizationID types.String `tfsdk:"organization_id"`
	ProjectID      types.String `tfsdk:"project_id"`
	IsConfigured   types.Bool   `tfsdk:"is_configured"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
}

type mcpVendorSettingsRequest struct {
	OrganizationID *string `json:"organization_id,omitempty"`
	ProjectID      *string `json:"project_id,omitempty"`
}

type mcpVendorSettingsAPI struct {
	IsConfigured   bool   `json:"is_configured"`
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id"`
}

func (r *MCPVendorSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_vendor_settings"
}

func (r *MCPVendorSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the account settings for a LangSmith MCP vendor (the vendor-side organization and project the integration connects to). A vendor has at most one settings object; updates fully replace it. Import ID format: `<vendor_slug>` or `<vendor_slug>:<workspace_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vendor_slug": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the MCP vendor (see the `langsmith_mcp_vendor` data source). Cannot be changed after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"organization_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Vendor-side organization identifier the integration connects to.",
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Vendor-side project identifier the integration connects to.",
			},
			"is_configured": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the vendor reports the settings as fully configured.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
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

func (r *MCPVendorSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func mcpVendorSettingsPath(vendorSlug string) string {
	return "/v1/platform/mcp-vendors/" + vendorSlug + "/settings"
}

func (r *MCPVendorSettingsResource) buildRequest(data *MCPVendorSettingsResourceModel) mcpVendorSettingsRequest {
	body := mcpVendorSettingsRequest{}
	if !data.OrganizationID.IsNull() && !data.OrganizationID.IsUnknown() {
		v := data.OrganizationID.ValueString()
		body.OrganizationID = &v
	}
	if !data.ProjectID.IsNull() && !data.ProjectID.IsUnknown() {
		v := data.ProjectID.ValueString()
		body.ProjectID = &v
	}
	return body
}

func (r *MCPVendorSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MCPVendorSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api mcpVendorSettingsAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Post(ctx, mcpVendorSettingsPath(data.VendorSlug.ValueString()), r.buildRequest(&data), &api); err != nil {
		resp.Diagnostics.AddError("Error creating MCP vendor settings", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created MCP vendor settings", map[string]interface{}{"vendor_slug": data.VendorSlug.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPVendorSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPVendorSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api mcpVendorSettingsAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, mcpVendorSettingsPath(data.VendorSlug.ValueString()), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading MCP vendor settings", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPVendorSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MCPVendorSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api mcpVendorSettingsAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Put(ctx, mcpVendorSettingsPath(data.VendorSlug.ValueString()), r.buildRequest(&data), &api); err != nil {
		resp.Diagnostics.AddError("Error updating MCP vendor settings", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPVendorSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MCPVendorSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, mcpVendorSettingsPath(data.VendorSlug.ValueString())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting MCP vendor settings", err.Error())
		return
	}
}

func (r *MCPVendorSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if parts[0] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<vendor_slug>\" or \"<vendor_slug>:<workspace_id>\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vendor_slug"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	if len(parts) == 2 && parts[1] != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[1])...)
	}
}

func (r *MCPVendorSettingsResource) mapResponse(api *mcpVendorSettingsAPI, data *MCPVendorSettingsResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(data.VendorSlug.ValueString())
	if api.OrganizationID != "" {
		data.OrganizationID = types.StringValue(api.OrganizationID)
	} else {
		data.OrganizationID = types.StringNull()
	}
	if api.ProjectID != "" {
		data.ProjectID = types.StringValue(api.ProjectID)
	} else {
		data.ProjectID = types.StringNull()
	}
	data.IsConfigured = types.BoolValue(api.IsConfigured)
	reconcileWorkspaceID(&data.WorkspaceID, "", diags)
}

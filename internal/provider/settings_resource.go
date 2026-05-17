// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &SettingsResource{}
	_ resource.ResourceWithImportState = &SettingsResource{}
)

// NewSettingsResource returns a resource for the current workspace settings
// surface documented as GET /api/v1/settings and POST /api/v1/settings/handle.
func NewSettingsResource() resource.Resource {
	return &SettingsResource{}
}

// SettingsResource manages the workspace tenant handle for the workspace
// selected by the provider (X-Tenant-Id / LANGSMITH_TENANT_ID).
type SettingsResource struct {
	client *client.Client
}

// SettingsResourceModel is Terraform state for workspace settings.
type SettingsResourceModel struct {
	ID           types.String `tfsdk:"id"`
	TenantHandle types.String `tfsdk:"tenant_handle"`
	DisplayName  types.String `tfsdk:"display_name"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

// settingsTenantAPIResponse is the OpenAPI Tenant object returned by
// GET /api/v1/settings and POST /api/v1/settings/handle.
type settingsTenantAPIResponse struct {
	ID           string  `json:"id"`
	DisplayName  string  `json:"display_name"`
	CreatedAt    string  `json:"created_at"`
	TenantHandle *string `json:"tenant_handle"`
}

type setTenantHandleRequest struct {
	TenantHandle string `json:"tenant_handle"`
}

func (r *SettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings"
}

func (r *SettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the **current workspace** tenant handle via `POST /api/v1/settings/handle` and reads workspace metadata from `GET /api/v1/settings` (OpenAPI `Tenant`). " +
			"The workspace is whichever tenant the provider targets (`tenant_id` / `LANGSMITH_TENANT_ID` and `X-Tenant-Id`). " +
			"For a read-only view without changing the handle, use the `langsmith_settings` data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The workspace (tenant) UUID returned by the settings API.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant_handle": schema.StringAttribute{
				MarkdownDescription: "The desired workspace handle (slug). The provider issues `POST /api/v1/settings/handle` when this value differs from the server.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The workspace display name from `GET /api/v1/settings`.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Workspace creation timestamp (RFC3339) from the settings API.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.applyDesiredHandle(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created settings resource", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.readSettings(ctx, &data, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.applyDesiredHandle(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "updated settings resource", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Warn(ctx, "Workspace settings cannot be deleted via the LangSmith API; removing from Terraform state only.")
}

func (r *SettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SettingsResource) applyDesiredHandle(ctx context.Context, data *SettingsResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	rawDesiredHandle := data.TenantHandle.ValueString()
	desiredHandle := strings.TrimSpace(rawDesiredHandle)
	if desiredHandle == "" {
		diags.AddAttributeError(
			path.Root("tenant_handle"),
			"Invalid tenant_handle value",
			"tenant_handle must contain at least one non-whitespace character.",
		)
		return diags
	}
	if rawDesiredHandle != desiredHandle {
		diags.AddAttributeError(
			path.Root("tenant_handle"),
			"Invalid tenant_handle value",
			"tenant_handle must not include leading or trailing whitespace.",
		)
		return diags
	}

	var current settingsTenantAPIResponse
	if err := r.readSettingsRaw(ctx, &current); err != nil {
		diags.AddError("Error reading workspace settings", err.Error())
		return diags
	}

	if !data.ID.IsNull() && !data.ID.IsUnknown() && current.ID != data.ID.ValueString() {
		diags.AddError(
			"Workspace settings ID mismatch",
			fmt.Sprintf("The settings API returned tenant id %q, but state expected %q.", current.ID, data.ID.ValueString()),
		)
		return diags
	}

	apiHandle := tenantHandleStringFromAPI(current.TenantHandle)
	if desiredHandle != apiHandle {
		body := setTenantHandleRequest{TenantHandle: desiredHandle}
		var posted settingsTenantAPIResponse
		if err := r.client.Post(ctx, "/api/v1/settings/handle", body, &posted); err != nil {
			diags.AddError("Error setting tenant handle", err.Error())
			return diags
		}
		mapSettingsTenantToResourceModel(data, &posted)
		data.TenantHandle = types.StringValue(tenantHandleStringFromAPI(posted.TenantHandle))
		return diags
	}

	mapSettingsTenantToResourceModel(data, &current)
	data.TenantHandle = types.StringValue(apiHandle)
	return diags
}

// readSettings loads GET /api/v1/settings into data. If verifyID is true and data.ID is set,
// the response tenant id must match.
func (r *SettingsResource) readSettings(ctx context.Context, data *SettingsResourceModel, verifyID bool) diag.Diagnostics {
	var diags diag.Diagnostics
	var api settingsTenantAPIResponse
	if err := r.readSettingsRaw(ctx, &api); err != nil {
		diags.AddError("Error reading workspace settings", err.Error())
		return diags
	}

	if verifyID && !data.ID.IsNull() && !data.ID.IsUnknown() && api.ID != data.ID.ValueString() {
		diags.AddError(
			"Workspace settings ID mismatch",
			fmt.Sprintf("GET /api/v1/settings returned tenant id %q, but state has %q. Import or move this resource to match the provider workspace.", api.ID, data.ID.ValueString()),
		)
		return diags
	}

	mapSettingsTenantToResourceModel(data, &api)
	data.TenantHandle = types.StringValue(tenantHandleStringFromAPI(api.TenantHandle))
	return diags
}

func (r *SettingsResource) readSettingsRaw(ctx context.Context, api *settingsTenantAPIResponse) error {
	return r.client.Get(ctx, "/api/v1/settings", nil, api)
}

func mapSettingsTenantToResourceModel(data *SettingsResourceModel, api *settingsTenantAPIResponse) {
	data.ID = types.StringValue(api.ID)
	data.DisplayName = types.StringValue(api.DisplayName)
	data.CreatedAt = types.StringValue(api.CreatedAt)
}

func tenantHandleStringFromAPI(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

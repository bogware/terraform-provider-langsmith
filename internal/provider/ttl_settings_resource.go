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
	_ resource.Resource                = &TTLSettingsResource{}
	_ resource.ResourceWithImportState = &TTLSettingsResource{}
)

// NewTTLSettingsResource returns a new TTLSettingsResource for managing how long
// traces stick around -- like deciding how many seasons of Gunsmoke reruns
// the Long Branch keeps on the shelf.
func NewTTLSettingsResource() resource.Resource {
	return &TTLSettingsResource{}
}

// TTLSettingsResource manages workspace trace retention (TTL) settings in
// LangSmith. This is a singleton resource: one per workspace, always present,
// never truly created or destroyed -- much like the jail in Dodge City.
type TTLSettingsResource struct {
	client *client.Client
}

// TTLSettingsResourceModel holds the Terraform state for TTL settings.
type TTLSettingsResourceModel struct {
	ID               types.String `tfsdk:"id"`
	LonglivedTTLDays types.Int64  `tfsdk:"longlived_ttl_days"`
	IsCustom         types.Bool   `tfsdk:"is_custom"`
	TenantID         types.String `tfsdk:"tenant_id"`
}

// ttlSettingsUpdateRequest is the request body for updating TTL settings --
// just the number of days, plain and simple like a wanted poster.
type ttlSettingsUpdateRequest struct {
	LonglivedTTLDays int64 `json:"longlived_ttl_days"`
}

// ttlSettingsAPIResponse is what the API returns when you ask about TTL settings.
// Includes the tenant_id and whether a custom policy has been set.
type ttlSettingsAPIResponse struct {
	LonglivedTTLDays int64  `json:"longlived_ttl_days"`
	IsCustom         bool   `json:"is_custom"`
	TenantID         string `json:"tenant_id"`
}

func (r *TTLSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ttl_settings"
}

func (r *TTLSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages LangSmith workspace trace retention (TTL) settings. This is a singleton resource that always exists per workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the TTL settings (set to the tenant ID).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"longlived_ttl_days": schema.Int64Attribute{
				MarkdownDescription: "The number of days to retain longlived traces.",
				Required:            true,
			},
			"is_custom": schema.BoolAttribute{
				MarkdownDescription: "Whether a custom TTL policy is configured.",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The workspace tenant ID.",
				Computed:            true,
			},
		},
	}
}

func (r *TTLSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TTLSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TTLSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TTL settings always exist -- like the marshal's office, they're
	// part of the town whether you built them or not. So "create" is
	// really just laying down the law with a PUT.
	body := ttlSettingsUpdateRequest{
		LonglivedTTLDays: data.LonglivedTTLDays.ValueInt64(),
	}

	err := r.client.Put(ctx, "/workspaces/current/ttl-settings", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating TTL settings", err.Error())
		return
	}

	// Read back the settings to get the full picture, including
	// tenant_id and is_custom -- the details the PUT won't tell you.
	r.readTTLSettings(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created TTL settings resource", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TTLSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TTLSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readTTLSettings(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TTLSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TTLSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := ttlSettingsUpdateRequest{
		LonglivedTTLDays: data.LonglivedTTLDays.ValueInt64(),
	}

	err := r.client.Put(ctx, "/workspaces/current/ttl-settings", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating TTL settings", err.Error())
		return
	}

	// Read back the updated settings, same as after Create.
	r.readTTLSettings(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "updated TTL settings resource", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TTLSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// You can't truly delete TTL settings any more than you can tear
	// down the jail in Dodge City. We'll just tip our hat and ride on.
	tflog.Warn(ctx, "TTL settings are a singleton resource and cannot be deleted. Removing from Terraform state only.")
}

func (r *TTLSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readTTLSettings fetches the current TTL settings from the API and maps
// them into the Terraform state model. Shared between Create, Read, and
// Update -- because a good deputy doesn't repeat himself when one
// explanation will do.
func (r *TTLSettingsResource) readTTLSettings(ctx context.Context, data *TTLSettingsResourceModel, diags *diag.Diagnostics) {
	var result ttlSettingsAPIResponse
	err := r.client.Get(ctx, "/workspaces/current/ttl-settings", nil, &result)
	if err != nil {
		diags.AddError("Error reading TTL settings", err.Error())
		return
	}

	mapTTLSettingsResponseToState(data, &result)
}

// mapTTLSettingsResponseToState brands the Terraform state with values from the
// API response -- straightforward enough that even Festus could follow along.
func mapTTLSettingsResponseToState(data *TTLSettingsResourceModel, result *ttlSettingsAPIResponse) {
	data.ID = types.StringValue(result.TenantID)
	data.LonglivedTTLDays = types.Int64Value(result.LonglivedTTLDays)
	data.IsCustom = types.BoolValue(result.IsCustom)
	data.TenantID = types.StringValue(result.TenantID)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

// TTLSettingsResource manages org-level trace retention (TTL) settings in
// LangSmith. This is a singleton resource: one per org, always present,
// never truly created or destroyed -- much like the jail in Dodge City.
type TTLSettingsResource struct {
	client *client.Client
}

// TTLSettingsResourceModel holds the Terraform state for TTL settings.
type TTLSettingsResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	DefaultTraceTier   types.String `tfsdk:"default_trace_tier"`
	ApplyToAllProjects types.Bool   `tfsdk:"apply_to_all_projects"`
	TenantID           types.String `tfsdk:"tenant_id"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	ConfiguredBy       types.String `tfsdk:"configured_by"`
	LonglivedTTLDays   types.Int64  `tfsdk:"longlived_ttl_days"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

// ttlSettingsUpsertRequest is the request body for upserting TTL settings.
type ttlSettingsUpsertRequest struct {
	DefaultTraceTier   string  `json:"default_trace_tier"`
	TenantID           *string `json:"tenant_id,omitempty"`
	ApplyToAllProjects bool    `json:"apply_to_all_projects"`
}

// ttlSettingsAPIResponse is what the API returns when you ask about TTL settings.
type ttlSettingsAPIResponse struct {
	ID                 string  `json:"id"`
	TenantID           *string `json:"tenant_id"`
	DefaultTraceTier   string  `json:"default_trace_tier"`
	ApplyToAllProjects bool    `json:"apply_to_all_projects"`
	OrganizationID     string  `json:"organization_id"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	ConfiguredBy       string  `json:"configured_by"`
	LonglivedTTLDays   *int64  `json:"longlived_ttl_days"`
}

func (r *TTLSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ttl_settings"
}

func (r *TTLSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages LangSmith organization trace retention (TTL) settings. This is a singleton resource that configures the default trace tier for an organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the TTL settings.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"default_trace_tier": schema.StringAttribute{
				MarkdownDescription: "The default trace retention tier. Valid values: `longlived`, `shortlived`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("longlived", "shortlived")},
			},
			"apply_to_all_projects": schema.BoolAttribute{
				MarkdownDescription: "Whether to apply TTL settings to all projects.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant (workspace) ID to scope the TTL settings to. If omitted, applies at the org level.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns these TTL settings.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"configured_by": schema.StringAttribute{
				MarkdownDescription: "Who configured the settings: `system` or `user`.",
				Computed:            true,
			},
			"longlived_ttl_days": schema.Int64Attribute{
				MarkdownDescription: "The number of days longlived traces are retained.",
				Computed:            true,
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

	body := ttlSettingsUpsertRequest{
		DefaultTraceTier:   data.DefaultTraceTier.ValueString(),
		ApplyToAllProjects: data.ApplyToAllProjects.ValueBool(),
	}

	if !data.TenantID.IsNull() && !data.TenantID.IsUnknown() {
		v := data.TenantID.ValueString()
		body.TenantID = &v
	}

	var result ttlSettingsAPIResponse
	err := effectiveClient(r.client, data.TenantID).Put(ctx, "/api/v1/orgs/ttl-settings", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating TTL settings", err.Error())
		return
	}

	mapTTLSettingsResponseToState(&data, &result, &resp.Diagnostics)
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

	if data.ID.IsNull() {
		resp.State.RemoveResource(ctx)
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

	body := ttlSettingsUpsertRequest{
		DefaultTraceTier:   data.DefaultTraceTier.ValueString(),
		ApplyToAllProjects: data.ApplyToAllProjects.ValueBool(),
	}

	if !data.TenantID.IsNull() && !data.TenantID.IsUnknown() {
		v := data.TenantID.ValueString()
		body.TenantID = &v
	}

	var result ttlSettingsAPIResponse
	err := effectiveClient(r.client, data.TenantID).Put(ctx, "/api/v1/orgs/ttl-settings", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating TTL settings", err.Error())
		return
	}

	mapTTLSettingsResponseToState(&data, &result, &resp.Diagnostics)
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

// readTTLSettings fetches the current TTL settings from the API. The GET
// endpoint returns a list, so we find the matching entry by ID.
func (r *TTLSettingsResource) readTTLSettings(ctx context.Context, data *TTLSettingsResourceModel, diags *diag.Diagnostics) {
	var results []ttlSettingsAPIResponse
	err := effectiveClient(r.client, data.TenantID).Get(ctx, "/api/v1/orgs/ttl-settings", nil, &results)
	if err != nil {
		if client.IsNotFound(err) {
			data.ID = types.StringNull()
			return
		}
		diags.AddError("Error reading TTL settings", err.Error())
		return
	}

	if len(results) == 0 {
		diags.AddError("Error reading TTL settings", "No TTL settings found")
		return
	}

	// Find the matching entry by ID, or fall back to the first entry
	// for singleton-style usage.
	var found *ttlSettingsAPIResponse
	for i := range results {
		if results[i].ID == data.ID.ValueString() {
			found = &results[i]
			break
		}
	}
	if found == nil {
		found = &results[0]
	}

	mapTTLSettingsResponseToState(data, found, diags)
}

// mapTTLSettingsResponseToState brands the Terraform state with values from the
// API response -- straightforward enough that even Festus could follow along.
func mapTTLSettingsResponseToState(data *TTLSettingsResourceModel, result *ttlSettingsAPIResponse, diags *diag.Diagnostics) {
	data.ID = types.StringValue(result.ID)
	data.DefaultTraceTier = types.StringValue(result.DefaultTraceTier)
	data.ApplyToAllProjects = types.BoolValue(result.ApplyToAllProjects)
	data.OrganizationID = types.StringValue(result.OrganizationID)
	data.ConfiguredBy = types.StringValue(result.ConfiguredBy)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	apiTenantID := ""
	if result.TenantID != nil {
		apiTenantID = *result.TenantID
	}
	if apiTenantID != "" {
		reconcileTenantID(&data.TenantID, apiTenantID, diags)
	}

	if result.LonglivedTTLDays != nil {
		data.LonglivedTTLDays = types.Int64Value(*result.LonglivedTTLDays)
	} else {
		data.LonglivedTTLDays = types.Int64Null()
	}
}

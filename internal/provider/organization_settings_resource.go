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
	_ resource.Resource                = &OrganizationSettingsResource{}
	_ resource.ResourceWithImportState = &OrganizationSettingsResource{}
)

// NewOrganizationSettingsResource returns a resource for managing settings on
// the current organization.
func NewOrganizationSettingsResource() resource.Resource {
	return &OrganizationSettingsResource{}
}

// OrganizationSettingsResource manages settings of the organization the API
// key belongs to. This is an org-scoped singleton: creating it adopts the
// existing organization, and destroying it only removes it from state.
type OrganizationSettingsResource struct {
	client *client.Client
}

// OrganizationSettingsResourceModel maps the Terraform schema for organization settings.
type OrganizationSettingsResourceModel struct {
	ID                           types.String  `tfsdk:"id"`
	DisplayName                  types.String  `tfsdk:"display_name"`
	PublicSharingDisabled        types.Bool    `tfsdk:"public_sharing_disabled"`
	PatCreationDisabled          types.Bool    `tfsdk:"pat_creation_disabled"`
	JitProvisioningEnabled       types.Bool    `tfsdk:"jit_provisioning_enabled"`
	InvitesEnabled               types.Bool    `tfsdk:"invites_enabled"`
	WorkspaceAdminCanInviteToOrg types.Bool    `tfsdk:"workspace_admin_can_invite_to_org"`
	MaxAPIKeyExpiryDays          types.Int64   `tfsdk:"max_api_key_expiry_days"`
	MaxPatExpiryDays             types.Int64   `tfsdk:"max_pat_expiry_days"`
	MaxServiceKeyExpiryDays      types.Int64   `tfsdk:"max_service_key_expiry_days"`
	SecurityContact              types.String  `tfsdk:"security_contact"`
	ScimGroupNameSeparator       types.String  `tfsdk:"scim_group_name_separator"`
	IPAllowlist                  types.List    `tfsdk:"ip_allowlist"`
	DisabledModelProviders       types.List    `tfsdk:"disabled_model_providers"`
	LLMAuthProxyEnabled          types.Bool    `tfsdk:"llm_auth_proxy_enabled"`
	LLMAuthProxyJWTAudience      types.String  `tfsdk:"llm_auth_proxy_jwt_audience"`
	LLMAuthProxyAllowedURLs      types.List    `tfsdk:"llm_auth_proxy_allowed_urls"`
	RestrictBrowserSecrets       types.Bool    `tfsdk:"restrict_browser_secrets"`
	BYOCCreateSaaSWorkspace      types.Bool    `tfsdk:"byoc_create_saas_workspace_enabled"`
	EngineEnabled                types.Bool    `tfsdk:"engine_enabled"`
	EngineLCUSpendLimitMonthly   types.Float64 `tfsdk:"engine_lcu_spend_limit_monthly"`
	SsoOnly                      types.Bool    `tfsdk:"sso_only"`
	IPAllowlistEnabled           types.Bool    `tfsdk:"ip_allowlist_enabled"`
	SsoLoginSlug                 types.String  `tfsdk:"sso_login_slug"`
	IsPersonal                   types.Bool    `tfsdk:"is_personal"`
}

// organizationUpdateRequest is sent to PATCH /api/v1/orgs/current/info.
// Only fields explicitly configured by the user are included.
type organizationUpdateRequest struct {
	DisplayName                  *string   `json:"display_name,omitempty"`
	PublicSharingDisabled        *bool     `json:"public_sharing_disabled,omitempty"`
	PatCreationDisabled          *bool     `json:"pat_creation_disabled,omitempty"`
	JitProvisioningEnabled       *bool     `json:"jit_provisioning_enabled,omitempty"`
	InvitesEnabled               *bool     `json:"invites_enabled,omitempty"`
	WorkspaceAdminCanInviteToOrg *bool     `json:"workspace_admin_can_invite_to_org,omitempty"`
	MaxAPIKeyExpiryDays          *int64    `json:"max_api_key_expiry_days,omitempty"`
	MaxPatExpiryDays             *int64    `json:"max_pat_expiry_days,omitempty"`
	MaxServiceKeyExpiryDays      *int64    `json:"max_service_key_expiry_days,omitempty"`
	SecurityContact              *string   `json:"security_contact,omitempty"`
	ScimGroupNameSeparator       *string   `json:"scim_group_name_separator,omitempty"`
	IPAllowlist                  *[]string `json:"ip_allowlist,omitempty"`
	DisabledModelProviders       *[]string `json:"disabled_model_providers,omitempty"`
	LLMAuthProxyEnabled          *bool     `json:"llm_auth_proxy_enabled,omitempty"`
	LLMAuthProxyJWTAudience      *string   `json:"llm_auth_proxy_jwt_audience,omitempty"`
	LLMAuthProxyAllowedURLs      *[]string `json:"llm_auth_proxy_allowed_urls,omitempty"`
	RestrictBrowserSecrets       *bool     `json:"restrict_browser_secrets,omitempty"`
	BYOCCreateSaaSWorkspace      *bool     `json:"byoc_create_saas_workspace_enabled,omitempty"`
	EngineEnabled                *bool     `json:"engine_enabled,omitempty"`
	EngineLCUSpendLimitMonthly   *float64  `json:"engine_lcu_spend_limit_monthly,omitempty"`
}

// allowedLoginMethodsUpdateRequest is sent to PATCH /api/v1/orgs/current/login-methods.
type allowedLoginMethodsUpdateRequest struct {
	SsoOnly *bool `json:"sso_only,omitempty"`
}

// organizationInfoAPIResponse is the OrganizationInfo shape returned by the API
// (billing and feature-flag fields are intentionally omitted).
type organizationInfoAPIResponse struct {
	ID                           *string  `json:"id"`
	DisplayName                  *string  `json:"display_name"`
	IsPersonal                   bool     `json:"is_personal"`
	SsoOnly                      bool     `json:"sso_only"`
	SsoLoginSlug                 *string  `json:"sso_login_slug"`
	JitProvisioningEnabled       bool     `json:"jit_provisioning_enabled"`
	InvitesEnabled               bool     `json:"invites_enabled"`
	PublicSharingDisabled        bool     `json:"public_sharing_disabled"`
	PatCreationDisabled          bool     `json:"pat_creation_disabled"`
	WorkspaceAdminCanInviteToOrg bool     `json:"workspace_admin_can_invite_to_org"`
	MaxAPIKeyExpiryDays          *int64   `json:"max_api_key_expiry_days"`
	MaxPatExpiryDays             *int64   `json:"max_pat_expiry_days"`
	MaxServiceKeyExpiryDays      *int64   `json:"max_service_key_expiry_days"`
	SecurityContact              *string  `json:"security_contact"`
	ScimGroupNameSeparator       string   `json:"scim_group_name_separator"`
	IPAllowlist                  []string `json:"ip_allowlist"`
	DisabledModelProviders       []string `json:"disabled_model_providers"`
	LLMAuthProxyEnabled          bool     `json:"llm_auth_proxy_enabled"`
	LLMAuthProxyJWTAudience      *string  `json:"llm_auth_proxy_jwt_audience"`
	LLMAuthProxyAllowedURLs      []string `json:"llm_auth_proxy_allowed_urls"`
	RestrictBrowserSecrets       bool     `json:"restrict_browser_secrets"`
	BYOCCreateSaaSWorkspace      bool     `json:"byoc_create_saas_workspace_enabled"`
	EngineEnabled                bool     `json:"engine_enabled"`
	EngineLCUSpendLimitMonthly   *float64 `json:"engine_lcu_spend_limit_monthly"`
	IPAllowlistEnabled           bool     `json:"ip_allowlist_enabled"`
}

func (r *OrganizationSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_settings"
}

func (r *OrganizationSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages settings of the current LangSmith organization (the organization the API key belongs to). This is an org-scoped singleton resource: creating it adopts the existing organization and applies any configured settings; " +
			"destroying it only removes it from Terraform state — the organization itself cannot be deleted via this resource. Only fields explicitly set in configuration are sent to the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "UUID of the organization.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the organization. May contain letters, digits, spaces, hyphens, and underscores.",
				Optional:            true,
				Computed:            true,
			},
			"public_sharing_disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether public sharing of traces, datasets, and prompts is disabled org-wide.",
				Optional:            true,
				Computed:            true,
			},
			"pat_creation_disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether creation of personal access tokens is disabled org-wide.",
				Optional:            true,
				Computed:            true,
			},
			"jit_provisioning_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether just-in-time user provisioning is enabled.",
				Optional:            true,
				Computed:            true,
			},
			"invites_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether member invites are enabled.",
				Optional:            true,
				Computed:            true,
			},
			"workspace_admin_can_invite_to_org": schema.BoolAttribute{
				MarkdownDescription: "Whether workspace admins may invite users to the organization.",
				Optional:            true,
				Computed:            true,
			},
			"max_api_key_expiry_days": schema.Int64Attribute{
				MarkdownDescription: "Maximum allowed expiry (in days) for newly created API keys.",
				Optional:            true,
				Computed:            true,
			},
			"max_pat_expiry_days": schema.Int64Attribute{
				MarkdownDescription: "Maximum allowed expiry (in days) for newly created personal access tokens.",
				Optional:            true,
				Computed:            true,
			},
			"max_service_key_expiry_days": schema.Int64Attribute{
				MarkdownDescription: "Maximum allowed expiry (in days) for newly created service keys.",
				Optional:            true,
				Computed:            true,
			},
			"security_contact": schema.StringAttribute{
				MarkdownDescription: "Email address of the organization's security contact.",
				Optional:            true,
				Computed:            true,
			},
			"scim_group_name_separator": schema.StringAttribute{
				MarkdownDescription: "Single-character separator used when parsing SCIM group names (defaults to `:`).",
				Optional:            true,
				Computed:            true,
			},
			"ip_allowlist": schema.ListAttribute{
				MarkdownDescription: "List of CIDR ranges allowed to access the organization (requires the IP allowlist feature).",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"disabled_model_providers": schema.ListAttribute{
				MarkdownDescription: "Model providers that are disabled organization-wide.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"llm_auth_proxy_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the LLM auth proxy is enabled for the organization.",
				Optional:            true,
				Computed:            true,
			},
			"llm_auth_proxy_jwt_audience": schema.StringAttribute{
				MarkdownDescription: "Expected `aud` claim for JWTs presented to the LLM auth proxy.",
				Optional:            true,
				Computed:            true,
			},
			"llm_auth_proxy_allowed_urls": schema.ListAttribute{
				MarkdownDescription: "URLs the LLM auth proxy is permitted to forward requests to.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"restrict_browser_secrets": schema.BoolAttribute{
				MarkdownDescription: "Whether workspace secrets are withheld from the browser client.",
				Optional:            true,
				Computed:            true,
			},
			"byoc_create_saas_workspace_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether a BYOC organization may also create SaaS-hosted workspaces.",
				Optional:            true,
				Computed:            true,
			},
			"engine_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the LangSmith engine is enabled for the organization.",
				Optional:            true,
				Computed:            true,
			},
			"engine_lcu_spend_limit_monthly": schema.Float64Attribute{
				MarkdownDescription: "Monthly engine spend limit, in LCUs. Null means no limit.",
				Optional:            true,
				Computed:            true,
			},
			"sso_only": schema.BoolAttribute{
				MarkdownDescription: "Whether SSO is the only allowed login method for the organization. Managed via the login-methods endpoint.",
				Optional:            true,
				Computed:            true,
			},
			"ip_allowlist_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the IP allowlist is currently enforced.",
				Computed:            true,
			},
			"sso_login_slug": schema.StringAttribute{
				MarkdownDescription: "The SSO login slug of the organization, if configured.",
				Computed:            true,
			},
			"is_personal": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a personal organization.",
				Computed:            true,
			},
		},
	}
}

func (r *OrganizationSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// applyOrganizationSettings PATCHes the org info and login methods based on
// what is explicitly set in configuration, then re-reads the org so all
// computed attributes reflect the final server state.
func (r *OrganizationSettingsResource) applyOrganizationSettings(ctx context.Context, config *OrganizationSettingsResourceModel, data *OrganizationSettingsResourceModel, diags *diag.Diagnostics) {
	body, hasUpdates := orgUpdateFromConfig(ctx, config, diags)
	if diags.HasError() {
		return
	}

	if hasUpdates {
		if err := r.client.Patch(ctx, "/api/v1/orgs/current/info", body, nil); err != nil {
			diags.AddError("Error updating organization settings", err.Error())
			return
		}
	}

	if !config.SsoOnly.IsNull() && !config.SsoOnly.IsUnknown() {
		v := config.SsoOnly.ValueBool()
		loginBody := allowedLoginMethodsUpdateRequest{SsoOnly: &v}
		if err := r.client.Patch(ctx, "/api/v1/orgs/current/login-methods", loginBody, nil); err != nil {
			diags.AddError("Error updating organization login methods", err.Error())
			return
		}
	}

	var result organizationInfoAPIResponse
	if err := r.client.Get(ctx, "/api/v1/orgs/current/info", nil, &result); err != nil {
		diags.AddError("Error reading organization settings", err.Error())
		return
	}
	mapOrganizationInfoToState(ctx, data, &result, diags)
}

func (r *OrganizationSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data, config OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyOrganizationSettings(ctx, &config, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "adopted organization settings", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result organizationInfoAPIResponse
	if err := r.client.Get(ctx, "/api/v1/orgs/current/info", nil, &result); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading organization settings", err.Error())
		return
	}

	mapOrganizationInfoToState(ctx, &data, &result, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, config OrganizationSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyOrganizationSettings(ctx, &config, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Organizations cannot be deleted through this resource; deleting only
	// removes the settings from Terraform state.
	resp.Diagnostics.AddWarning(
		"Organization not deleted in LangSmith",
		"langsmith_organization_settings manages settings on an existing organization; destroying it removes the resource from Terraform state only. The organization and its current settings are left unchanged.",
	)
	tflog.Warn(ctx, "organization settings are a singleton resource and cannot be deleted; removing from Terraform state only")
}

func (r *OrganizationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Singleton: the import ID is the organization UUID, but the value is
	// overwritten from the API on the first refresh.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// orgUpdateFromConfig builds the PATCH body from explicitly configured
// attributes only, so unset fields are never sent (and never clobbered).
func orgUpdateFromConfig(ctx context.Context, config *OrganizationSettingsResourceModel, diags *diag.Diagnostics) (organizationUpdateRequest, bool) {
	var body organizationUpdateRequest
	hasUpdates := false

	setString := func(v types.String, dst **string) {
		if !v.IsNull() && !v.IsUnknown() {
			s := v.ValueString()
			*dst = &s
			hasUpdates = true
		}
	}
	setBool := func(v types.Bool, dst **bool) {
		if !v.IsNull() && !v.IsUnknown() {
			b := v.ValueBool()
			*dst = &b
			hasUpdates = true
		}
	}
	setInt := func(v types.Int64, dst **int64) {
		if !v.IsNull() && !v.IsUnknown() {
			i := v.ValueInt64()
			*dst = &i
			hasUpdates = true
		}
	}

	setString(config.DisplayName, &body.DisplayName)
	setBool(config.PublicSharingDisabled, &body.PublicSharingDisabled)
	setBool(config.PatCreationDisabled, &body.PatCreationDisabled)
	setBool(config.JitProvisioningEnabled, &body.JitProvisioningEnabled)
	setBool(config.InvitesEnabled, &body.InvitesEnabled)
	setBool(config.WorkspaceAdminCanInviteToOrg, &body.WorkspaceAdminCanInviteToOrg)
	setInt(config.MaxAPIKeyExpiryDays, &body.MaxAPIKeyExpiryDays)
	setInt(config.MaxPatExpiryDays, &body.MaxPatExpiryDays)
	setInt(config.MaxServiceKeyExpiryDays, &body.MaxServiceKeyExpiryDays)
	setString(config.SecurityContact, &body.SecurityContact)
	setString(config.ScimGroupNameSeparator, &body.ScimGroupNameSeparator)

	setBool(config.LLMAuthProxyEnabled, &body.LLMAuthProxyEnabled)
	setString(config.LLMAuthProxyJWTAudience, &body.LLMAuthProxyJWTAudience)
	setBool(config.RestrictBrowserSecrets, &body.RestrictBrowserSecrets)
	setBool(config.BYOCCreateSaaSWorkspace, &body.BYOCCreateSaaSWorkspace)
	setBool(config.EngineEnabled, &body.EngineEnabled)
	if !config.EngineLCUSpendLimitMonthly.IsNull() && !config.EngineLCUSpendLimitMonthly.IsUnknown() {
		f := config.EngineLCUSpendLimitMonthly.ValueFloat64()
		body.EngineLCUSpendLimitMonthly = &f
		hasUpdates = true
	}

	setList := func(v types.List, dst **[]string) bool {
		if v.IsNull() || v.IsUnknown() {
			return true
		}
		var list []string
		diags.Append(v.ElementsAs(ctx, &list, false)...)
		if diags.HasError() {
			return false
		}
		*dst = &list
		hasUpdates = true
		return true
	}

	if !setList(config.IPAllowlist, &body.IPAllowlist) ||
		!setList(config.DisabledModelProviders, &body.DisabledModelProviders) ||
		!setList(config.LLMAuthProxyAllowedURLs, &body.LLMAuthProxyAllowedURLs) {
		return body, false
	}

	return body, hasUpdates
}

// mapOrganizationInfoToState copies the API organization info onto the
// Terraform state model.
func mapOrganizationInfoToState(ctx context.Context, data *OrganizationSettingsResourceModel, result *organizationInfoAPIResponse, diags *diag.Diagnostics) {
	if result.ID != nil {
		data.ID = types.StringValue(*result.ID)
	} else {
		data.ID = types.StringNull()
	}
	if result.DisplayName != nil {
		data.DisplayName = types.StringValue(*result.DisplayName)
	} else {
		data.DisplayName = types.StringNull()
	}
	data.PublicSharingDisabled = types.BoolValue(result.PublicSharingDisabled)
	data.PatCreationDisabled = types.BoolValue(result.PatCreationDisabled)
	data.JitProvisioningEnabled = types.BoolValue(result.JitProvisioningEnabled)
	data.InvitesEnabled = types.BoolValue(result.InvitesEnabled)
	data.WorkspaceAdminCanInviteToOrg = types.BoolValue(result.WorkspaceAdminCanInviteToOrg)
	if result.MaxAPIKeyExpiryDays != nil {
		data.MaxAPIKeyExpiryDays = types.Int64Value(*result.MaxAPIKeyExpiryDays)
	} else {
		data.MaxAPIKeyExpiryDays = types.Int64Null()
	}
	if result.MaxPatExpiryDays != nil {
		data.MaxPatExpiryDays = types.Int64Value(*result.MaxPatExpiryDays)
	} else {
		data.MaxPatExpiryDays = types.Int64Null()
	}
	if result.MaxServiceKeyExpiryDays != nil {
		data.MaxServiceKeyExpiryDays = types.Int64Value(*result.MaxServiceKeyExpiryDays)
	} else {
		data.MaxServiceKeyExpiryDays = types.Int64Null()
	}
	if result.SecurityContact != nil {
		data.SecurityContact = types.StringValue(*result.SecurityContact)
	} else {
		data.SecurityContact = types.StringNull()
	}
	data.ScimGroupNameSeparator = types.StringValue(result.ScimGroupNameSeparator)

	ipAllowlist, d := types.ListValueFrom(ctx, types.StringType, result.IPAllowlist)
	diags.Append(d...)
	data.IPAllowlist = ipAllowlist

	disabledProviders, d := types.ListValueFrom(ctx, types.StringType, result.DisabledModelProviders)
	diags.Append(d...)
	data.DisabledModelProviders = disabledProviders

	proxyURLs, d := types.ListValueFrom(ctx, types.StringType, result.LLMAuthProxyAllowedURLs)
	diags.Append(d...)
	data.LLMAuthProxyAllowedURLs = proxyURLs

	data.LLMAuthProxyEnabled = types.BoolValue(result.LLMAuthProxyEnabled)
	if result.LLMAuthProxyJWTAudience != nil {
		data.LLMAuthProxyJWTAudience = types.StringValue(*result.LLMAuthProxyJWTAudience)
	} else {
		data.LLMAuthProxyJWTAudience = types.StringNull()
	}
	data.RestrictBrowserSecrets = types.BoolValue(result.RestrictBrowserSecrets)
	data.BYOCCreateSaaSWorkspace = types.BoolValue(result.BYOCCreateSaaSWorkspace)
	data.EngineEnabled = types.BoolValue(result.EngineEnabled)
	if result.EngineLCUSpendLimitMonthly != nil {
		data.EngineLCUSpendLimitMonthly = types.Float64Value(*result.EngineLCUSpendLimitMonthly)
	} else {
		data.EngineLCUSpendLimitMonthly = types.Float64Null()
	}

	data.SsoOnly = types.BoolValue(result.SsoOnly)
	data.IPAllowlistEnabled = types.BoolValue(result.IPAllowlistEnabled)
	if result.SsoLoginSlug != nil {
		data.SsoLoginSlug = types.StringValue(*result.SsoLoginSlug)
	} else {
		data.SsoLoginSlug = types.StringNull()
	}
	data.IsPersonal = types.BoolValue(result.IsPersonal)
}

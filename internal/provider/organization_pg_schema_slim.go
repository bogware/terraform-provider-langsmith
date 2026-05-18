// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// organizationPGSchemaSlimAPI matches OpenAPI `OrganizationPGSchemaSlim` (list views).
type organizationPGSchemaSlimAPI struct {
	ID                           string   `json:"id"`
	DisplayName                  string   `json:"display_name"`
	Tier                         *string  `json:"tier"`
	CreatedAt                    *string  `json:"created_at"`
	CreatedByUserID              *string  `json:"created_by_user_id"`
	CreatedByLsUserID            *string  `json:"created_by_ls_user_id"`
	ModifiedAt                   *string  `json:"modified_at"`
	IsPersonal                   bool     `json:"is_personal"`
	Disabled                     bool     `json:"disabled"`
	SsoLoginSlug                 *string  `json:"sso_login_slug"`
	SsoOnly                      *bool    `json:"sso_only"`
	JitProvisioningEnabled       *bool    `json:"jit_provisioning_enabled"`
	InvitesEnabled               *bool    `json:"invites_enabled"`
	PublicSharingDisabled        *bool    `json:"public_sharing_disabled"`
	PatCreationDisabled          *bool    `json:"pat_creation_disabled"`
	WorkspaceAdminCanInviteToOrg *bool    `json:"workspace_admin_can_invite_to_org"`
	DefaultSsoProvision          *bool    `json:"default_sso_provision"`
	MaxAPIKeyExpiryDays          *int     `json:"max_api_key_expiry_days"`
	SecurityContact              *string  `json:"security_contact"`
	MaxPatExpiryDays             *int     `json:"max_pat_expiry_days"`
	MaxServiceKeyExpiryDays      *int     `json:"max_service_key_expiry_days"`
	ScimGroupNameSeparator       *string  `json:"scim_group_name_separator"`
	LlmAuthProxyEnabled          *bool    `json:"llm_auth_proxy_enabled"`
	LlmAuthProxyJwtAudience      *string  `json:"llm_auth_proxy_jwt_audience"`
	IpAllowlist                  []string `json:"ip_allowlist"`
	RestrictBrowserSecrets       *bool    `json:"restrict_browser_secrets"`
	LlmAuthProxyAllowedURLs      []string `json:"llm_auth_proxy_allowed_urls"`
	EngineEnabled                *bool    `json:"engine_enabled"`
}

// organizationPGSlimRowAttrTypes is the Terraform object type for each slim org row.
var organizationPGSlimRowAttrTypes = map[string]attr.Type{
	"id":                                types.StringType,
	"display_name":                      types.StringType,
	"tier":                              types.StringType,
	"created_at":                        types.StringType,
	"created_by_user_id":                types.StringType,
	"created_by_ls_user_id":             types.StringType,
	"modified_at":                       types.StringType,
	"is_personal":                       types.BoolType,
	"disabled":                          types.BoolType,
	"sso_login_slug":                    types.StringType,
	"sso_only":                          types.BoolType,
	"jit_provisioning_enabled":          types.BoolType,
	"invites_enabled":                   types.BoolType,
	"public_sharing_disabled":           types.BoolType,
	"pat_creation_disabled":             types.BoolType,
	"workspace_admin_can_invite_to_org": types.BoolType,
	"default_sso_provision":             types.BoolType,
	"max_api_key_expiry_days":           types.Int64Type,
	"security_contact":                  types.StringType,
	"max_pat_expiry_days":               types.Int64Type,
	"max_service_key_expiry_days":       types.Int64Type,
	"scim_group_name_separator":         types.StringType,
	"llm_auth_proxy_enabled":            types.BoolType,
	"llm_auth_proxy_jwt_audience":       types.StringType,
	"ip_allowlist":                      types.ListType{ElemType: types.StringType},
	"restrict_browser_secrets":          types.BoolType,
	"llm_auth_proxy_allowed_urls":       types.ListType{ElemType: types.StringType},
	"engine_enabled":                    types.BoolType,
}

func organizationPGSlimNestedAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Organization UUID.",
		},
		"display_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Display name.",
		},
		"tier": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Plan tier (`PaymentPlanTier` in OpenAPI), if set.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Creation time (RFC3339), if present.",
		},
		"created_by_user_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Creating user id, if present.",
		},
		"created_by_ls_user_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Creating LangSmith user id, if present.",
		},
		"modified_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Last modification time (RFC3339), if present.",
		},
		"is_personal": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether this is a personal organization.",
		},
		"disabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the organization is disabled.",
		},
		"sso_login_slug": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "SSO login slug, if configured.",
		},
		"sso_only": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether SSO-only login is enforced.",
		},
		"jit_provisioning_enabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Just-in-time provisioning flag.",
		},
		"invites_enabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether invites are enabled.",
		},
		"public_sharing_disabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether public sharing is disabled.",
		},
		"pat_creation_disabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether PAT creation is disabled.",
		},
		"workspace_admin_can_invite_to_org": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether workspace admins may invite to the org.",
		},
		"default_sso_provision": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Default SSO provisioning flag.",
		},
		"max_api_key_expiry_days": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Maximum API key expiry in days, if set.",
		},
		"security_contact": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Security contact email, if set.",
		},
		"max_pat_expiry_days": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Maximum PAT expiry in days, if set.",
		},
		"max_service_key_expiry_days": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Maximum service key expiry in days, if set.",
		},
		"scim_group_name_separator": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "SCIM group name separator (API default `:` when absent).",
		},
		"llm_auth_proxy_enabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "LLM auth proxy enabled flag.",
		},
		"llm_auth_proxy_jwt_audience": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "LLM auth proxy JWT audience, if set.",
		},
		"ip_allowlist": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "IP allowlist CIDRs or entries (sorted in state for stable plans).",
		},
		"restrict_browser_secrets": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether browser secret access is restricted.",
		},
		"llm_auth_proxy_allowed_urls": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Allowed URLs for the LLM auth proxy (sorted in state).",
		},
		"engine_enabled": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Engine enabled flag from the API, if present.",
		},
	}
}

func optionalInt64FromInt(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}

func organizationPGSlimObjectValue(row *organizationPGSchemaSlimAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	ip := sortedStringsPreserveDuplicates(row.IpAllowlist)
	ipList, d := types.ListValue(types.StringType, stringSliceToAttrValues(ip))
	diags.Append(d...)

	urls := sortedStringsPreserveDuplicates(row.LlmAuthProxyAllowedURLs)
	urlList, d2 := types.ListValue(types.StringType, stringSliceToAttrValues(urls))
	diags.Append(d2...)

	if diags.HasError() {
		return types.ObjectNull(organizationPGSlimRowAttrTypes), diags
	}

	obj := types.ObjectValueMust(organizationPGSlimRowAttrTypes, map[string]attr.Value{
		"id":                                types.StringValue(row.ID),
		"display_name":                      types.StringValue(row.DisplayName),
		"tier":                              types.StringPointerValue(row.Tier),
		"created_at":                        types.StringPointerValue(row.CreatedAt),
		"created_by_user_id":                types.StringPointerValue(row.CreatedByUserID),
		"created_by_ls_user_id":             types.StringPointerValue(row.CreatedByLsUserID),
		"modified_at":                       types.StringPointerValue(row.ModifiedAt),
		"is_personal":                       types.BoolValue(row.IsPersonal),
		"disabled":                          types.BoolValue(row.Disabled),
		"sso_login_slug":                    types.StringPointerValue(row.SsoLoginSlug),
		"sso_only":                          boolPointerAttrValue(row.SsoOnly),
		"jit_provisioning_enabled":          boolPointerAttrValue(row.JitProvisioningEnabled),
		"invites_enabled":                   boolPointerAttrValue(row.InvitesEnabled),
		"public_sharing_disabled":           boolPointerAttrValue(row.PublicSharingDisabled),
		"pat_creation_disabled":             boolPointerAttrValue(row.PatCreationDisabled),
		"workspace_admin_can_invite_to_org": boolPointerAttrValue(row.WorkspaceAdminCanInviteToOrg),
		"default_sso_provision":             boolPointerAttrValue(row.DefaultSsoProvision),
		"max_api_key_expiry_days":           optionalInt64FromInt(row.MaxAPIKeyExpiryDays),
		"security_contact":                  types.StringPointerValue(row.SecurityContact),
		"max_pat_expiry_days":               optionalInt64FromInt(row.MaxPatExpiryDays),
		"max_service_key_expiry_days":       optionalInt64FromInt(row.MaxServiceKeyExpiryDays),
		"scim_group_name_separator":         types.StringPointerValue(row.ScimGroupNameSeparator),
		"llm_auth_proxy_enabled":            boolPointerAttrValue(row.LlmAuthProxyEnabled),
		"llm_auth_proxy_jwt_audience":       types.StringPointerValue(row.LlmAuthProxyJwtAudience),
		"ip_allowlist":                      ipList,
		"restrict_browser_secrets":          boolPointerAttrValue(row.RestrictBrowserSecrets),
		"llm_auth_proxy_allowed_urls":       urlList,
		"engine_enabled":                    boolPointerAttrValue(row.EngineEnabled),
	})
	return obj, diags
}

func organizationPGSlimListValue(rows []organizationPGSchemaSlimAPI) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	elems := make([]attr.Value, 0, len(rows))
	for i := range rows {
		obj, d := organizationPGSlimObjectValue(&rows[i])
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: organizationPGSlimRowAttrTypes}), diags
		}
		elems = append(elems, obj)
	}

	list, ldiags := types.ListValue(types.ObjectType{AttrTypes: organizationPGSlimRowAttrTypes}, elems)
	diags.Append(ldiags...)
	return list, diags
}

func sortedStringsPreserveDuplicates(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

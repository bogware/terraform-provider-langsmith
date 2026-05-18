// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

const fleetMCPServersAPIPath = "/v1/platform/fleet/mcp-servers"

var (
	_ resource.Resource                   = &FleetMCPServerResource{}
	_ resource.ResourceWithImportState    = &FleetMCPServerResource{}
	_ resource.ResourceWithValidateConfig = &FleetMCPServerResource{}
)

// NewFleetMCPServerResource returns a resource for workspace MCP server
// registrations on the LangSmith platform fleet API.
func NewFleetMCPServerResource() resource.Resource {
	return &FleetMCPServerResource{}
}

// FleetMCPServerResource manages POST/GET/PATCH/DELETE on
// `/v1/platform/fleet/mcp-servers` and `/v1/platform/fleet/mcp-servers/{id}`.
type FleetMCPServerResource struct {
	client *client.Client
}

// FleetMCPServerResourceModel is Terraform state for langsmith_fleet_mcp_server.
type FleetMCPServerResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	URL              types.String `tfsdk:"url"`
	AuthType         types.String `tfsdk:"auth_type"`
	VendorID         types.String `tfsdk:"vendor_id"`
	MCPVendorID      types.String `tfsdk:"mcp_vendor_id"`
	ExternalSystemID types.String `tfsdk:"external_system_id"`
	OAuthMode        types.String `tfsdk:"oauth_mode"`
	OAuthProviderID  types.String `tfsdk:"oauth_provider_id"`
	Headers          types.String `tfsdk:"headers"`
	CanInvoke        types.Bool   `tfsdk:"can_invoke"`
	TenantID         types.String `tfsdk:"tenant_id"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

type mcpServerAPI struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	URL              string          `json:"url"`
	AuthType         *string         `json:"auth_type,omitempty"`
	VendorID         *string         `json:"vendor_id,omitempty"`
	MCPVendorID      *string         `json:"mcp_vendor_id,omitempty"`
	ExternalSystemID *string         `json:"external_system_id,omitempty"`
	OAuthMode        *string         `json:"oauth_mode,omitempty"`
	OAuthProviderID  *string         `json:"oauth_provider_id,omitempty"`
	Headers          json.RawMessage `json:"headers,omitempty"`
	CanInvoke        *bool           `json:"can_invoke,omitempty"`
	TenantID         *string         `json:"tenant_id,omitempty"`
	CreatedAt        *string         `json:"created_at,omitempty"`
	UpdatedAt        *string         `json:"updated_at,omitempty"`
}

type createMcpServerPayload struct {
	Name             string          `json:"name"`
	URL              string          `json:"url"`
	AuthType         *string         `json:"auth_type,omitempty"`
	ExternalSystemID *string         `json:"external_system_id,omitempty"`
	Headers          json.RawMessage `json:"headers,omitempty"`
	OAuthMode        *string         `json:"oauth_mode,omitempty"`
	OAuthProviderID  *string         `json:"oauth_provider_id,omitempty"`
	VendorID         *string         `json:"vendor_id,omitempty"`
}

func (r *FleetMCPServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fleet_mcp_server"
}

func (r *FleetMCPServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers and manages a **fleet MCP server** for the current workspace using the platform API " +
			"([`POST /v1/platform/fleet/mcp-servers`](https://api.smith.langchain.com/openapi.json), `GET`/`PATCH`/`DELETE` `/v1/platform/fleet/mcp-servers/{mcp_server_id}`). " +
			"Only `url`, `auth_type`, `oauth_provider_id`, and `headers` can be changed in place; changing `name`, `vendor_id`, `external_system_id`, or `oauth_mode` forces replacement. " +
			"Set `headers` to a JSON **array** of objects (for example `[{\"Authorization\":\"Bearer …\"}]`) matching the OpenAPI `headers` field.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "MCP server identifier returned by the API.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the MCP server (immutable after create).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "MCP server endpoint URL.",
				Required:            true,
			},
			"auth_type": schema.StringAttribute{
				MarkdownDescription: "Authentication mode: `headers` or `oauth`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("headers", "oauth"),
				},
			},
			"vendor_id": schema.StringAttribute{
				MarkdownDescription: "Optional vendor identifier supplied at registration time.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mcp_vendor_id": schema.StringAttribute{
				MarkdownDescription: "Vendor linkage id materialized by LangSmith (read-only).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"external_system_id": schema.StringAttribute{
				MarkdownDescription: "Optional external system id (create-time only).",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"oauth_mode": schema.StringAttribute{
				MarkdownDescription: "OAuth registration mode: `legacy_shared_provider` or `per_user_dynamic_client` (create-time only).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("legacy_shared_provider", "per_user_dynamic_client"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"oauth_provider_id": schema.StringAttribute{
				MarkdownDescription: "OAuth provider id when `auth_type` is `oauth`.",
				Optional:            true,
			},
			"headers": schema.StringAttribute{
				MarkdownDescription: "JSON array of header objects sent to the MCP server when using `auth_type = \"headers\"`.",
				Optional:            true,
			},
			"can_invoke": schema.BoolAttribute{
				MarkdownDescription: "Whether the current principal may invoke tools on this server (read-only).",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Workspace tenant id owning the registration (read-only).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 creation timestamp from the API.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 last-update timestamp from the API.",
				Computed:            true,
			},
		},
	}
}

func (r *FleetMCPServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FleetMCPServerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data FleetMCPServerResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateFleetMCPServerConfig(&data)...)
}

func (r *FleetMCPServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FleetMCPServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := buildCreateMcpServerPayload(&plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created mcpServerAPI
	if err := r.client.Post(ctx, fleetMCPServersAPIPath, body, &created); err != nil {
		resp.Diagnostics.AddError("Error creating fleet MCP server", err.Error())
		return
	}

	mapMcpServerAPIToModel(&created, &plan)
	tflog.Trace(ctx, "created fleet MCP server", map[string]any{"id": plan.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FleetMCPServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FleetMCPServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rec mcpServerAPI
	apiPath := fleetMCPServersAPIPath + "/" + url.PathEscape(data.ID.ValueString())
	if err := r.client.Get(ctx, apiPath, nil, &rec); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading fleet MCP server", err.Error())
		return
	}

	mapMcpServerAPIToModel(&rec, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FleetMCPServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FleetMCPServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state FleetMCPServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := buildUpdateMcpServerPayload(&plan, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updated mcpServerAPI
	apiPath := fleetMCPServersAPIPath + "/" + url.PathEscape(plan.ID.ValueString())
	if err := r.client.Patch(ctx, apiPath, body, &updated); err != nil {
		resp.Diagnostics.AddError("Error updating fleet MCP server", err.Error())
		return
	}

	mapMcpServerAPIToModel(&updated, &plan)
	tflog.Trace(ctx, "updated fleet MCP server", map[string]any{"id": plan.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FleetMCPServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FleetMCPServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fleetMCPServersAPIPath + "/" + url.PathEscape(data.ID.ValueString())
	if err := r.client.Delete(ctx, apiPath); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting fleet MCP server", err.Error())
		return
	}
	tflog.Trace(ctx, "deleted fleet MCP server", map[string]any{"id": data.ID.ValueString()})
}

func (r *FleetMCPServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildCreateMcpServerPayload(data *FleetMCPServerResourceModel) (createMcpServerPayload, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := createMcpServerPayload{
		Name: data.Name.ValueString(),
		URL:  data.URL.ValueString(),
	}
	if !data.AuthType.IsNull() {
		v := data.AuthType.ValueString()
		body.AuthType = &v
	}
	if !data.VendorID.IsNull() {
		v := data.VendorID.ValueString()
		body.VendorID = &v
	}
	if !data.ExternalSystemID.IsNull() {
		v := data.ExternalSystemID.ValueString()
		body.ExternalSystemID = &v
	}
	if !data.OAuthMode.IsNull() {
		v := data.OAuthMode.ValueString()
		body.OAuthMode = &v
	}
	if !data.OAuthProviderID.IsNull() {
		v := data.OAuthProviderID.ValueString()
		body.OAuthProviderID = &v
	}
	if !data.Headers.IsNull() {
		raw := json.RawMessage(data.Headers.ValueString())
		if err := validateHeadersJSON(raw); err != nil {
			diags.AddError("Invalid headers JSON", err.Error())
			return body, diags
		}
		body.Headers = raw
	}
	return body, diags
}

func buildUpdateMcpServerPayload(plan, state *FleetMCPServerResourceModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	patch := map[string]interface{}{}
	if !plan.URL.Equal(state.URL) {
		patch["url"] = plan.URL.ValueString()
	}
	if !plan.AuthType.Equal(state.AuthType) {
		patch["auth_type"] = stringValueOrNil(plan.AuthType)
	}
	if !plan.OAuthProviderID.Equal(state.OAuthProviderID) {
		patch["oauth_provider_id"] = stringValueOrNil(plan.OAuthProviderID)
	}
	if !plan.Headers.Equal(state.Headers) {
		if plan.Headers.IsNull() {
			patch["headers"] = nil
		} else {
			raw := json.RawMessage(plan.Headers.ValueString())
			if err := validateHeadersJSON(raw); err != nil {
				diags.AddError("Invalid headers JSON", err.Error())
				return nil, diags
			}
			patch["headers"] = raw
		}
	}
	return patch, diags
}

func validateHeadersJSON(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return fmt.Errorf("headers must be a JSON array (use [] to send an empty list)")
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return fmt.Errorf("headers must be a JSON array: %w", err)
	}
	return nil
}

func mapMcpServerAPIToModel(api *mcpServerAPI, data *FleetMCPServerResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.URL = types.StringValue(api.URL)
	if api.AuthType != nil {
		data.AuthType = types.StringValue(*api.AuthType)
	} else {
		data.AuthType = types.StringNull()
	}
	if api.VendorID != nil {
		data.VendorID = types.StringValue(*api.VendorID)
	} else {
		data.VendorID = types.StringNull()
	}
	if api.MCPVendorID != nil {
		data.MCPVendorID = types.StringValue(*api.MCPVendorID)
	} else {
		data.MCPVendorID = types.StringNull()
	}
	if api.ExternalSystemID != nil {
		data.ExternalSystemID = types.StringValue(*api.ExternalSystemID)
	} else {
		data.ExternalSystemID = types.StringNull()
	}
	if api.OAuthMode != nil {
		data.OAuthMode = types.StringValue(*api.OAuthMode)
	} else {
		data.OAuthMode = types.StringNull()
	}
	if api.OAuthProviderID != nil {
		data.OAuthProviderID = types.StringValue(*api.OAuthProviderID)
	} else {
		data.OAuthProviderID = types.StringNull()
	}
	data.Headers = jsonStringValuePreservingEquivalent(api.Headers, data.Headers)
	if api.CanInvoke != nil {
		data.CanInvoke = types.BoolValue(*api.CanInvoke)
	} else {
		data.CanInvoke = types.BoolNull()
	}
	if api.TenantID != nil {
		data.TenantID = types.StringValue(*api.TenantID)
	} else {
		data.TenantID = types.StringNull()
	}
	if api.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*api.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if api.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(*api.UpdatedAt)
	} else {
		data.UpdatedAt = types.StringNull()
	}
}

func validateFleetMCPServerConfig(data *FleetMCPServerResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	authType, authTypeSet := knownStringValue(data.AuthType)
	_, oauthProviderSet := knownStringValue(data.OAuthProviderID)
	_, oauthModeSet := knownStringValue(data.OAuthMode)
	_, headersSet := knownStringValue(data.Headers)

	switch authType {
	case "oauth":
		if !oauthProviderSet {
			diags.AddAttributeError(
				path.Root("oauth_provider_id"),
				"Missing oauth_provider_id for oauth auth_type",
				`Set "oauth_provider_id" when "auth_type" is "oauth".`,
			)
		}
		if headersSet {
			diags.AddAttributeError(
				path.Root("headers"),
				"Invalid headers for oauth auth_type",
				`Do not set "headers" when "auth_type" is "oauth".`,
			)
		}
	case "headers":
		if !headersSet {
			diags.AddAttributeError(
				path.Root("headers"),
				"Missing headers for headers auth_type",
				`Set "headers" when "auth_type" is "headers".`,
			)
		}
		if oauthProviderSet {
			diags.AddAttributeError(
				path.Root("oauth_provider_id"),
				"Invalid oauth_provider_id for headers auth_type",
				`Do not set "oauth_provider_id" when "auth_type" is "headers".`,
			)
		}
		if oauthModeSet {
			diags.AddAttributeError(
				path.Root("oauth_mode"),
				"Invalid oauth_mode for headers auth_type",
				`Do not set "oauth_mode" when "auth_type" is "headers".`,
			)
		}
	default:
		if !authTypeSet {
			if oauthProviderSet {
				diags.AddAttributeError(
					path.Root("oauth_provider_id"),
					"Missing auth_type for oauth_provider_id",
					`Set "auth_type" to "oauth" when "oauth_provider_id" is configured.`,
				)
			}
			if oauthModeSet {
				diags.AddAttributeError(
					path.Root("oauth_mode"),
					"Missing auth_type for oauth_mode",
					`Set "auth_type" to "oauth" when "oauth_mode" is configured.`,
				)
			}
			if headersSet {
				diags.AddAttributeError(
					path.Root("headers"),
					"Missing auth_type for headers",
					`Set "auth_type" to "headers" when "headers" is configured.`,
				)
			}
		}
	}

	return diags
}

func knownStringValue(v types.String) (string, bool) {
	if v.IsNull() || v.IsUnknown() {
		return "", false
	}
	return v.ValueString(), true
}

func stringValueOrNil(v types.String) interface{} {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueString()
}

func jsonStringValuePreservingEquivalent(raw json.RawMessage, saved types.String) types.String {
	if len(raw) == 0 || string(raw) == "null" {
		return types.StringNull()
	}
	if !saved.IsNull() && !saved.IsUnknown() && normalizeJSON(saved.ValueString()) == normalizeJSON(string(raw)) {
		return saved
	}
	return jsonStringValue(raw)
}

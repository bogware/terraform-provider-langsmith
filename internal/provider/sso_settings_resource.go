// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

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
	_ resource.Resource                = &SSOSettingsResource{}
	_ resource.ResourceWithImportState = &SSOSettingsResource{}
)

// NewSSOSettingsResource returns a new SSOSettingsResource -- the gatekeeper
// that decides who rides into town through the single sign-on pass.
func NewSSOSettingsResource() resource.Resource {
	return &SSOSettingsResource{}
}

// SSOSettingsResource manages SSO settings in LangSmith. Like the Long Branch
// Saloon's door policy, it controls who gets in and under what terms.
type SSOSettingsResource struct {
	client *client.Client
}

// SSOSettingsResourceModel describes the Terraform state for SSO settings.
type SSOSettingsResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	DefaultWorkspaceRoleID types.String `tfsdk:"default_workspace_role_id"`
	DefaultWorkspaceIDs    types.String `tfsdk:"default_workspace_ids"`
	MetadataURL            types.String `tfsdk:"metadata_url"`
	MetadataXML            types.String `tfsdk:"metadata_xml"`
	ProviderID             types.String `tfsdk:"provider_id"`
	OrganizationID         types.String `tfsdk:"organization_id"`
}

// ssoSettingsCreateRequest is the order to set up the SSO checkpoint on the
// trail into LangSmith.
type ssoSettingsCreateRequest struct {
	DefaultWorkspaceRoleID *string         `json:"default_workspace_role_id,omitempty"`
	DefaultWorkspaceIDs    json.RawMessage `json:"default_workspace_ids,omitempty"`
	MetadataURL            *string         `json:"metadata_url,omitempty"`
	MetadataXML            *string         `json:"metadata_xml,omitempty"`
	AttributeMapping       json.RawMessage `json:"attribute_mapping,omitempty"`
}

// ssoSettingsUpdateRequest patches the SSO checkpoint -- adjusting the rules
// without tearing down the whole guardhouse.
type ssoSettingsUpdateRequest struct {
	DefaultWorkspaceRoleID *string         `json:"default_workspace_role_id,omitempty"`
	DefaultWorkspaceIDs    json.RawMessage `json:"default_workspace_ids,omitempty"`
	MetadataURL            *string         `json:"metadata_url,omitempty"`
	MetadataXML            *string         `json:"metadata_xml,omitempty"`
	AttributeMapping       json.RawMessage `json:"attribute_mapping,omitempty"`
}

// ssoSettingsAPIResponse is the full dispatch the API returns about an SSO
// configuration and its standing orders.
type ssoSettingsAPIResponse struct {
	ID                     string          `json:"id"`
	OrganizationID         string          `json:"organization_id"`
	ProviderID             string          `json:"provider_id"`
	DefaultWorkspaceRoleID string          `json:"default_workspace_role_id"`
	DefaultWorkspaceIDs    json.RawMessage `json:"default_workspace_ids"`
	MetadataURL            string          `json:"metadata_url"`
	MetadataXML            string          `json:"metadata_xml"`
}

// ssoSettingsListAPIResponse is the full manifest -- every SSO configuration
// the organization has on file.
type ssoSettingsListAPIResponse []ssoSettingsAPIResponse

func (r *SSOSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_settings"
}

func (r *SSOSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages LangSmith SSO settings.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the SSO settings.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"default_workspace_role_id": schema.StringAttribute{
				MarkdownDescription: "Default role ID for SSO-provisioned users.",
				Optional:            true,
			},
			"default_workspace_ids": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of default workspace IDs for SSO-provisioned users.",
				Optional:            true,
			},
			"metadata_url": schema.StringAttribute{
				MarkdownDescription: "The SAML metadata URL.",
				Optional:            true,
			},
			"metadata_xml": schema.StringAttribute{
				MarkdownDescription: "The SAML metadata XML.",
				Optional:            true,
				Sensitive:           true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "The SSO provider ID.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns these SSO settings.",
				Computed:            true,
			},
		},
	}
}

func (r *SSOSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SSOSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SSOSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := ssoSettingsCreateRequest{}

	if !data.DefaultWorkspaceRoleID.IsNull() && !data.DefaultWorkspaceRoleID.IsUnknown() {
		v := data.DefaultWorkspaceRoleID.ValueString()
		body.DefaultWorkspaceRoleID = &v
	}

	// Default workspace IDs ride along as raw JSON -- a whole wagon train of UUIDs.
	if !data.DefaultWorkspaceIDs.IsNull() && !data.DefaultWorkspaceIDs.IsUnknown() {
		body.DefaultWorkspaceIDs = json.RawMessage(data.DefaultWorkspaceIDs.ValueString())
	}

	if !data.MetadataURL.IsNull() && !data.MetadataURL.IsUnknown() {
		v := data.MetadataURL.ValueString()
		body.MetadataURL = &v
	}

	if !data.MetadataXML.IsNull() && !data.MetadataXML.IsUnknown() {
		v := data.MetadataXML.ValueString()
		body.MetadataXML = &v
	}

	var result ssoSettingsAPIResponse
	err := r.client.Post(ctx, "/api/v1/orgs/current/sso-settings", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating SSO settings", err.Error())
		return
	}

	mapSSOSettingsResponseToState(&data, &result)
	tflog.Trace(ctx, "created SSO settings resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSOSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SSOSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Like rounding up strays, we have to fetch the whole herd and pick ours
	// out by brand -- the API only offers a list endpoint.
	var listResult ssoSettingsListAPIResponse
	err := r.client.Get(ctx, "/api/v1/orgs/current/sso-settings", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading SSO settings", err.Error())
		return
	}

	var found *ssoSettingsAPIResponse
	for _, sso := range listResult {
		if sso.ID == data.ID.ValueString() {
			found = &sso
			break
		}
	}

	if found == nil {
		// SSO settings have vanished like a tumbleweed in the wind.
		resp.State.RemoveResource(ctx)
		return
	}

	mapSSOSettingsResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSOSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SSOSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := ssoSettingsUpdateRequest{}

	if !data.DefaultWorkspaceRoleID.IsNull() && !data.DefaultWorkspaceRoleID.IsUnknown() {
		v := data.DefaultWorkspaceRoleID.ValueString()
		body.DefaultWorkspaceRoleID = &v
	}

	if !data.DefaultWorkspaceIDs.IsNull() && !data.DefaultWorkspaceIDs.IsUnknown() {
		body.DefaultWorkspaceIDs = json.RawMessage(data.DefaultWorkspaceIDs.ValueString())
	}

	if !data.MetadataURL.IsNull() && !data.MetadataURL.IsUnknown() {
		v := data.MetadataURL.ValueString()
		body.MetadataURL = &v
	}

	if !data.MetadataXML.IsNull() && !data.MetadataXML.IsUnknown() {
		v := data.MetadataXML.ValueString()
		body.MetadataXML = &v
	}

	var result ssoSettingsAPIResponse
	err := r.client.Patch(ctx, "/api/v1/orgs/current/sso-settings/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating SSO settings", err.Error())
		return
	}

	mapSSOSettingsResponseToState(&data, &result)
	tflog.Trace(ctx, "updated SSO settings resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSOSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SSOSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/orgs/current/sso-settings/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting SSO settings", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted SSO settings resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *SSOSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapSSOSettingsResponseToState maps the API response onto Terraform state,
// leaving optional fields null when the API sends back nothing -- like an
// empty hitching post outside the Long Branch.
func mapSSOSettingsResponseToState(data *SSOSettingsResourceModel, result *ssoSettingsAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.OrganizationID = types.StringValue(result.OrganizationID)
	data.ProviderID = types.StringValue(result.ProviderID)

	if result.DefaultWorkspaceRoleID != "" {
		data.DefaultWorkspaceRoleID = types.StringValue(result.DefaultWorkspaceRoleID)
	} else {
		data.DefaultWorkspaceRoleID = types.StringNull()
	}

	if len(result.DefaultWorkspaceIDs) > 0 && string(result.DefaultWorkspaceIDs) != "null" {
		data.DefaultWorkspaceIDs = types.StringValue(string(result.DefaultWorkspaceIDs))
	} else {
		data.DefaultWorkspaceIDs = types.StringNull()
	}

	if result.MetadataURL != "" {
		data.MetadataURL = types.StringValue(result.MetadataURL)
	} else {
		data.MetadataURL = types.StringNull()
	}

	if result.MetadataXML != "" {
		data.MetadataXML = types.StringValue(result.MetadataXML)
	} else {
		data.MetadataXML = types.StringNull()
	}
}

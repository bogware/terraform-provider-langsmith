// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.Resource                = &AgentBuilderIntegrationsResource{}
	_ resource.ResourceWithImportState = &AgentBuilderIntegrationsResource{}
)

// agentBuilderIntegrationsID is the synthetic Terraform ID for this singleton
// resource — the API has exactly one settings document per workspace and no
// server-side identifier.
const agentBuilderIntegrationsID = "agent_builder_integrations"

func NewAgentBuilderIntegrationsResource() resource.Resource {
	return &AgentBuilderIntegrationsResource{}
}

type AgentBuilderIntegrationsResource struct {
	client *client.Client
}

type AgentBuilderIntegrationsResourceModel struct {
	ID                           types.String `tfsdk:"id"`
	IntegrationsEnabledByDefault types.Bool   `tfsdk:"integrations_enabled_by_default"`
	IntegrationOverrides         types.List   `tfsdk:"integration_overrides"`
	IntegrationCatalog           types.List   `tfsdk:"integration_catalog"`
	WorkspaceID                  types.String `tfsdk:"workspace_id"`
}

type agentBuilderIntegrationOverride struct {
	IntegrationKey string `json:"integration_key"`
	IsEnabled      bool   `json:"is_enabled"`
}

type agentBuilderIntegrationCatalogEntry struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	CanInvoke   bool   `json:"can_invoke"`
}

// agentBuilderIntegrationsPutRequest replaces the default policy and overrides.
// IntegrationOverrides is intentionally not omitempty: the PUT has replace
// semantics, so an empty list explicitly clears all overrides.
type agentBuilderIntegrationsPutRequest struct {
	IntegrationsEnabledByDefault bool                              `json:"integrations_enabled_by_default"`
	IntegrationOverrides         []agentBuilderIntegrationOverride `json:"integration_overrides"`
}

type agentBuilderIntegrationsAPI struct {
	IntegrationCatalog           []agentBuilderIntegrationCatalogEntry `json:"integration_catalog"`
	IntegrationOverrides         []agentBuilderIntegrationOverride     `json:"integration_overrides"`
	IntegrationsEnabledByDefault bool                                  `json:"integrations_enabled_by_default"`
}

var integrationOverrideObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"integration_key": types.StringType,
	"is_enabled":      types.BoolType,
}}

var integrationCatalogObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":           types.StringType,
	"key":          types.StringType,
	"display_name": types.StringType,
	"can_invoke":   types.BoolType,
}}

func (r *AgentBuilderIntegrationsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_builder_integrations"
}

func (r *AgentBuilderIntegrationsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Agent Builder integrations settings for a LangSmith workspace. This is a singleton, always-present settings document: creating the resource performs a PUT that replaces the workspace's default policy and per-integration overrides. `terraform destroy` only removes the resource from state — the settings are left as-is on the server because the API has no delete endpoint.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier (always `agent_builder_integrations`); the API has no server-side ID for this singleton.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"integrations_enabled_by_default": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Default policy: whether integrations not listed in `integration_overrides` are enabled for Agent Builder.",
			},
			"integration_overrides": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Per-integration overrides of the default policy. The PUT has replace semantics: omitting this attribute (or setting an empty list) clears all overrides.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"integration_key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Integration key from the catalog (see `integration_catalog`).",
						},
						"is_enabled": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Whether this integration is enabled, overriding the default policy.",
						},
					},
				},
			},
			"integration_catalog": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Read-only catalog of integrations known to the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true},
						"key":          schema.StringAttribute{Computed: true},
						"display_name": schema.StringAttribute{Computed: true},
						"can_invoke": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the caller can currently invoke this integration given the effective policy.",
						},
					},
				},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *AgentBuilderIntegrationsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AgentBuilderIntegrationsResource) buildPut(ctx context.Context, data *AgentBuilderIntegrationsResourceModel, diags *diag.Diagnostics) agentBuilderIntegrationsPutRequest {
	body := agentBuilderIntegrationsPutRequest{
		IntegrationsEnabledByDefault: data.IntegrationsEnabledByDefault.ValueBool(),
		IntegrationOverrides:         []agentBuilderIntegrationOverride{},
	}
	if !data.IntegrationOverrides.IsNull() && !data.IntegrationOverrides.IsUnknown() {
		var items []struct {
			IntegrationKey types.String `tfsdk:"integration_key"`
			IsEnabled      types.Bool   `tfsdk:"is_enabled"`
		}
		diags.Append(data.IntegrationOverrides.ElementsAs(ctx, &items, false)...)
		if diags.HasError() {
			return body
		}
		for _, it := range items {
			body.IntegrationOverrides = append(body.IntegrationOverrides, agentBuilderIntegrationOverride{
				IntegrationKey: it.IntegrationKey.ValueString(),
				IsEnabled:      it.IsEnabled.ValueBool(),
			})
		}
	}
	return body
}

func (r *AgentBuilderIntegrationsResource) upsert(ctx context.Context, data *AgentBuilderIntegrationsResourceModel, diags *diag.Diagnostics) {
	body := r.buildPut(ctx, data, diags)
	if diags.HasError() {
		return
	}
	var api agentBuilderIntegrationsAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Put(ctx, "/v1/agent-builder/integrations", body, &api); err != nil {
		diags.AddError("Error updating Agent Builder integrations settings", err.Error())
		return
	}
	r.mapResponse(&api, data, diags)
}

func (r *AgentBuilderIntegrationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentBuilderIntegrationsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created agent builder integrations settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentBuilderIntegrationsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentBuilderIntegrationsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api agentBuilderIntegrationsAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/v1/agent-builder/integrations", nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Agent Builder integrations settings", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentBuilderIntegrationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentBuilderIntegrationsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentBuilderIntegrationsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Agent Builder integrations settings are a singleton with no delete
	// endpoint. Removing the resource from state leaves the last-applied
	// settings in place on the server.
	tflog.Warn(ctx, "Agent Builder integrations settings cannot be deleted; the last-applied settings remain in effect. Removing from Terraform state only.")
}

func (r *AgentBuilderIntegrationsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import with the synthetic ID "agent_builder_integrations" for the
	// provider-default workspace, or with a workspace UUID to scope the
	// resource to that workspace.
	if req.ID != "" && req.ID != agentBuilderIntegrationsID {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), req.ID)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), agentBuilderIntegrationsID)...)
}

func (r *AgentBuilderIntegrationsResource) mapResponse(api *agentBuilderIntegrationsAPI, data *AgentBuilderIntegrationsResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(agentBuilderIntegrationsID)
	data.IntegrationsEnabledByDefault = types.BoolValue(api.IntegrationsEnabledByDefault)

	if len(api.IntegrationOverrides) > 0 {
		elems := make([]attr.Value, 0, len(api.IntegrationOverrides))
		for _, ov := range api.IntegrationOverrides {
			obj, d := types.ObjectValue(integrationOverrideObjectType.AttrTypes, map[string]attr.Value{
				"integration_key": types.StringValue(ov.IntegrationKey),
				"is_enabled":      types.BoolValue(ov.IsEnabled),
			})
			diags.Append(d...)
			elems = append(elems, obj)
		}
		list, d := types.ListValue(integrationOverrideObjectType, elems)
		diags.Append(d...)
		data.IntegrationOverrides = list
	} else if data.IntegrationOverrides.IsNull() || data.IntegrationOverrides.IsUnknown() {
		data.IntegrationOverrides = types.ListNull(integrationOverrideObjectType)
	}
	// Otherwise keep the configured value (e.g. an explicit empty list).

	catalogElems := make([]attr.Value, 0, len(api.IntegrationCatalog))
	for _, entry := range api.IntegrationCatalog {
		obj, d := types.ObjectValue(integrationCatalogObjectType.AttrTypes, map[string]attr.Value{
			"id":           types.StringValue(entry.ID),
			"key":          types.StringValue(entry.Key),
			"display_name": types.StringValue(entry.DisplayName),
			"can_invoke":   types.BoolValue(entry.CanInvoke),
		})
		diags.Append(d...)
		catalogElems = append(catalogElems, obj)
	}
	catalog, d := types.ListValue(integrationCatalogObjectType, catalogElems)
	diags.Append(d...)
	data.IntegrationCatalog = catalog

	// The API does not return a workspace_id; normalise null/unknown only.
	reconcileWorkspaceID(&data.WorkspaceID, "", diags)
}

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &APIKeyResource{}
	_ resource.ResourceWithImportState = &APIKeyResource{}
)

// NewAPIKeyResource constructs an APIKeyResource. The full key is only returned
// once, at creation time.
func NewAPIKeyResource() resource.Resource {
	return &APIKeyResource{}
}

// APIKeyResource manages a durable LangSmith API key (workspace- or org-scoped),
// distinct from service keys, personal access tokens, and SCIM tokens. API keys
// cannot be updated; any change forces recreation.
type APIKeyResource struct {
	client *client.Client
}

// apiKeyResourceModel holds the Terraform state for an API key. The full key is
// sensitive and only surfaces at creation; UseStateForUnknown preserves it across
// reads since it is never re-readable.
type apiKeyResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Description          types.String `tfsdk:"description"`
	ExpiresAt            types.String `tfsdk:"expires_at"`
	Workspaces           types.List   `tfsdk:"workspaces"`
	RoleID               types.String `tfsdk:"role_id"`
	OrgRoleID            types.String `tfsdk:"org_role_id"`
	ShortKey             types.String `tfsdk:"short_key"`
	Key                  types.String `tfsdk:"key"`
	CreatedAt            types.String `tfsdk:"created_at"`
	LastUsedAt           types.String `tfsdk:"last_used_at"`
	AccessScope          types.String `tfsdk:"access_scope"`
	WorkspaceNames       types.List   `tfsdk:"workspace_names"`
	DefaultWorkspaceName types.String `tfsdk:"default_workspace_name"`
	WorkspaceID          types.String `tfsdk:"workspace_id"`
}

// apiKeyCreateRequest is the wire format for minting a new API key. Optional
// fields are sent only when the caller pins them on.
type apiKeyCreateRequest struct {
	Description string   `json:"description,omitempty"`
	ExpiresAt   *string  `json:"expires_at,omitempty"`
	Workspaces  []string `json:"workspaces,omitempty"`
	RoleID      *string  `json:"role_id,omitempty"`
	OrgRoleID   *string  `json:"org_role_id,omitempty"`
}

// apiKeyCreateResponse is the one-time create response that includes the full
// API key — guard it carefully.
type apiKeyCreateResponse struct {
	ID                   string   `json:"id"`
	ShortKey             string   `json:"short_key"`
	Description          string   `json:"description"`
	Key                  string   `json:"key"`
	CreatedAt            *string  `json:"created_at"`
	LastUsedAt           *string  `json:"last_used_at"`
	ExpiresAt            *string  `json:"expires_at"`
	WorkspaceNames       []string `json:"workspace_names"`
	DefaultWorkspaceName *string  `json:"default_workspace_name"`
	RoleID               *string  `json:"role_id"`
	OrgRoleID            *string  `json:"org_role_id"`
	AccessScope          *string  `json:"access_scope"`
}

// apiKeyListItem is a single API key from the list response. The full key is
// never present — only the short key remains.
type apiKeyListItem struct {
	ID                   string   `json:"id"`
	ShortKey             string   `json:"short_key"`
	Description          string   `json:"description"`
	CreatedAt            *string  `json:"created_at"`
	LastUsedAt           *string  `json:"last_used_at"`
	ExpiresAt            *string  `json:"expires_at"`
	WorkspaceNames       []string `json:"workspace_names"`
	DefaultWorkspaceName *string  `json:"default_workspace_name"`
	RoleID               *string  `json:"role_id"`
	OrgRoleID            *string  `json:"org_role_id"`
	AccessScope          *string  `json:"access_scope"`
}

// apiKeyListResponse is the bare JSON array returned by GET /api/v1/api-key.
type apiKeyListResponse []apiKeyListItem

func (r *APIKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *APIKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a durable LangSmith API key (workspace- or org-scoped). API keys cannot be updated; changing any input attribute forces recreation. The full key is only returned at creation time and cannot be read back afterwards.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the API key.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description for the API key. Defaults to `Default API key` server-side when omitted.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Default API key"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the API key expires. Omit for a non-expiring key.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspaces": schema.ListAttribute{
				MarkdownDescription: "List of workspace UUIDs this key may access. Feature-flagged on the LangSmith side.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the workspace role to assign to the API key. If omitted, the server picks a default role.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"org_role_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the organization role for org-scoped keys. If omitted, defaults to ORG_USER server-side.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"short_key": schema.StringAttribute{
				MarkdownDescription: "The shortened, non-secret version of the API key for display purposes.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The full API key. Only available at creation time; cannot be read back afterwards.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the API key.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the API key's most recent use, if ever used.",
				Computed:            true,
			},
			"access_scope": schema.StringAttribute{
				MarkdownDescription: "The access scope of the key: `organization` or `workspace`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_names": schema.ListAttribute{
				MarkdownDescription: "Human-readable names of the workspaces this key can access, as returned by the API.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"default_workspace_name": schema.StringAttribute{
				MarkdownDescription: "Name of the key's default workspace, as returned by the API.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource. Used for request routing only; it does not bind the key to a workspace (use `workspaces` for that).",
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

func (r *APIKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *APIKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := apiKeyCreateRequest{
		Description: data.Description.ValueString(),
	}

	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		v := data.ExpiresAt.ValueString()
		body.ExpiresAt = &v
	}
	if !data.RoleID.IsNull() && !data.RoleID.IsUnknown() {
		v := data.RoleID.ValueString()
		body.RoleID = &v
	}
	if !data.OrgRoleID.IsNull() && !data.OrgRoleID.IsUnknown() {
		v := data.OrgRoleID.ValueString()
		body.OrgRoleID = &v
	}
	if !data.Workspaces.IsNull() && !data.Workspaces.IsUnknown() {
		var ws []string
		resp.Diagnostics.Append(data.Workspaces.ElementsAs(ctx, &ws, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Workspaces = ws
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)

	var result apiKeyCreateResponse
	if err := effClient.Post(ctx, "/api/v1/api-key", body, &result); err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.ShortKey = types.StringValue(result.ShortKey)
	data.Description = types.StringValue(result.Description)
	data.Key = types.StringValue(result.Key)
	data.CreatedAt = types.StringPointerValue(result.CreatedAt)
	data.LastUsedAt = types.StringPointerValue(result.LastUsedAt)
	data.AccessScope = types.StringPointerValue(result.AccessScope)
	data.DefaultWorkspaceName = types.StringPointerValue(result.DefaultWorkspaceName)

	resp.Diagnostics.Append(apiKeySetWorkspaceNames(ctx, &data.WorkspaceNames, result.WorkspaceNames)...)
	if resp.Diagnostics.HasError() {
		return
	}

	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)
	tflog.Trace(ctx, "created API key resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *APIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	effClient := effectiveClient(r.client, data.WorkspaceID)

	var listResult apiKeyListResponse
	if err := effClient.Get(ctx, "/api/v1/api-key", nil, &listResult); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading API keys", err.Error())
		return
	}

	var found *apiKeyListItem
	for i := range listResult {
		if listResult[i].ID == data.ID.ValueString() {
			found = &listResult[i]
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(found.ID)
	data.ShortKey = types.StringValue(found.ShortKey)
	data.Description = types.StringValue(found.Description)
	data.CreatedAt = types.StringPointerValue(found.CreatedAt)
	data.LastUsedAt = types.StringPointerValue(found.LastUsedAt)
	data.AccessScope = types.StringPointerValue(found.AccessScope)
	data.DefaultWorkspaceName = types.StringPointerValue(found.DefaultWorkspaceName)
	// The full key is never returned on read — it was a one-time reveal.
	// UseStateForUnknown keeps the original value safe in state.

	resp.Diagnostics.Append(apiKeySetWorkspaceNames(ctx, &data.WorkspaceNames, found.WorkspaceNames)...)
	if resp.Diagnostics.HasError() {
		return
	}

	finalizeWorkspaceID(&data.WorkspaceID, effClient, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is unreachable: every configurable attribute — description, expires_at,
// workspaces, role_id, org_role_id and workspace_id — carries RequiresReplace,
// and the LangSmith API exposes no update endpoint for API keys, so any change
// plans as a replacement. It errors loudly rather than silently accepting the
// plan, so that re-introducing an updatable attribute fails the apply instead of
// writing a changed value to state that was never sent to the API.
func (r *APIKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"API keys cannot be updated; the LangSmith API exposes no update endpoint for them. "+
			"All configurable attributes are marked RequiresReplace, so this method should be unreachable — "+
			"reaching it means an updatable attribute was added to the schema without update support. "+
			"Please report this as a provider bug.",
	)
}

func (r *APIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/api-key/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting API key", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted API key resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *APIKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// apiKeySetWorkspaceNames writes the API-returned workspace names into the given
// Terraform list attribute, normalising an absent list to a typed null.
func apiKeySetWorkspaceNames(ctx context.Context, dst *types.List, names []string) diag.Diagnostics {
	if len(names) == 0 {
		*dst = types.ListNull(types.StringType)
		return nil
	}
	v, diags := types.ListValueFrom(ctx, types.StringType, names)
	if diags.HasError() {
		return diags
	}
	*dst = v
	return diags
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
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

// NewAPIKeyResource constructs an APIKeyResource for tenant/workspace API keys
// under GET/POST /api/v1/api-key and DELETE /api/v1/api-key/{id}.
func NewAPIKeyResource() resource.Resource {
	return &APIKeyResource{}
}

// APIKeyResource manages LangSmith API keys for the current tenant (workspace
// context via X-Tenant-Id when configured on the provider).
type APIKeyResource struct {
	client *client.Client
}

// APIKeyResourceModel is Terraform state for langsmith_api_key.
type APIKeyResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Description          types.String `tfsdk:"description"`
	ReadOnly             types.Bool   `tfsdk:"read_only"`
	ShortKey             types.String `tfsdk:"short_key"`
	Key                  types.String `tfsdk:"key"`
	CreatedAt            types.String `tfsdk:"created_at"`
	LastUsedAt           types.String `tfsdk:"last_used_at"`
	ExpiresAt            types.String `tfsdk:"expires_at"`
	WorkspaceNames       types.List   `tfsdk:"workspace_names"`
	DefaultWorkspaceName types.String `tfsdk:"default_workspace_name"`
	DefaultWorkspaceID   types.String `tfsdk:"default_workspace_id"`
	RoleID               types.String `tfsdk:"role_id"`
	OrgRoleID            types.String `tfsdk:"org_role_id"`
}

type apiKeyCreateRequest struct {
	Description        string  `json:"description"`
	ReadOnly           bool    `json:"read_only"`
	ExpiresAt          *string `json:"expires_at,omitempty"`
	DefaultWorkspaceID *string `json:"default_workspace_id,omitempty"`
	RoleID             *string `json:"role_id,omitempty"`
	OrgRoleID          *string `json:"org_role_id,omitempty"`
}

type apiKeyCreateResponse struct {
	CreatedAt            *string  `json:"created_at"`
	ID                   string   `json:"id"`
	ShortKey             string   `json:"short_key"`
	Description          string   `json:"description"`
	ReadOnly             bool     `json:"read_only"`
	LastUsedAt           *string  `json:"last_used_at"`
	ExpiresAt            *string  `json:"expires_at"`
	WorkspaceNames       []string `json:"workspace_names"`
	DefaultWorkspaceName *string  `json:"default_workspace_name"`
	Key                  string   `json:"key"`
}

type apiKeyGetResponse apiKeyCreateResponse

func (r *APIKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *APIKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith API key for the **current tenant** (`GET`/`POST` [`/api/v1/api-key`](https://api.smith.langchain.com/openapi.json), `DELETE` `/api/v1/api-key/{api_key_id}`). " +
			"Use the provider's `tenant_id` / `LANGSMITH_TENANT_ID` so requests include `X-Tenant-Id` when your credentials are org-scoped. " +
			"The LangSmith API does not expose `PATCH` for these keys; changing any create-time attribute replaces the key. " +
			"The full secret is returned only once at creation (and is empty after `terraform import`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "API key identifier (UUID).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Human-readable description stored with the key.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Default API key"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"read_only": schema.BoolAttribute{
				MarkdownDescription: "When true, the API assigns a reader role for the key (maps to LangSmith's deprecated `read_only` flag on create).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"short_key": schema.StringAttribute{
				MarkdownDescription: "Masked / shortened key prefix for display.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Full API key secret. Only present immediately after create; later reads and import leave this empty.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation time (RFC3339) when returned by the API.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "Last-used timestamp when the API provides it.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "Optional RFC3339 expiry for the key.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_names": schema.ListAttribute{
				MarkdownDescription: "Workspace names associated with the key (from the API).",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"default_workspace_name": schema.StringAttribute{
				MarkdownDescription: "Default workspace display name when returned by the API.",
				Computed:            true,
			},
			"default_workspace_id": schema.StringAttribute{
				MarkdownDescription: "Optional default workspace UUID for the new key.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "Optional workspace role UUID; if omitted the API picks a default based on `read_only`.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"org_role_id": schema.StringAttribute{
				MarkdownDescription: "Optional organization role UUID for org-scoped keys (create body only; not echoed on read).",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
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
	var data APIKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := apiKeyCreateRequest{
		Description: data.Description.ValueString(),
		ReadOnly:    data.ReadOnly.ValueBool(),
	}
	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		v := data.ExpiresAt.ValueString()
		body.ExpiresAt = &v
	}
	if !data.DefaultWorkspaceID.IsNull() && !data.DefaultWorkspaceID.IsUnknown() {
		v := data.DefaultWorkspaceID.ValueString()
		body.DefaultWorkspaceID = &v
	}
	if !data.RoleID.IsNull() && !data.RoleID.IsUnknown() {
		v := data.RoleID.ValueString()
		body.RoleID = &v
	}
	if !data.OrgRoleID.IsNull() && !data.OrgRoleID.IsUnknown() {
		v := data.OrgRoleID.ValueString()
		body.OrgRoleID = &v
	}

	var result apiKeyCreateResponse
	if err := r.client.Post(ctx, "/api/v1/api-key", body, &result); err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Description = types.StringValue(result.Description)
	data.ReadOnly = types.BoolValue(result.ReadOnly)
	data.ShortKey = types.StringValue(result.ShortKey)
	data.Key = types.StringValue(result.Key)
	if result.CreatedAt != nil && *result.CreatedAt != "" {
		data.CreatedAt = types.StringValue(*result.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if result.LastUsedAt != nil && *result.LastUsedAt != "" {
		data.LastUsedAt = types.StringValue(*result.LastUsedAt)
	} else {
		data.LastUsedAt = types.StringNull()
	}
	if result.ExpiresAt != nil && *result.ExpiresAt != "" {
		data.ExpiresAt = types.StringValue(*result.ExpiresAt)
	} else {
		data.ExpiresAt = types.StringNull()
	}
	if len(result.WorkspaceNames) > 0 {
		names, diags := types.ListValueFrom(ctx, types.StringType, result.WorkspaceNames)
		resp.Diagnostics.Append(diags...)
		data.WorkspaceNames = names
	} else {
		data.WorkspaceNames = types.ListNull(types.StringType)
	}
	if result.DefaultWorkspaceName != nil && *result.DefaultWorkspaceName != "" {
		data.DefaultWorkspaceName = types.StringValue(*result.DefaultWorkspaceName)
	} else {
		data.DefaultWorkspaceName = types.StringNull()
	}

	tflog.Trace(ctx, "created langsmith_api_key", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *APIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data APIKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var list []apiKeyGetResponse
	if err := r.client.Get(ctx, "/api/v1/api-key", nil, &list); err != nil {
		resp.Diagnostics.AddError("Error reading API keys", err.Error())
		return
	}

	var found *apiKeyGetResponse
	id := data.ID.ValueString()
	for i := range list {
		if list[i].ID == id {
			found = &list[i]
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(found.ID)
	data.Description = types.StringValue(found.Description)
	data.ReadOnly = types.BoolValue(found.ReadOnly)
	data.ShortKey = types.StringValue(found.ShortKey)
	if found.CreatedAt != nil && *found.CreatedAt != "" {
		data.CreatedAt = types.StringValue(*found.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if found.LastUsedAt != nil && *found.LastUsedAt != "" {
		data.LastUsedAt = types.StringValue(*found.LastUsedAt)
	} else {
		data.LastUsedAt = types.StringNull()
	}
	if found.ExpiresAt != nil && *found.ExpiresAt != "" {
		data.ExpiresAt = types.StringValue(*found.ExpiresAt)
	} else {
		data.ExpiresAt = types.StringNull()
	}
	if len(found.WorkspaceNames) > 0 {
		names, diags := types.ListValueFrom(ctx, types.StringType, found.WorkspaceNames)
		resp.Diagnostics.Append(diags...)
		data.WorkspaceNames = names
	} else {
		data.WorkspaceNames = types.ListNull(types.StringType)
	}
	if found.DefaultWorkspaceName != nil && *found.DefaultWorkspaceName != "" {
		data.DefaultWorkspaceName = types.StringValue(*found.DefaultWorkspaceName)
	} else {
		data.DefaultWorkspaceName = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *APIKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Tenant API keys are not patchable in the LangSmith API; all mutable attributes use create-time replacement.",
	)
}

func (r *APIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data APIKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/api-key/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting API key", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted langsmith_api_key", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *APIKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

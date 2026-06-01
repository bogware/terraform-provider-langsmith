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
	_ resource.Resource                = &AccessPolicyResource{}
	_ resource.ResourceWithImportState = &AccessPolicyResource{}
)

func NewAccessPolicyResource() resource.Resource {
	return &AccessPolicyResource{}
}

type AccessPolicyResource struct {
	client *client.Client
}

type AccessPolicyResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Effect          types.String `tfsdk:"effect"`
	ConditionGroups types.String `tfsdk:"condition_groups"`
	RoleIDs         types.String `tfsdk:"role_ids"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
}

type accessPolicyCreateRequest struct {
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	Effect          string          `json:"effect"`
	ConditionGroups json.RawMessage `json:"condition_groups,omitempty"`
	RoleIDs         []string        `json:"role_ids,omitempty"`
}

type accessPolicyAPIResponse struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Effect          string          `json:"effect"`
	ConditionGroups json.RawMessage `json:"condition_groups"`
	RoleIDs         []string        `json:"role_ids"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

type accessPolicyCreateResponse struct {
	ID string `json:"id"`
}

func (r *AccessPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_policy"
}

func (r *AccessPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith access policy (ABAC). Access policies define fine-grained permissions for roles.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the access policy.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the access policy.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the access policy.",
				Optional:            true,
			},
			"effect": schema.StringAttribute{
				MarkdownDescription: "The policy effect (`allow` or `deny`).",
				Required:            true,
			},
			"condition_groups": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of condition groups.",
				Optional:            true,
			},
			"role_ids": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of role IDs to attach this policy to.",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *AccessPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AccessPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccessPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := accessPolicyCreateRequest{
		Name:   data.Name.ValueString(),
		Effect: data.Effect.ValueString(),
	}
	setOptionalString(&body.Description, data.Description)

	if !data.ConditionGroups.IsNull() && !data.ConditionGroups.IsUnknown() {
		body.ConditionGroups = json.RawMessage(data.ConditionGroups.ValueString())
	}
	if !data.RoleIDs.IsNull() && !data.RoleIDs.IsUnknown() {
		var roleIDs []string
		if err := json.Unmarshal([]byte(data.RoleIDs.ValueString()), &roleIDs); err != nil {
			resp.Diagnostics.AddError("Invalid role_ids", fmt.Sprintf("role_ids must be a JSON array of strings: %s", err))
			return
		}
		body.RoleIDs = roleIDs
	}

	var createResult accessPolicyCreateResponse
	err := effectiveClient(r.client, data.WorkspaceID).Post(ctx, "/v1/platform/orgs/current/access-policies", body, &createResult)
	if err != nil {
		resp.Diagnostics.AddError("Error creating access policy", err.Error())
		return
	}

	data.ID = types.StringValue(createResult.ID)

	// Read back to get full state.
	var result accessPolicyAPIResponse
	err = effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/v1/platform/orgs/current/access-policies/"+createResult.ID, nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading access policy after creation", err.Error())
		return
	}

	mapAccessPolicyResponseToState(&data, &result)
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	tflog.Trace(ctx, "created access policy resource", map[string]interface{}{"id": createResult.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccessPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result accessPolicyAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/v1/platform/orgs/current/access-policies/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading access policy", err.Error())
		return
	}

	mapAccessPolicyResponseToState(&data, &result)
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The access policy API doesn't have an update endpoint.
	// Changes require delete + create (RequiresReplace could be used, but
	// keeping Update as a no-op preserves state consistency for now).
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Access policies cannot be updated in place. Delete and recreate the policy.",
	)
}

func (r *AccessPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccessPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/v1/platform/orgs/current/access-policies/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting access policy", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted access policy resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *AccessPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapAccessPolicyResponseToState(data *AccessPolicyResourceModel, result *accessPolicyAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.Effect = types.StringValue(result.Effect)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.ConditionGroups = jsonStringValue(result.ConditionGroups)
	if len(result.RoleIDs) > 0 {
		roleIDsJSON, _ := json.Marshal(result.RoleIDs)
		data.RoleIDs = types.StringValue(normalizeJSON(string(roleIDsJSON)))
	} else {
		data.RoleIDs = types.StringNull()
	}
}

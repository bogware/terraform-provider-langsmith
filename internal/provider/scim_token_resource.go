// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
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
	_ resource.Resource                = &SCIMTokenResource{}
	_ resource.ResourceWithImportState = &SCIMTokenResource{}
)

func NewSCIMTokenResource() resource.Resource {
	return &SCIMTokenResource{}
}

type SCIMTokenResource struct {
	client *client.Client
}

type SCIMTokenResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Description types.String `tfsdk:"description"`
	Token       types.String `tfsdk:"token"`
	ShortToken  types.String `tfsdk:"short_token"`
	LastUsedAt  types.String `tfsdk:"last_used_at"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type scimTokenCreateRequest struct {
	Description *string `json:"description,omitempty"`
}

type scimTokenUpdateRequest struct {
	Description *string `json:"description,omitempty"`
}

type scimTokenSensitiveResponse struct {
	ID          string  `json:"id"`
	Description *string `json:"description"`
	Token       string  `json:"token"`
	ShortToken  string  `json:"short_token"`
	LastUsedAt  *string `json:"last_used_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type scimTokenResponse struct {
	ID          string  `json:"id"`
	Description *string `json:"description"`
	ShortToken  string  `json:"short_token"`
	LastUsedAt  *string `json:"last_used_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func (r *SCIMTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_token"
}

func (r *SCIMTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith SCIM provisioning token for enterprise SSO/SCIM integration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the SCIM token.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the SCIM token.",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "The full SCIM token value. Only available at creation time.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"short_token": schema.StringAttribute{
				MarkdownDescription: "A truncated version of the token for display.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "The last time the token was used.",
				Computed:            true,
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
		},
	}
}

func (r *SCIMTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SCIMTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SCIMTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := scimTokenCreateRequest{}
	setOptionalString(&body.Description, data.Description)

	var result scimTokenSensitiveResponse
	err := r.client.Post(ctx, "/v1/platform/orgs/current/scim/tokens", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating SCIM token", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Token = types.StringValue(result.Token)
	data.ShortToken = types.StringValue(result.ShortToken)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.LastUsedAt, result.LastUsedAt)

	tflog.Trace(ctx, "created SCIM token resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCIMTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SCIMTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result scimTokenResponse
	err := r.client.Get(ctx, "/v1/platform/orgs/current/scim/tokens/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading SCIM token", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.ShortToken = types.StringValue(result.ShortToken)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.LastUsedAt, result.LastUsedAt)
	// Token is only available at creation, preserve existing state value.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCIMTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SCIMTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := scimTokenUpdateRequest{}
	setOptionalString(&body.Description, data.Description)

	var result scimTokenResponse
	err := r.client.Patch(ctx, "/v1/platform/orgs/current/scim/tokens/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating SCIM token", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.ShortToken = types.StringValue(result.ShortToken)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.LastUsedAt, result.LastUsedAt)

	tflog.Trace(ctx, "updated SCIM token resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCIMTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SCIMTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/v1/platform/orgs/current/scim/tokens/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting SCIM token", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted SCIM token resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *SCIMTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

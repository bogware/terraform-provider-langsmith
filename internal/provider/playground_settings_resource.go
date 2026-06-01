// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &PlaygroundSettingsResource{}
	_ resource.ResourceWithImportState = &PlaygroundSettingsResource{}
)

// NewPlaygroundSettingsResource returns a new PlaygroundSettingsResource for
// wrangling the LangSmith playground configuration.
func NewPlaygroundSettingsResource() resource.Resource {
	return &PlaygroundSettingsResource{}
}

// PlaygroundSettingsResource manages LangSmith playground settings -- the saloon
// where folks go to try out prompts before taking them into the real world.
type PlaygroundSettingsResource struct {
	client *client.Client
}

// PlaygroundSettingsResourceModel holds the Terraform state for playground settings.
// The "settings" field is a JSON string -- flexible enough to carry whatever
// configuration the playground needs without a rigid schema.
type PlaygroundSettingsResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Settings     types.String `tfsdk:"settings"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
	Options      types.String `tfsdk:"options"`
	SettingsType types.String `tfsdk:"settings_type"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
}

// playgroundSettingsAPICreateRequest is the request body for creating playground settings.
// Every new saloon in Dodge City needs a proper blueprint before the first nail goes in.
type playgroundSettingsAPICreateRequest struct {
	Name         *string         `json:"name,omitempty"`
	Description  *string         `json:"description,omitempty"`
	Settings     json.RawMessage `json:"settings"`
	Options      json.RawMessage `json:"options,omitempty"`
	SettingsType *string         `json:"settings_type,omitempty"`
}

// playgroundSettingsAPIUpdateRequest is the request body for updating playground settings.
// Even Marshal Dillon had to make adjustments to the law now and then.
type playgroundSettingsAPIUpdateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Settings    json.RawMessage `json:"settings"`
	Options     json.RawMessage `json:"options,omitempty"`
}

// playgroundSettingsAPIResponse is the API response for playground settings.
type playgroundSettingsAPIResponse struct {
	ID           string          `json:"id"`
	Name         *string         `json:"name"`
	Description  *string         `json:"description"`
	Settings     json.RawMessage `json:"settings"`
	Options      json.RawMessage `json:"options"`
	SettingsType string          `json:"settings_type"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

func (r *PlaygroundSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_playground_settings"
}

func (r *PlaygroundSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages LangSmith playground settings.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the playground settings.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the playground settings.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the playground settings.",
				Optional:            true,
			},
			"settings": schema.StringAttribute{
				MarkdownDescription: "A JSON string containing the settings object.",
				Required:            true,
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
			"options": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded options object.",
				Optional:            true,
			},
			"settings_type": schema.StringAttribute{
				MarkdownDescription: "The settings type. Valid values: `complex`, `simple`. Defaults to `complex`. Cannot be changed after creation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{stringvalidator.OneOf("complex", "simple")},
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

func (r *PlaygroundSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PlaygroundSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PlaygroundSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := playgroundSettingsAPICreateRequest{
		Settings: json.RawMessage(data.Settings.ValueString()),
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	// Saddle up the options if somebody packed them for the ride.
	if !data.Options.IsNull() && !data.Options.IsUnknown() {
		body.Options = json.RawMessage(data.Options.ValueString())
	}
	// Pin on the settings type badge if the deputy brought one along.
	if !data.SettingsType.IsNull() && !data.SettingsType.IsUnknown() {
		v := data.SettingsType.ValueString()
		body.SettingsType = &v
	}

	// Preserve plan values; the API may normalize or expand JSON fields.
	planSettings := data.Settings
	planOptions := data.Options

	var result playgroundSettingsAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Post(ctx, "/api/v1/playground-settings", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating playground settings", err.Error())
		return
	}

	mapPlaygroundSettingsResponseToState(&data, &result)
	data.Settings = planSettings
	if !planOptions.IsNull() && !planOptions.IsUnknown() {
		data.Options = planOptions
	}
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	tflog.Trace(ctx, "created playground settings resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlaygroundSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PlaygroundSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []playgroundSettingsAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/api/v1/playground-settings", nil, &results)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading playground settings", err.Error())
		return
	}

	var found *playgroundSettingsAPIResponse
	for i := range results {
		if results[i].ID == data.ID.ValueString() {
			found = &results[i]
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapPlaygroundSettingsResponseToState(&data, found)

	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlaygroundSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PlaygroundSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := playgroundSettingsAPIUpdateRequest{
		Settings: json.RawMessage(data.Settings.ValueString()),
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	// Pack the options for the trail if the cowhand brought any.
	if !data.Options.IsNull() && !data.Options.IsUnknown() {
		body.Options = json.RawMessage(data.Options.ValueString())
	}

	// Preserve plan values; the API may normalize or expand JSON fields.
	planSettings := data.Settings
	planOptions := data.Options

	var result playgroundSettingsAPIResponse
	err := effectiveClient(r.client, data.WorkspaceID).Patch(ctx, "/api/v1/playground-settings/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating playground settings", err.Error())
		return
	}

	mapPlaygroundSettingsResponseToState(&data, &result)
	data.Settings = planSettings
	if !planOptions.IsNull() && !planOptions.IsUnknown() {
		data.Options = planOptions
	}
	tflog.Trace(ctx, "updated playground settings resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlaygroundSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PlaygroundSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/playground-settings/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting playground settings", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted playground settings resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *PlaygroundSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapPlaygroundSettingsResponseToState corrals the API response into the Terraform
// state model, handling nullable name/description fields and raw JSON settings.
func mapPlaygroundSettingsResponseToState(data *PlaygroundSettingsResourceModel, result *playgroundSettingsAPIResponse) {
	data.ID = types.StringValue(result.ID)

	if result.Name != nil {
		data.Name = types.StringValue(*result.Name)
	} else {
		data.Name = types.StringNull()
	}

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Settings = jsonStringValue(result.Settings)

	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	// Stash the options in state -- like Miss Kitty's lockbox, it holds
	// whatever JSON valuables the API sent back from the Long Branch.
	data.Options = jsonStringValue(result.Options)

	data.SettingsType = types.StringValue(result.SettingsType)
}

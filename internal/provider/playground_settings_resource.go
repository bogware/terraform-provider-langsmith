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

	OAuthEnabled                 types.Bool   `tfsdk:"oauth_enabled"`
	OAuthTokenURL                types.String `tfsdk:"oauth_token_url"`
	OAuthClientID                types.String `tfsdk:"oauth_client_id"`
	OAuthClientSecret            types.String `tfsdk:"oauth_client_secret"`
	OAuthTokenEndpointAuthMethod types.String `tfsdk:"oauth_token_endpoint_auth_method"`
	OAuthParams                  types.String `tfsdk:"oauth_params"`
	OAuthHeaders                 types.String `tfsdk:"oauth_headers"`

	AvailableInPlayground    types.Bool   `tfsdk:"available_in_playground"`
	AvailableInEvaluators    types.Bool   `tfsdk:"available_in_evaluators"`
	AvailableInAgentBuilder  types.Bool   `tfsdk:"available_in_agent_builder"`
	AvailableInPolly         types.Bool   `tfsdk:"available_in_polly"`
	AvailableInInsightsHeavy types.Bool   `tfsdk:"available_in_insights_heavy"`
	AvailableInInsightsLight types.Bool   `tfsdk:"available_in_insights_light"`
	WorkspaceID              types.String `tfsdk:"workspace_id"`
}

// playgroundSettingsAPICreateRequest is the request body for creating playground settings.
// Every new saloon in Dodge City needs a proper blueprint before the first nail goes in.
type playgroundSettingsAPICreateRequest struct {
	Name                         *string         `json:"name,omitempty"`
	Description                  *string         `json:"description,omitempty"`
	Settings                     json.RawMessage `json:"settings"`
	Options                      json.RawMessage `json:"options,omitempty"`
	SettingsType                 *string         `json:"settings_type,omitempty"`
	OAuthEnabled                 *bool           `json:"oauth_enabled,omitempty"`
	OAuthTokenURL                *string         `json:"oauth_token_url,omitempty"`
	OAuthClientID                *string         `json:"oauth_client_id,omitempty"`
	OAuthClientSecret            *string         `json:"oauth_client_secret,omitempty"`
	OAuthTokenEndpointAuthMethod *string         `json:"oauth_token_endpoint_auth_method,omitempty"`
	OAuthParams                  json.RawMessage `json:"oauth_params,omitempty"`
	OAuthHeaders                 json.RawMessage `json:"oauth_headers,omitempty"`
}

// playgroundSettingsAPIUpdateRequest is the request body for updating playground settings.
// Even Marshal Dillon had to make adjustments to the law now and then.
type playgroundSettingsAPIUpdateRequest struct {
	Name                         *string         `json:"name,omitempty"`
	Description                  *string         `json:"description,omitempty"`
	Settings                     json.RawMessage `json:"settings"`
	Options                      json.RawMessage `json:"options,omitempty"`
	OAuthEnabled                 *bool           `json:"oauth_enabled,omitempty"`
	OAuthTokenURL                *string         `json:"oauth_token_url,omitempty"`
	OAuthClientID                *string         `json:"oauth_client_id,omitempty"`
	OAuthClientSecret            *string         `json:"oauth_client_secret,omitempty"`
	OAuthTokenEndpointAuthMethod *string         `json:"oauth_token_endpoint_auth_method,omitempty"`
	OAuthParams                  json.RawMessage `json:"oauth_params,omitempty"`
	OAuthHeaders                 json.RawMessage `json:"oauth_headers,omitempty"`

	// The available_in_* flags are accepted only by the update endpoint, so a
	// create that sets them is followed by a PATCH.
	AvailableInPlayground    *bool `json:"available_in_playground,omitempty"`
	AvailableInEvaluators    *bool `json:"available_in_evaluators,omitempty"`
	AvailableInAgentBuilder  *bool `json:"available_in_agent_builder,omitempty"`
	AvailableInPolly         *bool `json:"available_in_polly,omitempty"`
	AvailableInInsightsHeavy *bool `json:"available_in_insights_heavy,omitempty"`
	AvailableInInsightsLight *bool `json:"available_in_insights_light,omitempty"`
}

// playgroundSettingsAPIResponse is the API response for playground settings.
type playgroundSettingsAPIResponse struct {
	ID           string          `json:"id"`
	Name         *string         `json:"name"`
	Description  *string         `json:"description"`
	Settings     json.RawMessage `json:"settings"`
	Options      json.RawMessage `json:"options"`
	SettingsType string          `json:"settings_type"`

	OAuthEnabled                 bool            `json:"oauth_enabled"`
	OAuthTokenURL                *string         `json:"oauth_token_url"`
	OAuthClientID                *string         `json:"oauth_client_id"`
	OAuthClientSecret            *string         `json:"oauth_client_secret"`
	OAuthTokenEndpointAuthMethod *string         `json:"oauth_token_endpoint_auth_method"`
	OAuthParams                  json.RawMessage `json:"oauth_params"`
	OAuthHeaders                 json.RawMessage `json:"oauth_headers"`

	AvailableInPlayground    bool   `json:"available_in_playground"`
	AvailableInEvaluators    bool   `json:"available_in_evaluators"`
	AvailableInAgentBuilder  bool   `json:"available_in_agent_builder"`
	AvailableInPolly         bool   `json:"available_in_polly"`
	AvailableInInsightsHeavy bool   `json:"available_in_insights_heavy"`
	AvailableInInsightsLight bool   `json:"available_in_insights_light"`
	CreatedAt                string `json:"created_at"`
	UpdatedAt                string `json:"updated_at"`
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
			"oauth_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the provider behind these settings authenticates with OAuth.",
				Optional:            true,
				Computed:            true,
			},
			"oauth_token_url": schema.StringAttribute{
				MarkdownDescription: "OAuth token endpoint used to mint access tokens.",
				Optional:            true,
				Computed:            true,
			},
			"oauth_client_id": schema.StringAttribute{
				MarkdownDescription: "OAuth client ID.",
				Optional:            true,
				Computed:            true,
			},
			"oauth_client_secret": schema.StringAttribute{
				MarkdownDescription: "OAuth client secret. The API returns this value, so it is stored in state — treat the state file accordingly.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
			},
			"oauth_token_endpoint_auth_method": schema.StringAttribute{
				MarkdownDescription: "How the client authenticates to the token endpoint: `client_secret_basic` or `client_secret_post`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("client_secret_basic", "client_secret_post"),
				},
			},
			"oauth_params": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded object of extra parameters sent to the token endpoint.",
				Optional:            true,
				Computed:            true,
			},
			"oauth_headers": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded object of extra headers sent to the token endpoint. May carry credentials, so it is marked sensitive.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
			},
			"available_in_playground": schema.BoolAttribute{
				MarkdownDescription: "Whether these settings are offered in the playground. Set only by the update endpoint, so creating a resource with this attribute issues a follow-up API call.",
				Optional:            true,
				Computed:            true,
			},
			"available_in_evaluators": schema.BoolAttribute{
				MarkdownDescription: "Whether these settings are offered when configuring evaluators.",
				Optional:            true,
				Computed:            true,
			},
			"available_in_agent_builder": schema.BoolAttribute{
				MarkdownDescription: "Whether these settings are offered in Agent Builder.",
				Optional:            true,
				Computed:            true,
			},
			"available_in_polly": schema.BoolAttribute{
				MarkdownDescription: "Whether these settings are offered to Polly.",
				Optional:            true,
				Computed:            true,
			},
			"available_in_insights_heavy": schema.BoolAttribute{
				MarkdownDescription: "Whether these settings are offered for heavyweight insights jobs.",
				Optional:            true,
				Computed:            true,
			},
			"available_in_insights_light": schema.BoolAttribute{
				MarkdownDescription: "Whether these settings are offered for lightweight insights jobs.",
				Optional:            true,
				Computed:            true,
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
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

	setPlaygroundOAuthFields(&data, &body.OAuthEnabled, &body.OAuthTokenURL, &body.OAuthClientID,
		&body.OAuthClientSecret, &body.OAuthTokenEndpointAuthMethod, &body.OAuthParams, &body.OAuthHeaders)

	// Preserve plan values; the API may normalize or expand JSON fields.
	planSettings := data.Settings
	planOptions := data.Options

	apiClient := effectiveClient(r.client, data.WorkspaceID)

	var result playgroundSettingsAPIResponse
	err := apiClient.Post(ctx, "/api/v1/playground-settings", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating playground settings", err.Error())
		return
	}

	// The available_in_* flags exist only on the update endpoint, so a create
	// that configures them needs a second call. Without this the apply would
	// fail with "provider produced inconsistent result" whenever a flag is set
	// to anything but the server default.
	availability := playgroundSettingsAPIUpdateRequest{Settings: json.RawMessage(data.Settings.ValueString())}
	if setPlaygroundAvailability(&data, &availability) {
		if err := apiClient.Patch(ctx, "/api/v1/playground-settings/"+result.ID, availability, &result); err != nil {
			resp.Diagnostics.AddError(
				"Error applying playground settings availability",
				fmt.Sprintf("The playground settings were created (id %s) but the available_in_* flags could not be applied: %s", result.ID, err),
			)
			return
		}
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
	setPlaygroundOAuthFields(&data, &body.OAuthEnabled, &body.OAuthTokenURL, &body.OAuthClientID,
		&body.OAuthClientSecret, &body.OAuthTokenEndpointAuthMethod, &body.OAuthParams, &body.OAuthHeaders)
	setPlaygroundAvailability(&data, &body)

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

	data.OAuthEnabled = types.BoolValue(result.OAuthEnabled)
	data.OAuthTokenURL = types.StringPointerValue(result.OAuthTokenURL)
	data.OAuthClientID = types.StringPointerValue(result.OAuthClientID)
	data.OAuthClientSecret = types.StringPointerValue(result.OAuthClientSecret)
	data.OAuthTokenEndpointAuthMethod = types.StringPointerValue(result.OAuthTokenEndpointAuthMethod)
	data.OAuthParams = jsonStringValue(result.OAuthParams)
	data.OAuthHeaders = jsonStringValue(result.OAuthHeaders)

	data.AvailableInPlayground = types.BoolValue(result.AvailableInPlayground)
	data.AvailableInEvaluators = types.BoolValue(result.AvailableInEvaluators)
	data.AvailableInAgentBuilder = types.BoolValue(result.AvailableInAgentBuilder)
	data.AvailableInPolly = types.BoolValue(result.AvailableInPolly)
	data.AvailableInInsightsHeavy = types.BoolValue(result.AvailableInInsightsHeavy)
	data.AvailableInInsightsLight = types.BoolValue(result.AvailableInInsightsLight)
}

// setPlaygroundOAuthFields copies the OAuth attributes onto a request body.
// Create and Update share the same field set, so they share this.
func setPlaygroundOAuthFields(data *PlaygroundSettingsResourceModel,
	enabled **bool, tokenURL, clientID, clientSecret, authMethod **string,
	params, headers *json.RawMessage) {
	if !data.OAuthEnabled.IsNull() && !data.OAuthEnabled.IsUnknown() {
		v := data.OAuthEnabled.ValueBool()
		*enabled = &v
	}
	if !data.OAuthTokenURL.IsNull() && !data.OAuthTokenURL.IsUnknown() {
		v := data.OAuthTokenURL.ValueString()
		*tokenURL = &v
	}
	if !data.OAuthClientID.IsNull() && !data.OAuthClientID.IsUnknown() {
		v := data.OAuthClientID.ValueString()
		*clientID = &v
	}
	if !data.OAuthClientSecret.IsNull() && !data.OAuthClientSecret.IsUnknown() {
		v := data.OAuthClientSecret.ValueString()
		*clientSecret = &v
	}
	if !data.OAuthTokenEndpointAuthMethod.IsNull() && !data.OAuthTokenEndpointAuthMethod.IsUnknown() {
		v := data.OAuthTokenEndpointAuthMethod.ValueString()
		*authMethod = &v
	}
	if !data.OAuthParams.IsNull() && !data.OAuthParams.IsUnknown() {
		*params = json.RawMessage(data.OAuthParams.ValueString())
	}
	if !data.OAuthHeaders.IsNull() && !data.OAuthHeaders.IsUnknown() {
		*headers = json.RawMessage(data.OAuthHeaders.ValueString())
	}
}

// setPlaygroundAvailability copies the available_in_* attributes onto an update
// body and reports whether any of them was configured.
func setPlaygroundAvailability(data *PlaygroundSettingsResourceModel, body *playgroundSettingsAPIUpdateRequest) bool {
	configured := false
	for _, f := range []struct {
		src types.Bool
		dst **bool
	}{
		{data.AvailableInPlayground, &body.AvailableInPlayground},
		{data.AvailableInEvaluators, &body.AvailableInEvaluators},
		{data.AvailableInAgentBuilder, &body.AvailableInAgentBuilder},
		{data.AvailableInPolly, &body.AvailableInPolly},
		{data.AvailableInInsightsHeavy, &body.AvailableInInsightsHeavy},
		{data.AvailableInInsightsLight, &body.AvailableInInsightsLight},
	} {
		if f.src.IsNull() || f.src.IsUnknown() {
			continue
		}
		v := f.src.ValueBool()
		*f.dst = &v
		configured = true
	}
	return configured
}

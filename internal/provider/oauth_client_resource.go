// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &OAuthClientResource{}
	_ resource.ResourceWithImportState = &OAuthClientResource{}
)

// NewOAuthClientResource manages an OAuth client registered with LangSmith's
// authorization server.
func NewOAuthClientResource() resource.Resource {
	return &OAuthClientResource{}
}

// OAuthClientResource manages /api/v1/platform/oauth/clients.
type OAuthClientResource struct {
	client *client.Client
}

// OAuthClientResourceModel is the Terraform state for an OAuth client.
type OAuthClientResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ClientID      types.String `tfsdk:"client_id"`
	ClientSecret  types.String `tfsdk:"client_secret"`
	ClientName    types.String `tfsdk:"client_name"`
	ClientType    types.String `tfsdk:"client_type"`
	ClientURI     types.String `tfsdk:"client_uri"`
	LogoURI       types.String `tfsdk:"logo_uri"`
	PolicyURI     types.String `tfsdk:"policy_uri"`
	TOSURI        types.String `tfsdk:"tos_uri"`
	RedirectURIs  types.List   `tfsdk:"redirect_uris"`
	GrantTypes    types.List   `tfsdk:"grant_types"`
	AllowedScopes types.List   `tfsdk:"allowed_scopes"`
	Disabled      types.Bool   `tfsdk:"disabled"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

// oauthClientAPI mirrors the client object the API returns.
type oauthClientAPI struct {
	ID            string   `json:"id"`
	ClientID      string   `json:"client_id"`
	ClientName    string   `json:"client_name"`
	ClientType    string   `json:"client_type"`
	ClientURI     string   `json:"client_uri"`
	LogoURI       string   `json:"logo_uri"`
	PolicyURI     string   `json:"policy_uri"`
	TOSURI        string   `json:"tos_uri"`
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types"`
	AllowedScopes []string `json:"allowed_scopes"`
	Disabled      bool     `json:"disabled"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// oauthClientCreateResponse wraps the client alongside the one-time secret.
type oauthClientCreateResponse struct {
	Client       oauthClientAPI `json:"client"`
	ClientSecret string         `json:"client_secret"`
}

type oauthClientCreateRequest struct {
	ClientName    string   `json:"client_name,omitempty"`
	ClientType    string   `json:"client_type,omitempty"`
	ClientURI     string   `json:"client_uri,omitempty"`
	LogoURI       string   `json:"logo_uri,omitempty"`
	PolicyURI     string   `json:"policy_uri,omitempty"`
	TOSURI        string   `json:"tos_uri,omitempty"`
	RedirectURIs  []string `json:"redirect_uris,omitempty"`
	GrantTypes    []string `json:"grant_types,omitempty"`
	AllowedScopes []string `json:"allowed_scopes,omitempty"`
}

// oauthClientUpdateRequest omits client_type and grant_types: the update
// endpoint does not accept them, so both force replacement.
type oauthClientUpdateRequest struct {
	ClientName    string   `json:"client_name,omitempty"`
	ClientURI     string   `json:"client_uri,omitempty"`
	LogoURI       string   `json:"logo_uri,omitempty"`
	PolicyURI     string   `json:"policy_uri,omitempty"`
	TOSURI        string   `json:"tos_uri,omitempty"`
	RedirectURIs  []string `json:"redirect_uris,omitempty"`
	AllowedScopes []string `json:"allowed_scopes,omitempty"`
	Disabled      bool     `json:"disabled"`
}

func (r *OAuthClientResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth_client"
}

func (r *OAuthClientResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers an OAuth client with the LangSmith authorization server, for applications that sign in on a user's behalf.\n\n" +
			"**The client secret is returned only when the client is created** (and again when rotated), so it is stored in state and cannot be read back from the API. " +
			"`client_type` and `grant_types` are fixed at registration — the update endpoint does not accept them — so changing either forces a new client and a new secret.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Internal UUID of the registration.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Public client identifier issued by the authorization server.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Client secret, issued once at registration. Confidential clients need it to authenticate to the token endpoint; it cannot be read back afterwards, so capture it into a secret manager in the same apply.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"client_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name shown on the consent screen.",
				Required:            true,
			},
			"client_type": schema.StringAttribute{
				MarkdownDescription: "`confidential` for clients that can keep a secret (server-side apps), `public` for those that cannot (native and single-page apps). Fixed at registration.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("public", "confidential"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_uri": schema.StringAttribute{
				MarkdownDescription: "Home page of the client application.",
				Optional:            true,
			},
			"logo_uri": schema.StringAttribute{
				MarkdownDescription: "Logo shown on the consent screen.",
				Optional:            true,
			},
			"policy_uri": schema.StringAttribute{
				MarkdownDescription: "Privacy policy shown on the consent screen.",
				Optional:            true,
			},
			"tos_uri": schema.StringAttribute{
				MarkdownDescription: "Terms of service shown on the consent screen.",
				Optional:            true,
			},
			"redirect_uris": schema.ListAttribute{
				MarkdownDescription: "Exact redirect URIs the authorization server will send the user back to. An authorization request naming any other URI is rejected.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"grant_types": schema.ListAttribute{
				MarkdownDescription: "OAuth grant types the client may use (for example `authorization_code`, `refresh_token`). Fixed at registration.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"allowed_scopes": schema.ListAttribute{
				MarkdownDescription: "Scopes the client may request.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the client is disabled. A disabled client cannot obtain new tokens.",
				Optional:            true,
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the client was registered.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "When the client was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *OAuthClientResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// listOf converts an optional list attribute to a slice, treating null and
// unknown as absent.
func (r *OAuthClientResource) listOf(ctx context.Context, v types.List) []string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []string
	_ = v.ElementsAs(ctx, &out, false)
	return out
}

func (r *OAuthClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OAuthClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := oauthClientCreateRequest{
		ClientName:    data.ClientName.ValueString(),
		ClientType:    data.ClientType.ValueString(),
		ClientURI:     data.ClientURI.ValueString(),
		LogoURI:       data.LogoURI.ValueString(),
		PolicyURI:     data.PolicyURI.ValueString(),
		TOSURI:        data.TOSURI.ValueString(),
		RedirectURIs:  r.listOf(ctx, data.RedirectURIs),
		GrantTypes:    r.listOf(ctx, data.GrantTypes),
		AllowedScopes: r.listOf(ctx, data.AllowedScopes),
	}

	var result oauthClientCreateResponse
	if err := r.client.Post(ctx, "/api/v1/platform/oauth/clients", body, &result); err != nil {
		resp.Diagnostics.AddError("Error creating OAuth client", err.Error())
		return
	}

	resp.Diagnostics.Append(r.mapClient(ctx, &data, &result.Client)...)
	// Only moment the secret is ever visible.
	data.ClientSecret = types.StringValue(result.ClientSecret)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created OAuth client", map[string]interface{}{"id": result.Client.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OAuthClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OAuthClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var api oauthClientAPI
	if err := r.client.Get(ctx, "/api/v1/platform/oauth/clients/"+data.ID.ValueString(), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading OAuth client", err.Error())
		return
	}

	resp.Diagnostics.Append(r.mapClient(ctx, &data, &api)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OAuthClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OAuthClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state OAuthClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := oauthClientUpdateRequest{
		ClientName:    data.ClientName.ValueString(),
		ClientURI:     data.ClientURI.ValueString(),
		LogoURI:       data.LogoURI.ValueString(),
		PolicyURI:     data.PolicyURI.ValueString(),
		TOSURI:        data.TOSURI.ValueString(),
		RedirectURIs:  r.listOf(ctx, data.RedirectURIs),
		AllowedScopes: r.listOf(ctx, data.AllowedScopes),
		Disabled:      data.Disabled.ValueBool(),
	}

	var api oauthClientAPI
	if err := r.client.Patch(ctx, "/api/v1/platform/oauth/clients/"+state.ID.ValueString(), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating OAuth client", err.Error())
		return
	}

	resp.Diagnostics.Append(r.mapClient(ctx, &data, &api)...)
	// The secret is not reissued by an update, so carry the stored one forward.
	data.ClientSecret = state.ClientSecret
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OAuthClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OAuthClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/api/v1/platform/oauth/clients/"+data.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting OAuth client", err.Error())
	}
}

func (r *OAuthClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// client_secret is unreadable, so an imported client carries a null secret.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapClient copies the API client object onto Terraform state.
func (r *OAuthClientResource) mapClient(ctx context.Context, data *OAuthClientResourceModel, api *oauthClientAPI) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(api.ID)
	data.ClientID = types.StringValue(api.ClientID)
	data.ClientName = types.StringValue(api.ClientName)
	data.ClientType = types.StringValue(api.ClientType)
	data.Disabled = types.BoolValue(api.Disabled)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)

	// The API answers with "" for an unset URI; keep those null so an omitted
	// attribute does not read back as an empty string.
	setOptional := func(v string, dst *types.String) {
		if v == "" {
			*dst = types.StringNull()
			return
		}
		*dst = types.StringValue(v)
	}
	setOptional(api.ClientURI, &data.ClientURI)
	setOptional(api.LogoURI, &data.LogoURI)
	setOptional(api.PolicyURI, &data.PolicyURI)
	setOptional(api.TOSURI, &data.TOSURI)

	redirects, d := types.ListValueFrom(ctx, types.StringType, api.RedirectURIs)
	diags.Append(d...)
	data.RedirectURIs = redirects

	grants, d := types.ListValueFrom(ctx, types.StringType, api.GrantTypes)
	diags.Append(d...)
	data.GrantTypes = grants

	scopes, d := types.ListValueFrom(ctx, types.StringType, api.AllowedScopes)
	diags.Append(d...)
	data.AllowedScopes = scopes

	return diags
}

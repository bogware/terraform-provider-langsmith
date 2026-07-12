// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

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
	_ resource.Resource                = &SandboxRegistryResource{}
	_ resource.ResourceWithImportState = &SandboxRegistryResource{}
)

// NewSandboxRegistryResource returns a new SandboxRegistryResource -- the
// credentials that let the sandbox fetch images from your own corral instead
// of the public range.
func NewSandboxRegistryResource() resource.Resource {
	return &SandboxRegistryResource{}
}

// SandboxRegistryResource manages a private container image registry credential
// used by LangSmith sandboxes. The registry itself is durable configuration;
// the sandboxes that pull from it are not.
type SandboxRegistryResource struct {
	client *client.Client
}

// SandboxRegistryResourceModel describes the Terraform state for a sandbox registry.
type SandboxRegistryResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	URL         types.String `tfsdk:"url"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"`
	CreatedAt   types.String `tfsdk:"created_at"`
	CreatedBy   types.String `tfsdk:"created_by"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	UpdatedBy   types.String `tfsdk:"updated_by"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

// sandboxRegistryCreateRequest is the POST payload. All four fields are
// required by the API.
type sandboxRegistryCreateRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// sandboxRegistryUpdateRequest is the PATCH payload. Every field is optional
// on the wire, but we always send the mutable trio so the remote object matches
// configuration exactly. `name` is deliberately omitted: the registry is
// addressed by name, so a rename is modelled as a replace.
type sandboxRegistryUpdateRequest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// sandboxRegistryAPI mirrors RegistryResponse. Note what is *missing*: the API
// never echoes back `username` or `password`. Those are write-only, and the
// provider preserves them from prior state on Read.
//
// The endpoint returns no workspace field either; WorkspaceID/TenantID are
// decoded defensively so a future API that starts returning one is picked up
// without a code change.
type sandboxRegistryAPI struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
	UpdatedAt   string `json:"updated_at"`
	UpdatedBy   string `json:"updated_by"`
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
}

// sandboxRegistryPath builds the per-registry path. Registries are addressed by
// name, not by id, so the name has to be escaped before it goes into the URL.
func sandboxRegistryPath(name string) string {
	return "/v2/sandboxes/registries/" + url.PathEscape(name)
}

func (r *SandboxRegistryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox_registry"
}

func (r *SandboxRegistryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a private container image registry credential used by LangSmith sandboxes.\n\n" +
			"Registries are addressed **by name**, so changing `name` forces a new resource.\n\n" +
			"`username` and `password` are **write-only**: the LangSmith API accepts them on create/update but never " +
			"returns them. This has two consequences:\n\n" +
			"* Terraform cannot detect drift on `username` or `password`. If the credentials are changed outside of " +
			"Terraform, the provider will not notice.\n" +
			"* **Importing a registry cannot recover them.** Immediately after an import both attributes are absent " +
			"from state, so the next plan will show a change for them and re-write the credentials from your " +
			"configuration. This is expected; apply the plan to converge.\n\n" +
			"Import accepts the registry name:\n\n" +
			"```shell\n" +
			"terraform import langsmith_sandbox_registry.example my-registry\n" +
			"```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the registry.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the registry. This is the key the API addresses the registry by, so changing it forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The URL of the container image registry (for example `ghcr.io` or `123456789012.dkr.ecr.us-east-1.amazonaws.com`).",
				Required:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "The username used to authenticate to the registry. This is write-only: the API never returns it, " +
					"so Terraform cannot detect drift on it and cannot populate it on import.",
				Required: true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The password (or access token) used to authenticate to the registry. This is write-only: the API " +
					"never returns it, so Terraform cannot detect drift on it and cannot populate it on import. After importing a " +
					"registry, the first plan will show a change for this attribute and re-write the credentials from your configuration.",
				Required:  true,
				Sensitive: true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the registry.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "The identifier of the user who created the registry.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last modification timestamp of the registry.",
				Computed:            true,
			},
			"updated_by": schema.StringAttribute{
				MarkdownDescription: "The identifier of the user who last modified the registry.",
				Computed:            true,
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

func (r *SandboxRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// applyAPI copies the server-returned fields into the model. It deliberately
// leaves Username and Password alone -- the API does not return them, and
// whatever the model already carries (config on write, prior state on read)
// is the only truth we have.
func (r *SandboxRegistryResource) applyAPI(data *SandboxRegistryResourceModel, api *sandboxRegistryAPI) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.URL = types.StringValue(api.URL)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.CreatedBy = types.StringValue(api.CreatedBy)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	data.UpdatedBy = types.StringValue(api.UpdatedBy)
}

func (r *SandboxRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SandboxRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := sandboxRegistryCreateRequest{
		Name:     data.Name.ValueString(),
		URL:      data.URL.ValueString(),
		Username: data.Username.ValueString(),
		Password: data.Password.ValueString(),
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var api sandboxRegistryAPI
	if err := c.Post(ctx, "/v2/sandboxes/registries", body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating sandbox registry", err.Error())
		return
	}

	r.applyAPI(&data, &api)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), &resp.Diagnostics)
	tflog.Trace(ctx, "created sandbox registry resource", map[string]interface{}{"name": data.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SandboxRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Registries are fetched by name -- the id is along for the ride only.
	c := effectiveClient(r.client, data.WorkspaceID)
	var api sandboxRegistryAPI
	if err := c.Get(ctx, sandboxRegistryPath(data.Name.ValueString()), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading sandbox registry", err.Error())
		return
	}

	// applyAPI leaves username/password untouched: the API never hands them
	// back, so state keeps whatever it already held. Like the Long Branch
	// bartender, we don't repeat what we were told in confidence.
	r.applyAPI(&data, &api)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SandboxRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// name is RequiresReplace, so the planned name always equals the name the
	// registry is currently addressed by.
	body := sandboxRegistryUpdateRequest{
		URL:      data.URL.ValueString(),
		Username: data.Username.ValueString(),
		Password: data.Password.ValueString(),
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var api sandboxRegistryAPI
	if err := c.Patch(ctx, sandboxRegistryPath(data.Name.ValueString()), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating sandbox registry", err.Error())
		return
	}

	r.applyAPI(&data, &api)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), &resp.Diagnostics)
	tflog.Trace(ctx, "updated sandbox registry resource", map[string]interface{}{"name": data.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SandboxRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	if err := c.Delete(ctx, sandboxRegistryPath(data.Name.ValueString())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting sandbox registry", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted sandbox registry resource", map[string]interface{}{"name": data.Name.ValueString()})
}

// ImportState imports a sandbox registry by its name -- the API addresses
// registries by name, and Read resolves the object from the name alone, so the
// name (not the id) is what has to land in state here.
//
// The id is seeded with the name so the imported resource is addressable; the
// Read that immediately follows replaces it with the registry's real
// identifier.
//
// Fair warning: username and password are write-only and are never returned by
// the API, so they cannot be recovered on import. State will hold no values for
// them afterwards, and the first plan following the import will show a change
// and re-write the credentials from your configuration. That is expected.
func (r *SandboxRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected the registry name. Note: username and password are write-only and cannot be recovered on import -- "+
				"the next plan will re-write them from your configuration.",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

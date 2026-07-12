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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &OrgRoleResource{}
	_ resource.ResourceWithImportState = &OrgRoleResource{}
)

// NewOrgRoleResource returns a new OrgRoleResource, ready to pin a badge
// on whoever the marshal sees fit.
func NewOrgRoleResource() resource.Resource {
	return &OrgRoleResource{}
}

// OrgRoleResource manages organization roles in LangSmith -- the law of the
// land when it comes to who can do what in Dodge City.
type OrgRoleResource struct {
	client *client.Client
}

// OrgRoleResourceModel describes the Terraform state for an organization role.
type OrgRoleResourceModel struct {
	ID             types.String `tfsdk:"id"`
	DisplayName    types.String `tfsdk:"display_name"`
	Description    types.String `tfsdk:"description"`
	Permissions    types.String `tfsdk:"permissions"`
	Name           types.String `tfsdk:"name"`
	OrganizationID types.String `tfsdk:"organization_id"`
	AccessScope    types.String `tfsdk:"access_scope"`
	IsRestricted   types.Bool   `tfsdk:"is_restricted"`
}

// orgRoleCreateRequest is the paperwork for swearing in a new role at the
// marshal's office.
type orgRoleCreateRequest struct {
	DisplayName string          `json:"display_name"`
	Description *string         `json:"description,omitempty"`
	Permissions json.RawMessage `json:"permissions"`
}

// orgRoleUpdateRequest is the amendment filed when a role's duties change.
type orgRoleUpdateRequest struct {
	DisplayName string          `json:"display_name"`
	Description *string         `json:"description,omitempty"`
	Permissions json.RawMessage `json:"permissions"`
}

// orgRoleRestrictionRequest is the writ that marks a role restricted (or lifts
// the restriction). The role create/update bodies do NOT carry this flag --
// PUT /api/v1/orgs/current/roles/{role_id}/restriction is the only way in.
type orgRoleRestrictionRequest struct {
	IsRestricted bool `json:"is_restricted"`
}

// orgRoleAPIResponse is what the API telegraphs back about an organization role.
//
// IsRestricted is a *bool on purpose: the list endpoint always returns
// `is_restricted`, but the create/update responses may omit it. A nil pointer
// means "the API said nothing", which is very different from "the API said
// false" -- see the restriction reconciliation in Create/Update.
type orgRoleAPIResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	DisplayName    string          `json:"display_name"`
	Description    string          `json:"description"`
	OrganizationID string          `json:"organization_id"`
	Permissions    json.RawMessage `json:"permissions"`
	AccessScope    string          `json:"access_scope"`
	IsRestricted   *bool           `json:"is_restricted"`
}

// orgRoleListAPIResponse is the full roster -- every role on the books.
type orgRoleListAPIResponse []orgRoleAPIResponse

func (r *OrgRoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_role"
}

func (r *OrgRoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith organization role for RBAC.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the role.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the role.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the role.",
				Optional:            true,
			},
			"permissions": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of permissions assigned to the role.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The internal name of the role.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns this role.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"access_scope": schema.StringAttribute{
				MarkdownDescription: "The access scope of the role.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"is_restricted": schema.BoolAttribute{
				MarkdownDescription: "Whether the role is restricted. Restricted roles can only be assigned by organization admins " +
					"and are not offered as a general-purpose role. LangSmith does not accept this flag on the role create/update " +
					"payload, so the provider sets it with a follow-up call to `PUT /api/v1/orgs/current/roles/{role_id}/restriction`. " +
					"When omitted, the role keeps whatever the server defaults to (currently unrestricted).",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *OrgRoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrgRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured — expected a *client.Client. Ensure provider configuration and Configure() ran successfully.",
		)
		return
	}

	// Validate permissions is valid JSON before sending to the API.
	permStr := data.Permissions.ValueString()
	if !json.Valid([]byte(permStr)) {
		resp.Diagnostics.AddError("Invalid permissions JSON", "The `permissions` attribute must be valid JSON.")
		return
	}

	body := orgRoleCreateRequest{
		DisplayName: data.DisplayName.ValueString(),
		Permissions: json.RawMessage(permStr),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	// The restriction flag the practitioner asked for (may be null/unknown when
	// they left it out of the configuration).
	desired := data.IsRestricted

	var result orgRoleAPIResponse
	err := r.client.Post(ctx, "/api/v1/orgs/current/roles", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization role", err.Error())
		return
	}

	// The role exists now. From here on out we always write state, even if the
	// restriction call goes sideways -- otherwise Terraform loses track of a
	// role that is very much alive on the server.
	restricted, err := r.reconcileOrgRoleRestriction(ctx, result.ID, desired, result.IsRestricted, types.BoolNull())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error setting organization role restriction",
			fmt.Sprintf("The role %q was created, but its `is_restricted` flag could not be set: %s", result.ID, err),
		)
	}

	mapOrgRoleResponseToState(&data, &result)
	data.IsRestricted = types.BoolValue(restricted)
	tflog.Trace(ctx, "created organization role resource", map[string]interface{}{"id": result.ID, "is_restricted": restricted})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured — expected a *client.Client. Ensure provider configuration and Configure() ran successfully.",
		)
		return
	}

	// The API only offers a list endpoint -- no direct lookup by ID.
	// We have to ride through the whole posse and find our man.
	found, err := r.findOrgRole(ctx, data.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			// Treat 404 as resource gone.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading organization roles", err.Error())
		return
	}

	if found == nil {
		// The role has left town without a trace.
		resp.State.RemoveResource(ctx)
		return
	}

	mapOrgRoleResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *OrgRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state OrgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured — expected a *client.Client. Ensure provider configuration and Configure() ran successfully.",
		)
		return
	}

	// Validate permissions is valid JSON before sending to the API.
	permStr := data.Permissions.ValueString()
	if !json.Valid([]byte(permStr)) {
		resp.Diagnostics.AddError("Invalid permissions JSON", "The `permissions` attribute must be valid JSON.")
		return
	}

	body := orgRoleUpdateRequest{
		DisplayName: data.DisplayName.ValueString(),
		Permissions: json.RawMessage(permStr),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	desired := data.IsRestricted

	var result orgRoleAPIResponse
	err := r.client.Patch(ctx, "/api/v1/orgs/current/roles/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization role", err.Error())
		return
	}

	// `is_restricted` rides on its own endpoint -- the PATCH above never
	// carries it. Only wire up the restriction call when the value actually
	// moved. Note this is deliberately NOT a replacement: restriction is
	// updatable in place.
	restricted, err := r.reconcileOrgRoleRestriction(ctx, data.ID.ValueString(), desired, result.IsRestricted, state.IsRestricted)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization role restriction", err.Error())
	}

	mapOrgRoleResponseToState(&data, &result)
	data.IsRestricted = types.BoolValue(restricted)
	tflog.Trace(ctx, "updated organization role resource", map[string]interface{}{"id": result.ID, "is_restricted": restricted})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/orgs/current/roles/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting organization role", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted organization role resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *OrgRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// findOrgRole rides the whole roster looking for one role by ID. The LangSmith
// API has no single-GET endpoint for org roles, so a linear search of
// GET /api/v1/orgs/current/roles is the only game in town. Returns (nil, nil)
// when the role simply isn't on the books.
func (r *OrgRoleResource) findOrgRole(ctx context.Context, roleID string) (*orgRoleAPIResponse, error) {
	var listResult orgRoleListAPIResponse
	if err := r.client.Get(ctx, "/api/v1/orgs/current/roles", nil, &listResult); err != nil {
		return nil, err
	}

	for i := range listResult {
		if listResult[i].ID == roleID {
			return &listResult[i], nil
		}
	}

	return nil, nil
}

// setOrgRoleRestriction throws the restriction switch. This is the only
// endpoint that accepts the flag -- the role create/update bodies ignore it.
func (r *OrgRoleResource) setOrgRoleRestriction(ctx context.Context, roleID string, restricted bool) error {
	return r.client.Put(
		ctx,
		"/api/v1/orgs/current/roles/"+roleID+"/restriction",
		orgRoleRestrictionRequest{IsRestricted: restricted},
		nil,
	)
}

// reconcileOrgRoleRestriction settles the difference between what the
// practitioner asked for (desired) and what the server currently says, calling
// the restriction endpoint only when the two disagree. It always returns the
// value that belongs in state -- known, never unknown -- so callers can set
// `is_restricted` unconditionally.
//
// apiValue is the flag as returned by the create/update response (nil when the
// response omitted it); prior is the value already in state (null on create).
// When neither source knows and the practitioner didn't ask for anything, we
// fall back to reading the role back off the list endpoint.
func (r *OrgRoleResource) reconcileOrgRoleRestriction(ctx context.Context, roleID string, desired types.Bool, apiValue *bool, prior types.Bool) (bool, error) {
	asked := !desired.IsNull() && !desired.IsUnknown()

	// Figure out where the server stands right now.
	var current bool
	switch {
	case apiValue != nil:
		current = *apiValue
	case !prior.IsNull() && !prior.IsUnknown():
		current = prior.ValueBool()
	case asked:
		// Nothing to compare against, but we know what we want: just write it.
		if err := r.setOrgRoleRestriction(ctx, roleID, desired.ValueBool()); err != nil {
			return false, err
		}
		return desired.ValueBool(), nil
	default:
		// Nobody has an opinion -- take whatever the server defaulted to.
		role, err := r.findOrgRole(ctx, roleID)
		if err != nil {
			return false, err
		}
		if role != nil && role.IsRestricted != nil {
			return *role.IsRestricted, nil
		}
		return false, nil
	}

	if !asked || desired.ValueBool() == current {
		return current, nil
	}

	if err := r.setOrgRoleRestriction(ctx, roleID, desired.ValueBool()); err != nil {
		return current, err
	}

	return desired.ValueBool(), nil
}

// mapOrgRoleResponseToState brands the Terraform state with the API response,
// handling optional fields the way Matt Dillon handles trouble -- carefully and
// with an eye for what's missing.
func mapOrgRoleResponseToState(data *OrgRoleResourceModel, result *orgRoleAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.Name = types.StringValue(result.Name)
	data.OrganizationID = types.StringValue(result.OrganizationID)
	// AccessScope may be omitted or returned as an empty string by some API
	// responses (for example on update). Avoid clobbering an existing
	// non-empty state value with an empty string — preserve the prior
	// state unless the API returns a meaningful value.
	if result.AccessScope != "" {
		data.AccessScope = types.StringValue(result.AccessScope)
	}

	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}

	// is_restricted must never be left unknown -- Terraform hard-fails on an
	// unknown value after apply. Trust the API when it speaks; otherwise hold
	// on to a known prior value, and fall back to false only when we have
	// nothing at all to go on (Create/Update overwrite this with the reconciled
	// value anyway).
	switch {
	case result.IsRestricted != nil:
		data.IsRestricted = types.BoolValue(*result.IsRestricted)
	case data.IsRestricted.IsNull() || data.IsRestricted.IsUnknown():
		data.IsRestricted = types.BoolValue(false)
	}

	data.Permissions = jsonStringValue(result.Permissions)
}

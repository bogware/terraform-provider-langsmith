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

// orgRoleAPIResponse is what the API telegraphs back about an organization role.
type orgRoleAPIResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	DisplayName    string          `json:"display_name"`
	Description    string          `json:"description"`
	OrganizationID string          `json:"organization_id"`
	Permissions    json.RawMessage `json:"permissions"`
	AccessScope    string          `json:"access_scope"`
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
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID that owns this role.",
				Computed:            true,
			},
			"access_scope": schema.StringAttribute{
				MarkdownDescription: "The access scope of the role.",
				Computed:            true,
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

	body := orgRoleCreateRequest{
		DisplayName: data.DisplayName.ValueString(),
		Permissions: json.RawMessage(data.Permissions.ValueString()),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	var result orgRoleAPIResponse
	err := r.client.Post(ctx, "/api/v1/orgs/current/roles", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization role", err.Error())
		return
	}

	mapOrgRoleResponseToState(&data, &result)
	tflog.Trace(ctx, "created organization role resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API only offers a list endpoint -- no direct lookup by ID.
	// We have to ride through the whole posse and find our man.
	var listResult orgRoleListAPIResponse
	err := r.client.Get(ctx, "/api/v1/orgs/current/roles", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization roles", err.Error())
		return
	}

	var found *orgRoleAPIResponse
	for _, role := range listResult {
		if role.ID == data.ID.ValueString() {
			found = &role
			break
		}
	}

	if found == nil {
		// The role has left town without a trace.
		resp.State.RemoveResource(ctx)
		return
	}

	mapOrgRoleResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := orgRoleUpdateRequest{
		DisplayName: data.DisplayName.ValueString(),
		Permissions: json.RawMessage(data.Permissions.ValueString()),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	var result orgRoleAPIResponse
	err := r.client.Patch(ctx, "/api/v1/orgs/current/roles/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization role", err.Error())
		return
	}

	mapOrgRoleResponseToState(&data, &result)
	tflog.Trace(ctx, "updated organization role resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrgRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/orgs/current/roles/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting organization role", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted organization role resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *OrgRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapOrgRoleResponseToState brands the Terraform state with the API response,
// handling optional fields the way Matt Dillon handles trouble -- carefully and
// with an eye for what's missing.
func mapOrgRoleResponseToState(data *OrgRoleResourceModel, result *orgRoleAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.Name = types.StringValue(result.Name)
	data.OrganizationID = types.StringValue(result.OrganizationID)
	data.AccessScope = types.StringValue(result.AccessScope)

	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}

	if len(result.Permissions) > 0 && string(result.Permissions) != "null" {
		data.Permissions = types.StringValue(string(result.Permissions))
	} else {
		data.Permissions = types.StringNull()
	}
}

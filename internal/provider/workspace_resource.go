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
	_ resource.Resource                = &WorkspaceResource{}
	_ resource.ResourceWithImportState = &WorkspaceResource{}
)

// NewWorkspaceResource returns a new WorkspaceResource -- your own plot of land in LangSmith.
func NewWorkspaceResource() resource.Resource {
	return &WorkspaceResource{}
}

// WorkspaceResource implements CRUD for LangSmith workspaces,
// the territories where teams stake their claims and do their work.
type WorkspaceResource struct {
	client *client.Client
}

// WorkspaceResourceModel describes the Terraform state for a workspace.
type WorkspaceResourceModel struct {
	ID             types.String `tfsdk:"id"`
	DisplayName    types.String `tfsdk:"display_name"`
	TenantHandle   types.String `tfsdk:"tenant_handle"`
	CreatedAt      types.String `tfsdk:"created_at"`
	OrganizationID types.String `tfsdk:"organization_id"`
	IsPersonal     types.Bool   `tfsdk:"is_personal"`
}

// workspaceCreateRequest is the deed for establishing a new workspace.
type workspaceCreateRequest struct {
	DisplayName  string  `json:"display_name"`
	TenantHandle *string `json:"tenant_handle,omitempty"`
}

// workspaceUpdateRequest carries the fields allowed when renaming your territory.
type workspaceUpdateRequest struct {
	DisplayName string `json:"display_name"`
}

// workspaceAPIResponse is what the API returns when you inquire about a workspace —
// the full survey of the territory, including who owns it and whether it is personal land.
type workspaceAPIResponse struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	TenantHandle   string `json:"tenant_handle"`
	CreatedAt      string `json:"created_at"`
	OrganizationID string `json:"organization_id"`
	IsPersonal     bool   `json:"is_personal"`
}

func (r *WorkspaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *WorkspaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the workspace.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the workspace.",
				Required:            true,
			},
			"tenant_handle": schema.StringAttribute{
				MarkdownDescription: "The workspace handle/slug.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the workspace was created.",
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization that owns this workspace — the ranch brand on the deed.",
				Computed:            true,
			},
			"is_personal": schema.BoolAttribute{
				MarkdownDescription: "Whether this workspace belongs to a single soul or the whole outfit.",
				Computed:            true,
			},
		},
	}
}

func (r *WorkspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := workspaceCreateRequest{
		DisplayName: data.DisplayName.ValueString(),
	}

	if !data.TenantHandle.IsNull() && !data.TenantHandle.IsUnknown() {
		v := data.TenantHandle.ValueString()
		body.TenantHandle = &v
	}

	var result workspaceAPIResponse
	err := r.client.Post(ctx, "/api/v1/workspaces", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating workspace", err.Error())
		return
	}

	mapWorkspaceResponseToState(&data, &result)
	tflog.Trace(ctx, "created workspace resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var workspaces []workspaceAPIResponse
	err := r.client.Get(ctx, "/api/v1/workspaces", nil, &workspaces)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading workspace", err.Error())
		return
	}

	var found *workspaceAPIResponse
	for i := range workspaces {
		if workspaces[i].ID == data.ID.ValueString() {
			found = &workspaces[i]
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapWorkspaceResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := workspaceUpdateRequest{
		DisplayName: data.DisplayName.ValueString(),
	}

	var result workspaceAPIResponse
	err := r.client.Patch(ctx, "/api/v1/workspaces/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating workspace", err.Error())
		return
	}

	mapWorkspaceResponseToState(&data, &result)
	tflog.Trace(ctx, "updated workspace resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/workspaces/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting workspace", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted workspace resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *WorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapWorkspaceResponseToState translates the API response into Terraform state,
// plain and simple like a deputy filing a report.
func mapWorkspaceResponseToState(data *WorkspaceResourceModel, result *workspaceAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.TenantHandle = types.StringValue(result.TenantHandle)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.OrganizationID = types.StringValue(result.OrganizationID)
	data.IsPersonal = types.BoolValue(result.IsPersonal)
}

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
	_ resource.Resource                = &ServiceAccountResource{}
	_ resource.ResourceWithImportState = &ServiceAccountResource{}
)

// NewServiceAccountResource returns a new ServiceAccountResource.
func NewServiceAccountResource() resource.Resource {
	return &ServiceAccountResource{}
}

// ServiceAccountResource defines the resource implementation.
type ServiceAccountResource struct {
	client *client.Client
}

// ServiceAccountResourceModel describes the resource data model.
type ServiceAccountResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	DefaultWorkspaceID types.String `tfsdk:"default_workspace_id"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

// serviceAccountAPICreateRequest is the request body for creating a service account.
type serviceAccountAPICreateRequest struct {
	Name string `json:"name"`
}

// serviceAccountAPIResponse is the API response for a single service account.
type serviceAccountAPIResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	OrganizationID     string `json:"organization_id"`
	DefaultWorkspaceID string `json:"default_workspace_id"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// serviceAccountListAPIResponse is the API response for listing service accounts.
type serviceAccountListAPIResponse []serviceAccountAPIResponse

func (r *ServiceAccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *ServiceAccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith service account. Service accounts cannot be updated; changing any mutable attribute will force recreation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the service account.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the service account.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID of the service account.",
				Computed:            true,
			},
			"default_workspace_id": schema.StringAttribute{
				MarkdownDescription: "The default workspace ID of the service account.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the service account.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last update timestamp of the service account.",
				Computed:            true,
			},
		},
	}
}

func (r *ServiceAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := serviceAccountAPICreateRequest{
		Name: data.Name.ValueString(),
	}

	var result serviceAccountAPIResponse
	err := r.client.Post(ctx, "/api/v1/service-accounts", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating service account", err.Error())
		return
	}

	mapServiceAccountResponseToState(&data, &result)
	tflog.Trace(ctx, "created service account resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var listResult serviceAccountListAPIResponse
	err := r.client.Get(ctx, "/api/v1/service-accounts", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service accounts", err.Error())
		return
	}

	var found *serviceAccountAPIResponse
	for _, sa := range listResult {
		if sa.ID == data.ID.ValueString() {
			found = &sa
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapServiceAccountResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Service accounts cannot be updated. This is unexpected — all mutable attributes should have RequiresReplace set.",
	)
}

func (r *ServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/service-accounts/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting service account", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted service account resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapServiceAccountResponseToState maps an API response to the Terraform state model.
func mapServiceAccountResponseToState(data *ServiceAccountResourceModel, result *serviceAccountAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.OrganizationID = types.StringValue(result.OrganizationID)
	data.DefaultWorkspaceID = types.StringValue(result.DefaultWorkspaceID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

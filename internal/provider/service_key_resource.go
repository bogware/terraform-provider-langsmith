// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &ServiceKeyResource{}
	_ resource.ResourceWithImportState = &ServiceKeyResource{}
)

// NewServiceKeyResource returns a new ServiceKeyResource.
func NewServiceKeyResource() resource.Resource {
	return &ServiceKeyResource{}
}

// ServiceKeyResource defines the resource implementation.
type ServiceKeyResource struct {
	client *client.Client
}

// ServiceKeyResourceModel describes the resource data model.
type ServiceKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Description types.String `tfsdk:"description"`
	ReadOnly    types.Bool   `tfsdk:"read_only"`
	ShortKey    types.String `tfsdk:"short_key"`
	Key         types.String `tfsdk:"key"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

// serviceKeyAPICreateRequest is the request body for creating a service key.
type serviceKeyAPICreateRequest struct {
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
}

// serviceKeyAPICreateResponse is the API response for creating a service key.
type serviceKeyAPICreateResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
	ShortKey    string `json:"short_key"`
	Key         string `json:"key"`
	CreatedAt   string `json:"created_at"`
}

// serviceKeyAPIListItem is a single item in the service keys list response.
type serviceKeyAPIListItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
	ShortKey    string `json:"short_key"`
	CreatedAt   string `json:"created_at"`
}

// serviceKeyAPIListResponse is the API response for listing service keys.
type serviceKeyAPIListResponse []serviceKeyAPIListItem

func (r *ServiceKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_key"
}

func (r *ServiceKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith service key (API key). Service keys cannot be updated; changing any mutable attribute will force recreation. The full API key is only available at creation time.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the service key.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description for the service key.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Default API key"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"read_only": schema.BoolAttribute{
				MarkdownDescription: "Whether the service key is read-only.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"short_key": schema.StringAttribute{
				MarkdownDescription: "The shortened version of the API key for display purposes.",
				Computed:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The full API key. Only available at creation time; will be empty after import.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the service key.",
				Computed:            true,
			},
		},
	}
}

func (r *ServiceKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServiceKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := serviceKeyAPICreateRequest{
		Description: data.Description.ValueString(),
		ReadOnly:    data.ReadOnly.ValueBool(),
	}

	var result serviceKeyAPICreateResponse
	err := r.client.Post(ctx, "/api/v1/orgs/current/service-keys", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating service key", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Description = types.StringValue(result.Description)
	data.ReadOnly = types.BoolValue(result.ReadOnly)
	data.ShortKey = types.StringValue(result.ShortKey)
	data.Key = types.StringValue(result.Key)
	data.CreatedAt = types.StringValue(result.CreatedAt)

	tflog.Trace(ctx, "created service key resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var listResult serviceKeyAPIListResponse
	err := r.client.Get(ctx, "/api/v1/orgs/current/service-keys", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service keys", err.Error())
		return
	}

	var found *serviceKeyAPIListItem
	for _, sk := range listResult {
		if sk.ID == data.ID.ValueString() {
			found = &sk
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(found.ID)
	data.Description = types.StringValue(found.Description)
	data.ReadOnly = types.BoolValue(found.ReadOnly)
	data.ShortKey = types.StringValue(found.ShortKey)
	data.CreatedAt = types.StringValue(found.CreatedAt)
	// key is not returned on read; preserve existing state value (UseStateForUnknown handles this)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Service keys cannot be updated. This is unexpected — all mutable attributes should have RequiresReplace set.",
	)
}

func (r *ServiceKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/orgs/current/service-keys/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting service key", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted service key resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ServiceKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

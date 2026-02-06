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
	_ resource.Resource                = &TagKeyResource{}
	_ resource.ResourceWithImportState = &TagKeyResource{}
)

// NewTagKeyResource returns a new TagKeyResource, fresh from the smithy.
func NewTagKeyResource() resource.Resource {
	return &TagKeyResource{}
}

// TagKeyResource manages tag keys in LangSmith -- the branding irons
// you use to mark and organize your resources.
type TagKeyResource struct {
	client *client.Client
}

// TagKeyResourceModel describes the Terraform state for a tag key.
type TagKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Key         types.String `tfsdk:"key"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

// tagKeyCreateRequest is the order form for forging a new tag key.
type tagKeyCreateRequest struct {
	Key         string  `json:"key"`
	Description *string `json:"description,omitempty"`
}

// tagKeyUpdateRequest is the request for re-stamping an existing tag key.
type tagKeyUpdateRequest struct {
	Key         string  `json:"key"`
	Description *string `json:"description,omitempty"`
}

// tagKeyAPIResponse is the API's account of a tag key and its particulars.
type tagKeyAPIResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (r *TagKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_key"
}

func (r *TagKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith tag key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the tag key.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The tag key name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the tag key.",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag key was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag key was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *TagKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TagKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TagKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := tagKeyCreateRequest{
		Key: data.Key.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	var result tagKeyAPIResponse
	err := r.client.Post(ctx, "/api/v1/workspaces/current/tag-keys", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating tag key", err.Error())
		return
	}

	mapTagKeyResponseToState(&data, &result)
	tflog.Trace(ctx, "created tag key resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TagKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result tagKeyAPIResponse
	err := r.client.Get(ctx, "/api/v1/workspaces/current/tag-keys/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading tag key", err.Error())
		return
	}

	mapTagKeyResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TagKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := tagKeyUpdateRequest{
		Key: data.Key.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	var result tagKeyAPIResponse
	err := r.client.Patch(ctx, "/api/v1/workspaces/current/tag-keys/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating tag key", err.Error())
		return
	}

	mapTagKeyResponseToState(&data, &result)
	tflog.Trace(ctx, "updated tag key resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TagKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/workspaces/current/tag-keys/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting tag key", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted tag key resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *TagKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapTagKeyResponseToState brands the Terraform state with values from the API response,
// leaving description null if the API came back empty-handed.
func mapTagKeyResponseToState(data *TagKeyResourceModel, result *tagKeyAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Key = types.StringValue(result.Key)

	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

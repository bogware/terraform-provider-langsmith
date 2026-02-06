// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                = &TagValueResource{}
	_ resource.ResourceWithImportState = &TagValueResource{}
)

// NewTagValueResource returns a new TagValueResource.
func NewTagValueResource() resource.Resource {
	return &TagValueResource{}
}

// TagValueResource defines the resource implementation.
type TagValueResource struct {
	client *client.Client
}

// TagValueResourceModel describes the resource data model.
type TagValueResourceModel struct {
	ID          types.String `tfsdk:"id"`
	TagKeyID    types.String `tfsdk:"tag_key_id"`
	Value       types.String `tfsdk:"value"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

// tagValueCreateRequest is the request body for creating a tag value.
type tagValueCreateRequest struct {
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

// tagValueUpdateRequest is the request body for updating a tag value.
type tagValueUpdateRequest struct {
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

// tagValueAPIResponse is the API response for a tag value.
type tagValueAPIResponse struct {
	ID          string `json:"id"`
	Value       string `json:"value"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (r *TagValueResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_value"
}

func (r *TagValueResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith tag value within a tag key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the tag value.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tag_key_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the parent tag key.",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The tag value.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the tag value.",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag value was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the tag value was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *TagValueResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TagValueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TagValueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := tagValueCreateRequest{
		Value: data.Value.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	apiPath := fmt.Sprintf("/api/v1/workspaces/current/tag-keys/%s/tag-values", data.TagKeyID.ValueString())

	var result tagValueAPIResponse
	err := r.client.Post(ctx, apiPath, body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating tag value", err.Error())
		return
	}

	mapTagValueResponseToState(&data, &result)
	tflog.Trace(ctx, "created tag value resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagValueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TagValueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/api/v1/workspaces/current/tag-keys/%s/tag-values/%s",
		data.TagKeyID.ValueString(), data.ID.ValueString())

	var result tagValueAPIResponse
	err := r.client.Get(ctx, apiPath, nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading tag value", err.Error())
		return
	}

	mapTagValueResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagValueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TagValueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := tagValueUpdateRequest{
		Value: data.Value.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}

	apiPath := fmt.Sprintf("/api/v1/workspaces/current/tag-keys/%s/tag-values/%s",
		data.TagKeyID.ValueString(), data.ID.ValueString())

	var result tagValueAPIResponse
	err := r.client.Patch(ctx, apiPath, body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating tag value", err.Error())
		return
	}

	mapTagValueResponseToState(&data, &result)
	tflog.Trace(ctx, "updated tag value resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagValueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TagValueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/api/v1/workspaces/current/tag-keys/%s/tag-values/%s",
		data.TagKeyID.ValueString(), data.ID.ValueString())

	err := r.client.Delete(ctx, apiPath)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting tag value", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted tag value resource", map[string]interface{}{"id": data.ID.ValueString()})
}

// ImportState handles importing a tag value resource.
// The import ID format is "tag_key_id/tag_value_id".
func (r *TagValueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'tag_key_id/tag_value_id', got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag_key_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// mapTagValueResponseToState maps an API response to the Terraform state model.
func mapTagValueResponseToState(data *TagValueResourceModel, result *tagValueAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Value = types.StringValue(result.Value)

	if result.Description != "" {
		data.Description = types.StringValue(result.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

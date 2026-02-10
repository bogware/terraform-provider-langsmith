// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
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
	_ resource.Resource                = &TaggingResource{}
	_ resource.ResourceWithImportState = &TaggingResource{}
)

func NewTaggingResource() resource.Resource {
	return &TaggingResource{}
}

type TaggingResource struct {
	client *client.Client
}

type TaggingResourceModel struct {
	ID           types.String `tfsdk:"id"`
	TagValueID   types.String `tfsdk:"tag_value_id"`
	ResourceType types.String `tfsdk:"resource_type"`
	ResourceID   types.String `tfsdk:"resource_id"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

type taggingCreateRequest struct {
	TagValueID   string `json:"tag_value_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

type taggingAPIResponse struct {
	ID           string `json:"id"`
	TagValueID   string `json:"tag_value_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	CreatedAt    string `json:"created_at"`
}

func (r *TaggingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagging"
}

func (r *TaggingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith tagging, which associates a tag value with a resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the tagging.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tag_value_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the tag value to apply.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resource_type": schema.StringAttribute{
				MarkdownDescription: "The type of resource to tag. Valid values: `alert`, `dashboard`, `dataset`, `deployment`, `experiment`, `project`, `prompt`, `queue`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.OneOf("alert", "dashboard", "dataset", "deployment", "experiment", "project", "prompt", "queue")},
			},
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the resource to tag.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *TaggingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *TaggingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TaggingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := taggingCreateRequest{
		TagValueID:   data.TagValueID.ValueString(),
		ResourceType: data.ResourceType.ValueString(),
		ResourceID:   data.ResourceID.ValueString(),
	}

	var result taggingAPIResponse
	err := r.client.Post(ctx, "/api/v1/workspaces/current/taggings", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating tagging", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.TagValueID = types.StringValue(result.TagValueID)
	data.ResourceType = types.StringValue(result.ResourceType)
	data.ResourceID = types.StringValue(result.ResourceID)
	data.CreatedAt = types.StringValue(result.CreatedAt)

	tflog.Trace(ctx, "created tagging resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaggingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TaggingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The tagging API returns a list; we search for our specific tagging by ID.
	var results []taggingAPIResponse
	err := r.client.Get(ctx, "/api/v1/workspaces/current/taggings", nil, &results)
	if err != nil {
		resp.Diagnostics.AddError("Error reading taggings", err.Error())
		return
	}

	var found *taggingAPIResponse
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

	data.ID = types.StringValue(found.ID)
	data.TagValueID = types.StringValue(found.TagValueID)
	data.ResourceType = types.StringValue(found.ResourceType)
	data.ResourceID = types.StringValue(found.ResourceID)
	data.CreatedAt = types.StringValue(found.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaggingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Taggings cannot be updated. All attributes have RequiresReplace set.",
	)
}

func (r *TaggingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TaggingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/workspaces/current/taggings/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting tagging", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted tagging resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *TaggingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

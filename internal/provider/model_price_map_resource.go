// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &ModelPriceMapResource{}
	_ resource.ResourceWithImportState = &ModelPriceMapResource{}
)

// NewModelPriceMapResource returns a new ModelPriceMapResource.
func NewModelPriceMapResource() resource.Resource {
	return &ModelPriceMapResource{}
}

// ModelPriceMapResource defines the resource implementation.
type ModelPriceMapResource struct {
	client *client.Client
}

// ModelPriceMapResourceModel describes the resource data model.
type ModelPriceMapResourceModel struct {
	ID             types.String  `tfsdk:"id"`
	Name           types.String  `tfsdk:"name"`
	MatchPattern   types.String  `tfsdk:"match_pattern"`
	PromptCost     types.Float64 `tfsdk:"prompt_cost"`
	CompletionCost types.Float64 `tfsdk:"completion_cost"`
	Provider       types.String  `tfsdk:"model_provider"`
	StartTime      types.String  `tfsdk:"start_time"`
	MatchPath      types.List    `tfsdk:"match_path"`
}

// modelPriceMapAPIRequest is the request body for creating/updating a model price map.
type modelPriceMapAPIRequest struct {
	Name           string   `json:"name"`
	MatchPattern   string   `json:"match_pattern"`
	PromptCost     float64  `json:"prompt_cost"`
	CompletionCost float64  `json:"completion_cost"`
	Provider       *string  `json:"provider,omitempty"`
	StartTime      *string  `json:"start_time,omitempty"`
	MatchPath      []string `json:"match_path,omitempty"`
}

// modelPriceMapAPIResponse is the API response for a model price map.
type modelPriceMapAPIResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	MatchPattern   string   `json:"match_pattern"`
	PromptCost     float64  `json:"prompt_cost"`
	CompletionCost float64  `json:"completion_cost"`
	Provider       *string  `json:"provider"`
	StartTime      *string  `json:"start_time"`
	MatchPath      []string `json:"match_path"`
}

func (r *ModelPriceMapResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_price_map"
}

func (r *ModelPriceMapResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith model price map entry for tracking costs of LLM usage.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the model price map entry.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The model name.",
				Required:            true,
			},
			"match_pattern": schema.StringAttribute{
				MarkdownDescription: "A regex pattern to match model names.",
				Required:            true,
			},
			"prompt_cost": schema.Float64Attribute{
				MarkdownDescription: "The cost per prompt token.",
				Required:            true,
			},
			"completion_cost": schema.Float64Attribute{
				MarkdownDescription: "The cost per completion token.",
				Required:            true,
			},
			"model_provider": schema.StringAttribute{
				MarkdownDescription: "The model provider name (e.g., `openai`, `anthropic`).",
				Optional:            true,
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The effective start time for this price map entry.",
				Optional:            true,
			},
			"match_path": schema.ListAttribute{
				MarkdownDescription: "Paths to match for model identification. Defaults to `[\"model\", \"model_name\", \"model_id\", \"model_path\", \"endpoint_name\"]`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *ModelPriceMapResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ModelPriceMapResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ModelPriceMapResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := modelPriceMapAPIRequest{
		Name:           data.Name.ValueString(),
		MatchPattern:   data.MatchPattern.ValueString(),
		PromptCost:     data.PromptCost.ValueFloat64(),
		CompletionCost: data.CompletionCost.ValueFloat64(),
	}

	if !data.Provider.IsNull() && !data.Provider.IsUnknown() {
		v := data.Provider.ValueString()
		body.Provider = &v
	}
	if !data.StartTime.IsNull() && !data.StartTime.IsUnknown() {
		v := data.StartTime.ValueString()
		body.StartTime = &v
	}
	if !data.MatchPath.IsNull() && !data.MatchPath.IsUnknown() {
		var matchPath []string
		resp.Diagnostics.Append(data.MatchPath.ElementsAs(ctx, &matchPath, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.MatchPath = matchPath
	}

	var result modelPriceMapAPIResponse
	err := r.client.Post(ctx, "/api/v1/model-price-map", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating model price map", err.Error())
		return
	}

	mapModelPriceMapResponseToState(ctx, &data, &result, &resp.Diagnostics)
	tflog.Trace(ctx, "created model price map resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelPriceMapResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ModelPriceMapResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []modelPriceMapAPIResponse
	err := r.client.Get(ctx, "/api/v1/model-price-map", nil, &results)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading model price map", err.Error())
		return
	}

	var found *modelPriceMapAPIResponse
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

	mapModelPriceMapResponseToState(ctx, &data, found, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelPriceMapResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ModelPriceMapResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := modelPriceMapAPIRequest{
		Name:           data.Name.ValueString(),
		MatchPattern:   data.MatchPattern.ValueString(),
		PromptCost:     data.PromptCost.ValueFloat64(),
		CompletionCost: data.CompletionCost.ValueFloat64(),
	}

	if !data.Provider.IsNull() && !data.Provider.IsUnknown() {
		v := data.Provider.ValueString()
		body.Provider = &v
	}
	if !data.StartTime.IsNull() && !data.StartTime.IsUnknown() {
		v := data.StartTime.ValueString()
		body.StartTime = &v
	}
	if !data.MatchPath.IsNull() && !data.MatchPath.IsUnknown() {
		var matchPath []string
		resp.Diagnostics.Append(data.MatchPath.ElementsAs(ctx, &matchPath, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.MatchPath = matchPath
	}

	var result modelPriceMapAPIResponse
	err := r.client.Put(ctx, "/api/v1/model-price-map/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating model price map", err.Error())
		return
	}

	mapModelPriceMapResponseToState(ctx, &data, &result, &resp.Diagnostics)
	tflog.Trace(ctx, "updated model price map resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ModelPriceMapResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModelPriceMapResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/model-price-map/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting model price map", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted model price map resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ModelPriceMapResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapModelPriceMapResponseToState maps an API response to the Terraform state model.
func mapModelPriceMapResponseToState(ctx context.Context, data *ModelPriceMapResourceModel, result *modelPriceMapAPIResponse, diagnostics *diag.Diagnostics) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.MatchPattern = types.StringValue(result.MatchPattern)
	data.PromptCost = types.Float64Value(result.PromptCost)
	data.CompletionCost = types.Float64Value(result.CompletionCost)

	if result.Provider != nil {
		data.Provider = types.StringValue(*result.Provider)
	} else {
		data.Provider = types.StringNull()
	}

	if result.StartTime != nil {
		data.StartTime = types.StringValue(*result.StartTime)
	} else {
		data.StartTime = types.StringNull()
	}

	if len(result.MatchPath) > 0 {
		matchPathList, diags := types.ListValueFrom(ctx, types.StringType, result.MatchPath)
		diagnostics.Append(diags...)
		data.MatchPath = matchPathList
	} else {
		data.MatchPath = types.ListNull(types.StringType)
	}
}

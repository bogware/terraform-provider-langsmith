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
	_ resource.Resource                = &UsageLimitResource{}
	_ resource.ResourceWithImportState = &UsageLimitResource{}
)

// NewUsageLimitResource returns a new UsageLimitResource.
func NewUsageLimitResource() resource.Resource {
	return &UsageLimitResource{}
}

// UsageLimitResource defines the resource implementation.
type UsageLimitResource struct {
	client *client.Client
}

// UsageLimitResourceModel describes the resource data model.
type UsageLimitResourceModel struct {
	ID         types.String `tfsdk:"id"`
	LimitType  types.String `tfsdk:"limit_type"`
	LimitValue types.Int64  `tfsdk:"limit_value"`
	TenantID   types.String `tfsdk:"tenant_id"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

// usageLimitAPIRequest is the request body for creating/updating a usage limit.
type usageLimitAPIRequest struct {
	LimitType  string `json:"limit_type"`
	LimitValue int64  `json:"limit_value"`
}

// usageLimitAPIResponse is the API response for a usage limit.
type usageLimitAPIResponse struct {
	ID         string `json:"id"`
	LimitType  string `json:"limit_type"`
	LimitValue int64  `json:"limit_value"`
	TenantID   string `json:"tenant_id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func (r *UsageLimitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_usage_limit"
}

func (r *UsageLimitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith usage limit.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the usage limit.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"limit_type": schema.StringAttribute{
				MarkdownDescription: "The type of usage limit.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"limit_value": schema.Int64Attribute{
				MarkdownDescription: "The limit value.",
				Required:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The last update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (r *UsageLimitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UsageLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UsageLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := usageLimitAPIRequest{
		LimitType:  data.LimitType.ValueString(),
		LimitValue: data.LimitValue.ValueInt64(),
	}

	var result usageLimitAPIResponse
	err := r.client.Put(ctx, "/api/v1/usage-limits", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating usage limit", err.Error())
		return
	}

	mapUsageLimitResponseToState(&data, &result)
	tflog.Trace(ctx, "created usage limit resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UsageLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UsageLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var results []usageLimitAPIResponse
	err := r.client.Get(ctx, "/api/v1/usage-limits", nil, &results)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading usage limit", err.Error())
		return
	}

	var found *usageLimitAPIResponse
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

	mapUsageLimitResponseToState(&data, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UsageLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UsageLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := usageLimitAPIRequest{
		LimitType:  data.LimitType.ValueString(),
		LimitValue: data.LimitValue.ValueInt64(),
	}

	var result usageLimitAPIResponse
	err := r.client.Put(ctx, "/api/v1/usage-limits", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating usage limit", err.Error())
		return
	}

	mapUsageLimitResponseToState(&data, &result)
	tflog.Trace(ctx, "updated usage limit resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UsageLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UsageLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/usage-limits/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting usage limit", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted usage limit resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *UsageLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapUsageLimitResponseToState maps an API response to the Terraform state model.
func mapUsageLimitResponseToState(data *UsageLimitResourceModel, result *usageLimitAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.LimitType = types.StringValue(result.LimitType)
	data.LimitValue = types.Int64Value(result.LimitValue)
	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)
}

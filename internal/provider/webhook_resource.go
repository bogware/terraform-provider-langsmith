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
	_ resource.Resource                = &WebhookResource{}
	_ resource.ResourceWithImportState = &WebhookResource{}
)

func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

type WebhookResource struct {
	client *client.Client
}

type WebhookResourceModel struct {
	ID              types.String `tfsdk:"id"`
	URL             types.String `tfsdk:"url"`
	Headers         types.Map    `tfsdk:"headers"`
	Triggers        types.List   `tfsdk:"triggers"`
	IncludePrompts  types.List   `tfsdk:"include_prompts"`
	ExcludePrompts  types.List   `tfsdk:"exclude_prompts"`
	TenantID        types.String `tfsdk:"tenant_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

type webhookCreateRequest struct {
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	Triggers       []string          `json:"triggers,omitempty"`
	IncludePrompts []string          `json:"include_prompts,omitempty"`
	ExcludePrompts []string          `json:"exclude_prompts,omitempty"`
}

type webhookAPIResponse struct {
	ID             string            `json:"id"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers"`
	Triggers       []string          `json:"triggers"`
	IncludePrompts []string          `json:"include_prompts"`
	ExcludePrompts []string          `json:"exclude_prompts"`
	TenantID       string            `json:"tenant_id"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

func (r *WebhookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a prompt webhook in LangSmith.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the webhook.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The webhook URL.",
				Required:            true,
			},
			"headers": schema.MapAttribute{
				MarkdownDescription: "Custom headers to include in webhook requests.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"triggers": schema.ListAttribute{
				MarkdownDescription: "Trigger events for the webhook.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"include_prompts": schema.ListAttribute{
				MarkdownDescription: "Prompt names to include.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"exclude_prompts": schema.ListAttribute{
				MarkdownDescription: "Prompt names to exclude.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the webhook was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "When the webhook was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *WebhookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WebhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := webhookCreateRequest{
		URL: data.URL.ValueString(),
	}
	if !data.Headers.IsNull() {
		headers := make(map[string]string)
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Headers = headers
	}
	if !data.Triggers.IsNull() {
		var triggers []string
		resp.Diagnostics.Append(data.Triggers.ElementsAs(ctx, &triggers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Triggers = triggers
	}
	if !data.IncludePrompts.IsNull() {
		var prompts []string
		resp.Diagnostics.Append(data.IncludePrompts.ElementsAs(ctx, &prompts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.IncludePrompts = prompts
	}
	if !data.ExcludePrompts.IsNull() {
		var prompts []string
		resp.Diagnostics.Append(data.ExcludePrompts.ElementsAs(ctx, &prompts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.ExcludePrompts = prompts
	}

	var result webhookAPIResponse
	err := r.client.Post(ctx, "/api/v1/prompt-webhooks", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating webhook", err.Error())
		return
	}

	r.mapResponseToModel(ctx, &result, &data, &resp.Diagnostics)

	tflog.Trace(ctx, "created webhook resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result webhookAPIResponse
	err := r.client.Get(ctx, fmt.Sprintf("/api/v1/prompt-webhooks/%s", data.ID.ValueString()), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading webhook", err.Error())
		return
	}

	r.mapResponseToModel(ctx, &result, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WebhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := webhookCreateRequest{
		URL: data.URL.ValueString(),
	}
	if !data.Headers.IsNull() {
		headers := make(map[string]string)
		resp.Diagnostics.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Headers = headers
	}
	if !data.Triggers.IsNull() {
		var triggers []string
		resp.Diagnostics.Append(data.Triggers.ElementsAs(ctx, &triggers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Triggers = triggers
	}
	if !data.IncludePrompts.IsNull() {
		var prompts []string
		resp.Diagnostics.Append(data.IncludePrompts.ElementsAs(ctx, &prompts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.IncludePrompts = prompts
	}
	if !data.ExcludePrompts.IsNull() {
		var prompts []string
		resp.Diagnostics.Append(data.ExcludePrompts.ElementsAs(ctx, &prompts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.ExcludePrompts = prompts
	}

	var result webhookAPIResponse
	err := r.client.Patch(ctx, fmt.Sprintf("/api/v1/prompt-webhooks/%s", data.ID.ValueString()), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating webhook", err.Error())
		return
	}

	r.mapResponseToModel(ctx, &result, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, fmt.Sprintf("/api/v1/prompt-webhooks/%s", data.ID.ValueString()))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting webhook", err.Error())
	}
}

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *WebhookResource) mapResponseToModel(ctx context.Context, result *webhookAPIResponse, data *WebhookResourceModel, diagnostics *diag.Diagnostics) {
	data.ID = types.StringValue(result.ID)
	data.URL = types.StringValue(result.URL)
	data.TenantID = types.StringValue(result.TenantID)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	if len(result.Headers) > 0 {
		headers, diags := types.MapValueFrom(ctx, types.StringType, result.Headers)
		diagnostics.Append(diags...)
		data.Headers = headers
	}
	if len(result.Triggers) > 0 {
		triggers, diags := types.ListValueFrom(ctx, types.StringType, result.Triggers)
		diagnostics.Append(diags...)
		data.Triggers = triggers
	}
	if len(result.IncludePrompts) > 0 {
		prompts, diags := types.ListValueFrom(ctx, types.StringType, result.IncludePrompts)
		diagnostics.Append(diags...)
		data.IncludePrompts = prompts
	}
	if len(result.ExcludePrompts) > 0 {
		prompts, diags := types.ListValueFrom(ctx, types.StringType, result.ExcludePrompts)
		diagnostics.Append(diags...)
		data.ExcludePrompts = prompts
	}
}

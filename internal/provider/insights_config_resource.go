// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
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
	_ resource.Resource                = &InsightsConfigResource{}
	_ resource.ResourceWithImportState = &InsightsConfigResource{}
)

func NewInsightsConfigResource() resource.Resource {
	return &InsightsConfigResource{}
}

type InsightsConfigResource struct {
	client *client.Client
}

type InsightsConfigResourceModel struct {
	ID           types.String `tfsdk:"id"`
	SessionID    types.String `tfsdk:"session_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Config       types.String `tfsdk:"config"`
	ScheduleCron types.String `tfsdk:"schedule_cron"`
}

type insightsConfigCreateRequest struct {
	Name         string                 `json:"name"`
	Description  *string                `json:"description,omitempty"`
	Config       map[string]interface{} `json:"config"`
	ScheduleCron *string                `json:"schedule_cron,omitempty"`
}

type insightsConfigUpdateRequest struct {
	Name         *string                `json:"name,omitempty"`
	Description  *string                `json:"description,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	ScheduleCron *string                `json:"schedule_cron,omitempty"`
}

type insightsConfigAPI struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  *string                `json:"description"`
	Config       map[string]interface{} `json:"config"`
	ScheduleCron *string                `json:"schedule_cron"`
}

func (r *InsightsConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insights_config"
}

func (r *InsightsConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "**Beta.** Manages a LangSmith run-insights (clustering) job config attached to a tracing project. The `config` is a complex object with many optional fields (model, hierarchy, partitions, filter, summary_prompt, etc.) so it is exposed as a JSON-encoded string — see the LangSmith API reference for `CreateRunClusteringJobRequest`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"session_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the tracing project (session) the config attaches to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Job config name (max 255 chars)."},
			"description": schema.StringAttribute{Optional: true, MarkdownDescription: "Free-form description."},
			"config": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "JSON-encoded clustering job configuration (`CreateRunClusteringJobRequest` body).",
			},
			"schedule_cron": schema.StringAttribute{Optional: true, MarkdownDescription: "Cron expression to schedule the job."},
		},
	}
}

func (r *InsightsConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *InsightsConfigResource) basePath(sessionID string) string {
	return "/api/v1/sessions/" + sessionID + "/insights/configs"
}

func (r *InsightsConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data InsightsConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data.Config.ValueString()), &cfg); err != nil {
		resp.Diagnostics.AddError("Invalid config JSON", err.Error())
		return
	}
	body := insightsConfigCreateRequest{Name: data.Name.ValueString(), Config: cfg}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.ScheduleCron.IsNull() && !data.ScheduleCron.IsUnknown() {
		v := data.ScheduleCron.ValueString()
		body.ScheduleCron = &v
	}
	planConfig := data.Config
	var api insightsConfigAPI
	if err := r.client.Post(ctx, r.basePath(data.SessionID.ValueString()), body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating insights config", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	// Server expands `config` with all optional fields (nulls). Preserve the
	// plan's normalized JSON so Terraform doesn't flag drift on values the
	// user did not set.
	data.Config = planConfig
	tflog.Trace(ctx, "created insights config", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type getInsightsConfigsResponse struct {
	Configs []insightsConfigAPI `json:"configs"`
}

func (r *InsightsConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data InsightsConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list getInsightsConfigsResponse
	if err := r.client.Get(ctx, r.basePath(data.SessionID.ValueString()), nil, &list); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading insights configs", err.Error())
		return
	}
	for i := range list.Configs {
		if list.Configs[i].ID == data.ID.ValueString() {
			savedConfig := data.Config
			r.mapResponse(&list.Configs[i], &data)
			// Preserve the previously-stored config value to avoid phantom
			// drift from server-injected null fields.
			data.Config = savedConfig
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *InsightsConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data InsightsConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := insightsConfigUpdateRequest{}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.Config.IsNull() && !data.Config.IsUnknown() && data.Config.ValueString() != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(data.Config.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddError("Invalid config JSON", err.Error())
			return
		}
		body.Config = cfg
	}
	if !data.ScheduleCron.IsNull() && !data.ScheduleCron.IsUnknown() {
		v := data.ScheduleCron.ValueString()
		body.ScheduleCron = &v
	}
	planConfig := data.Config
	var api insightsConfigAPI
	if err := r.client.Patch(ctx, r.basePath(data.SessionID.ValueString())+"/"+data.ID.ValueString(), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating insights config", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	data.Config = planConfig
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InsightsConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data InsightsConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.basePath(data.SessionID.ValueString())+"/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting insights config", err.Error())
		return
	}
}

func (r *InsightsConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<session_id>:<config_id>\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("session_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *InsightsConfigResource) mapResponse(api *insightsConfigAPI, data *InsightsConfigResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	if api.Description != nil {
		data.Description = types.StringValue(*api.Description)
	} else {
		data.Description = types.StringNull()
	}
	if len(api.Config) > 0 {
		b, _ := json.Marshal(api.Config)
		data.Config = jsonStringValue(b)
	}
	if api.ScheduleCron != nil {
		data.ScheduleCron = types.StringValue(*api.ScheduleCron)
	} else {
		data.ScheduleCron = types.StringNull()
	}
}

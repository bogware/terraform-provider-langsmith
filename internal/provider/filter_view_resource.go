// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                = &FilterViewResource{}
	_ resource.ResourceWithImportState = &FilterViewResource{}
)

func NewFilterViewResource() resource.Resource {
	return &FilterViewResource{}
}

type FilterViewResource struct {
	client *client.Client
}

type FilterViewResourceModel struct {
	ID                types.String `tfsdk:"id"`
	SessionID         types.String `tfsdk:"session_id"`
	DisplayName       types.String `tfsdk:"display_name"`
	Description       types.String `tfsdk:"description"`
	FilterString      types.String `tfsdk:"filter_string"`
	TraceFilterString types.String `tfsdk:"trace_filter_string"`
	TreeFilterString  types.String `tfsdk:"tree_filter_string"`
	Type              types.String `tfsdk:"type"`
	StartTime         types.String `tfsdk:"start_time"`
	EndTime           types.String `tfsdk:"end_time"`
	Duration          types.String `tfsdk:"duration"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

type filterViewCreateRequest struct {
	DisplayName       string  `json:"display_name"`
	Description       *string `json:"description,omitempty"`
	FilterString      *string `json:"filter_string,omitempty"`
	TraceFilterString *string `json:"trace_filter_string,omitempty"`
	TreeFilterString  *string `json:"tree_filter_string,omitempty"`
	Type              *string `json:"type,omitempty"`
	StartTime         *string `json:"start_time,omitempty"`
	EndTime           *string `json:"end_time,omitempty"`
	Duration          *string `json:"duration,omitempty"`
}

type filterViewUpdateRequest struct {
	DisplayName       *string `json:"display_name,omitempty"`
	Description       *string `json:"description,omitempty"`
	FilterString      *string `json:"filter_string,omitempty"`
	TraceFilterString *string `json:"trace_filter_string,omitempty"`
	TreeFilterString  *string `json:"tree_filter_string,omitempty"`
	Type              *string `json:"type,omitempty"`
	StartTime         *string `json:"start_time,omitempty"`
	EndTime           *string `json:"end_time,omitempty"`
	Duration          *string `json:"duration,omitempty"`
}

type filterViewAPIResponse struct {
	ID                string  `json:"id"`
	SessionID         *string `json:"session_id"`
	DisplayName       string  `json:"display_name"`
	Description       *string `json:"description"`
	FilterString      *string `json:"filter_string"`
	TraceFilterString *string `json:"trace_filter_string"`
	TreeFilterString  *string `json:"tree_filter_string"`
	Type              string  `json:"type"`
	StartTime         *string `json:"start_time"`
	EndTime           *string `json:"end_time"`
	Duration          *string `json:"duration"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

func (r *FilterViewResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filter_view"
}

func (r *FilterViewResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith filter view (saved filter) within a project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the filter view.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project/session ID this filter view belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the filter view.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the filter view.",
				Optional:            true,
			},
			"filter_string": schema.StringAttribute{
				MarkdownDescription: "The run filter expression.",
				Optional:            true,
			},
			"trace_filter_string": schema.StringAttribute{
				MarkdownDescription: "The trace filter expression.",
				Optional:            true,
			},
			"tree_filter_string": schema.StringAttribute{
				MarkdownDescription: "The tree filter expression.",
				Optional:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of filter view. Valid values: `runs`, `threads`.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("runs", "threads")},
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The start time filter (ISO 8601).",
				Optional:            true,
			},
			"end_time": schema.StringAttribute{
				MarkdownDescription: "The end time filter (ISO 8601).",
				Optional:            true,
			},
			"duration": schema.StringAttribute{
				MarkdownDescription: "The duration filter.",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (r *FilterViewResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FilterViewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FilterViewResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := filterViewCreateRequest{
		DisplayName: data.DisplayName.ValueString(),
	}
	setOptionalString(&body.Description, data.Description)
	setOptionalString(&body.FilterString, data.FilterString)
	setOptionalString(&body.TraceFilterString, data.TraceFilterString)
	setOptionalString(&body.TreeFilterString, data.TreeFilterString)
	setOptionalString(&body.Type, data.Type)
	setOptionalString(&body.StartTime, data.StartTime)
	setOptionalString(&body.EndTime, data.EndTime)
	setOptionalString(&body.Duration, data.Duration)

	var result filterViewAPIResponse
	apiPath := fmt.Sprintf("/api/v1/sessions/%s/views", data.SessionID.ValueString())
	err := r.client.Post(ctx, apiPath, body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating filter view", err.Error())
		return
	}

	mapFilterViewResponseToState(&data, &result)
	tflog.Trace(ctx, "created filter view resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FilterViewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FilterViewResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result filterViewAPIResponse
	apiPath := fmt.Sprintf("/api/v1/sessions/%s/views/%s", data.SessionID.ValueString(), data.ID.ValueString())
	err := r.client.Get(ctx, apiPath, nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading filter view", err.Error())
		return
	}

	mapFilterViewResponseToState(&data, &result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FilterViewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FilterViewResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := filterViewUpdateRequest{}
	setOptionalString(&body.DisplayName, data.DisplayName)
	setOptionalString(&body.Description, data.Description)
	setOptionalString(&body.FilterString, data.FilterString)
	setOptionalString(&body.TraceFilterString, data.TraceFilterString)
	setOptionalString(&body.TreeFilterString, data.TreeFilterString)
	setOptionalString(&body.Type, data.Type)
	setOptionalString(&body.StartTime, data.StartTime)
	setOptionalString(&body.EndTime, data.EndTime)
	setOptionalString(&body.Duration, data.Duration)

	var result filterViewAPIResponse
	apiPath := fmt.Sprintf("/api/v1/sessions/%s/views/%s", data.SessionID.ValueString(), data.ID.ValueString())
	err := r.client.Patch(ctx, apiPath, body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating filter view", err.Error())
		return
	}

	mapFilterViewResponseToState(&data, &result)
	tflog.Trace(ctx, "updated filter view resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FilterViewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FilterViewResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/api/v1/sessions/%s/views/%s", data.SessionID.ValueString(), data.ID.ValueString())
	err := r.client.Delete(ctx, apiPath)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting filter view", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted filter view resource", map[string]interface{}{"id": data.ID.ValueString()})
}

// ImportState requires the import ID in the format "session_id/view_id" because
// the Read endpoint needs both the session ID and view ID to construct the API path.
func (r *FilterViewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected import ID in the format: session_id/view_id",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("session_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func mapFilterViewResponseToState(data *FilterViewResourceModel, result *filterViewAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.DisplayName = types.StringValue(result.DisplayName)
	data.Type = types.StringValue(result.Type)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	if result.SessionID != nil {
		data.SessionID = types.StringValue(*result.SessionID)
	}
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.FilterString, result.FilterString)
	setStateOptionalString(&data.TraceFilterString, result.TraceFilterString)
	setStateOptionalString(&data.TreeFilterString, result.TreeFilterString)
	setStateOptionalString(&data.StartTime, result.StartTime)
	setStateOptionalString(&data.EndTime, result.EndTime)
	setStateOptionalString(&data.Duration, result.Duration)
}

// setOptionalString sets a *string pointer from a Terraform types.String if it has a value.
func setOptionalString(dst **string, src types.String) {
	if !src.IsNull() && !src.IsUnknown() {
		v := src.ValueString()
		*dst = &v
	}
}

// setStateOptionalString maps an API *string to Terraform state.
func setStateOptionalString(dst *types.String, src *string) {
	if src != nil {
		*dst = types.StringValue(*src)
	} else {
		*dst = types.StringNull()
	}
}

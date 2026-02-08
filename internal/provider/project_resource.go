// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
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
	_ resource.Resource                = &ProjectResource{}
	_ resource.ResourceWithImportState = &ProjectResource{}
)

// NewProjectResource constructs a fresh ProjectResource, ready to wrangle
// LangSmith tracer sessions.
func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

// ProjectResource manages a LangSmith project (tracer session) — the corral
// where your traces are rounded up and accounted for.
type ProjectResource struct {
	client *client.Client
}

// ProjectResourceModel holds the Terraform state for a project. Every field
// maps to a brand on the hide — change one and Terraform will know.
type ProjectResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	DefaultDatasetID   types.String `tfsdk:"default_dataset_id"`
	ReferenceDatasetID types.String `tfsdk:"reference_dataset_id"`
	Extra              types.String `tfsdk:"extra"`
	TraceTier          types.String `tfsdk:"trace_tier"`
	TenantID           types.String `tfsdk:"tenant_id"`
	StartTime          types.String `tfsdk:"start_time"`
}

// projectAPIRequest is the wire format for creating or updating a project via
// the LangSmith API.
type projectAPIRequest struct {
	Name               string          `json:"name"`
	Description        *string         `json:"description,omitempty"`
	DefaultDatasetID   *string         `json:"default_dataset_id,omitempty"`
	ReferenceDatasetID *string         `json:"reference_dataset_id,omitempty"`
	Extra              json.RawMessage `json:"extra,omitempty"`
	TraceTier          *string         `json:"trace_tier,omitempty"`
}

// projectAPIResponse is what the LangSmith API sends back when a project is
// read or created — the full deed of ownership.
type projectAPIResponse struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	DefaultDatasetID   *string         `json:"default_dataset_id"`
	ReferenceDatasetID *string         `json:"reference_dataset_id"`
	Extra              json.RawMessage `json:"extra"`
	TraceTier          *string         `json:"trace_tier"`
	TenantID           string          `json:"tenant_id"`
	StartTime          string          `json:"start_time"`
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith project (tracer session).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the project.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the project.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the project.",
				Optional:            true,
			},
			"default_dataset_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the default dataset for this project.",
				Optional:            true,
			},
			"reference_dataset_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the reference dataset for this project.",
				Optional:            true,
			},
			"extra": schema.StringAttribute{
				MarkdownDescription: "JSON string containing extra metadata for the project.",
				Optional:            true,
			},
			"trace_tier": schema.StringAttribute{
				MarkdownDescription: "The trace retention tier for the project. Valid values: `longlived`, `shortlived`.",
				Optional:            true,
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID of the project.",
				Computed:            true,
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The start time of the project.",
				Computed:            true,
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := projectAPIRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.DefaultDatasetID.IsNull() && !data.DefaultDatasetID.IsUnknown() {
		v := data.DefaultDatasetID.ValueString()
		body.DefaultDatasetID = &v
	}
	if !data.ReferenceDatasetID.IsNull() && !data.ReferenceDatasetID.IsUnknown() {
		v := data.ReferenceDatasetID.ValueString()
		body.ReferenceDatasetID = &v
	}
	if !data.Extra.IsNull() && !data.Extra.IsUnknown() {
		body.Extra = json.RawMessage(data.Extra.ValueString())
	}
	// A trace's tier determines how long it stays on the prairie before fading away.
	if !data.TraceTier.IsNull() && !data.TraceTier.IsUnknown() {
		v := data.TraceTier.ValueString()
		body.TraceTier = &v
	}

	var result projectAPIResponse
	err := r.client.Post(ctx, "/api/v1/sessions", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	mapProjectResponseToState(&data, &result)
	tflog.Trace(ctx, "created project resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result projectAPIResponse
	err := r.client.Get(ctx, "/api/v1/sessions/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	mapProjectResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := projectAPIRequest{
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.DefaultDatasetID.IsNull() && !data.DefaultDatasetID.IsUnknown() {
		v := data.DefaultDatasetID.ValueString()
		body.DefaultDatasetID = &v
	}
	if !data.ReferenceDatasetID.IsNull() && !data.ReferenceDatasetID.IsUnknown() {
		v := data.ReferenceDatasetID.ValueString()
		body.ReferenceDatasetID = &v
	}
	if !data.Extra.IsNull() && !data.Extra.IsUnknown() {
		body.Extra = json.RawMessage(data.Extra.ValueString())
	}
	// Even Marshal Dillon knows you can't outrun a retention policy.
	if !data.TraceTier.IsNull() && !data.TraceTier.IsUnknown() {
		v := data.TraceTier.ValueString()
		body.TraceTier = &v
	}

	var result projectAPIResponse
	err := r.client.Patch(ctx, "/api/v1/sessions/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	mapProjectResponseToState(&data, &result)
	tflog.Trace(ctx, "updated project resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/sessions/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted project resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapProjectResponseToState translates the API response into Terraform state,
// branding each field so Terraform can track it on the open range.
func mapProjectResponseToState(data *ProjectResourceModel, result *projectAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if result.Description != nil {
		data.Description = types.StringValue(*result.Description)
	} else {
		data.Description = types.StringNull()
	}

	if result.DefaultDatasetID != nil {
		data.DefaultDatasetID = types.StringValue(*result.DefaultDatasetID)
	} else {
		data.DefaultDatasetID = types.StringNull()
	}

	if result.ReferenceDatasetID != nil {
		data.ReferenceDatasetID = types.StringValue(*result.ReferenceDatasetID)
	} else {
		data.ReferenceDatasetID = types.StringNull()
	}

	if len(result.Extra) > 0 && string(result.Extra) != "null" {
		data.Extra = types.StringValue(string(result.Extra))
	} else {
		data.Extra = types.StringNull()
	}

	if result.TraceTier != nil {
		data.TraceTier = types.StringValue(*result.TraceTier)
	} else {
		data.TraceTier = types.StringNull()
	}

	data.TenantID = types.StringValue(result.TenantID)
	data.StartTime = types.StringValue(result.StartTime)
}

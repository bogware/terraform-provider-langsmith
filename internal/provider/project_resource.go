// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	TagValueIDs        types.List   `tfsdk:"tag_value_ids"`
	WorkspaceID        types.String `tfsdk:"workspace_id"`
	TenantID           types.String `tfsdk:"tenant_id"`
	StartTime          types.String `tfsdk:"start_time"`
	EndTime            types.String `tfsdk:"end_time"`
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
	TagValueIDs        []string        `json:"tag_value_ids,omitempty"`
	StartTime          *string         `json:"start_time,omitempty"`
	EndTime            *string         `json:"end_time,omitempty"`
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
	TagValueIDs        []string        `json:"tag_value_ids,omitempty"`
	WorkspaceID        string          `json:"workspace_id"`
	TenantID           string          `json:"tenant_id"`
	StartTime          string          `json:"start_time"`
	EndTime            *string         `json:"end_time"`
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
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf("longlived", "shortlived")},
			},
			"tag_value_ids": schema.ListAttribute{
				MarkdownDescription: "A list of tag value UUIDs (see `langsmith_tag_value`) to associate with the project. The LangSmith API does not echo these values back on read, so the configured value is preserved in state.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID of the resource. If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Deprecated: use `workspace_id` instead.",
				DeprecationMessage:  "Use workspace_id instead. This attribute will be removed in a future release.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"start_time": schema.StringAttribute{
				MarkdownDescription: "The start time of the project, as an RFC 3339 / ISO 8601 timestamp. If unset, the server assigns the creation time.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"end_time": schema.StringAttribute{
				MarkdownDescription: "The end time of the project, as an RFC 3339 / ISO 8601 timestamp.",
				Optional:            true,
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
	if !data.TagValueIDs.IsNull() && !data.TagValueIDs.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.TagValueIDs.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.TagValueIDs = ids
	}
	if !data.StartTime.IsNull() && !data.StartTime.IsUnknown() {
		v := data.StartTime.ValueString()
		body.StartTime = &v
	}
	if !data.EndTime.IsNull() && !data.EndTime.IsUnknown() {
		v := data.EndTime.ValueString()
		body.EndTime = &v
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var result projectAPIResponse
	err := c.Post(ctx, "/api/v1/sessions", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	mapProjectResponseToState(ctx, &data, &result, c, &resp.Diagnostics)
	tflog.Trace(ctx, "created project resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var result projectAPIResponse
	err := c.Get(ctx, "/api/v1/sessions/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	mapProjectResponseToState(ctx, &data, &result, c, &resp.Diagnostics)

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
	if !data.TagValueIDs.IsNull() && !data.TagValueIDs.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.TagValueIDs.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.TagValueIDs = ids
	}
	if !data.StartTime.IsNull() && !data.StartTime.IsUnknown() {
		v := data.StartTime.ValueString()
		body.StartTime = &v
	}
	if !data.EndTime.IsNull() && !data.EndTime.IsUnknown() {
		v := data.EndTime.ValueString()
		body.EndTime = &v
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var result projectAPIResponse
	err := c.Patch(ctx, "/api/v1/sessions/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	mapProjectResponseToState(ctx, &data, &result, c, &resp.Diagnostics)
	tflog.Trace(ctx, "updated project resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/sessions/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
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
func mapProjectResponseToState(ctx context.Context, data *ProjectResourceModel, result *projectAPIResponse, c *client.Client, diags *diag.Diagnostics) {
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

	data.Extra = jsonStringValue(result.Extra)

	if result.TraceTier != nil {
		data.TraceTier = types.StringValue(*result.TraceTier)
	} else {
		data.TraceTier = types.StringNull()
	}

	// The LangSmith API accepts tag_value_ids on create/update but does not
	// echo them back on read, so preserve the configured value already in
	// state and only overwrite it when the API actually returns the list.
	if len(result.TagValueIDs) > 0 {
		ids, d := types.ListValueFrom(ctx, types.StringType, result.TagValueIDs)
		diags.Append(d...)
		data.TagValueIDs = ids
	} else if data.TagValueIDs.IsUnknown() {
		data.TagValueIDs = types.ListNull(types.StringType)
	}

	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(result.WorkspaceID, result.TenantID), diags)
	data.TenantID = data.WorkspaceID
	data.StartTime = types.StringValue(result.StartTime)

	if result.EndTime != nil {
		data.EndTime = types.StringValue(*result.EndTime)
	} else {
		data.EndTime = types.StringNull()
	}
}

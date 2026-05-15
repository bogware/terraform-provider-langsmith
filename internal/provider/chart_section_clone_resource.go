// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &ChartSectionCloneResource{}
	_ resource.ResourceWithImportState = &ChartSectionCloneResource{}
)

func NewChartSectionCloneResource() resource.Resource {
	return &ChartSectionCloneResource{}
}

type ChartSectionCloneResource struct {
	client *client.Client
}

type ChartSectionCloneResourceModel struct {
	ID              types.String `tfsdk:"id"`
	SourceSectionID types.String `tfsdk:"source_section_id"`
	SessionID       types.String `tfsdk:"session_id"`
	Title           types.String `tfsdk:"title"`
	Description     types.String `tfsdk:"description"`
	Index           types.Int64  `tfsdk:"index"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

type chartSectionCloneRequest struct {
	SectionID *string `json:"section_id,omitempty"`
	SessionID *string `json:"session_id,omitempty"`
}

func (r *ChartSectionCloneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chart_section_clone"
}

func (r *ChartSectionCloneResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Clones a LangSmith custom chart section via `POST /api/v1/charts/section/clone`. After creation, the resulting section behaves identically to one managed by `langsmith_chart_section` (title/description/index are mutable). Changing `source_section_id` or `session_id` forces replacement (re-clone).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the cloned chart section.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_section_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the source section to clone. Changing this forces a new clone.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "Optional session/project ID to associate with the cloned section. Changing this forces a new clone.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "The title of the cloned section. If omitted, the API's clone-generated title is used. If specified and different from the clone result, the section is patched after cloning.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the section. If omitted, the cloned section inherits the source's description. Applied as a follow-up PATCH after cloning if set to a different value.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"index": schema.Int64Attribute{
				MarkdownDescription: "The display order index. Applied as a follow-up PATCH after cloning if set.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
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

func (r *ChartSectionCloneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ChartSectionCloneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ChartSectionCloneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloneBody := chartSectionCloneRequest{}
	setOptionalString(&cloneBody.SectionID, data.SourceSectionID)
	setOptionalString(&cloneBody.SessionID, data.SessionID)

	var cloned chartSectionAPIResponse
	if err := r.client.Post(ctx, "/api/v1/charts/section/clone", cloneBody, &cloned); err != nil {
		resp.Diagnostics.AddError("Error cloning chart section", err.Error())
		return
	}

	// If user supplied title/description/index that differ from the clone's
	// initial values, follow up with a PATCH so state matches the plan.
	needsPatch := false
	patchBody := chartSectionUpdateRequest{}
	if !data.Title.IsNull() && !data.Title.IsUnknown() && data.Title.ValueString() != cloned.Title {
		v := data.Title.ValueString()
		patchBody.Title = &v
		needsPatch = true
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		clonedDesc := ""
		if cloned.Description != nil {
			clonedDesc = *cloned.Description
		}
		if data.Description.ValueString() != clonedDesc {
			v := data.Description.ValueString()
			patchBody.Description = &v
			needsPatch = true
		}
	}
	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		clonedIdx := int64(-1)
		if cloned.Index != nil {
			clonedIdx = *cloned.Index
		}
		if data.Index.ValueInt64() != clonedIdx {
			v := data.Index.ValueInt64()
			patchBody.Index = &v
			needsPatch = true
		}
	}

	final := cloned
	if needsPatch {
		var patched chartSectionAPIResponse
		if err := r.client.Patch(ctx, "/api/v1/charts/section/"+cloned.ID, patchBody, &patched); err != nil {
			resp.Diagnostics.AddError("Error applying post-clone updates", err.Error())
			return
		}
		final = patched
	}

	// Map response to model. Preserve source_section_id and session_id from the
	// plan since they are clone-time inputs and not returned by the API.
	sourceSectionID := data.SourceSectionID
	sessionID := data.SessionID
	data.ID = types.StringValue(final.ID)
	data.Title = types.StringValue(final.Title)
	setStateOptionalString(&data.Description, final.Description)
	setStateOptionalString(&data.CreatedAt, final.CreatedAt)
	setStateOptionalString(&data.UpdatedAt, final.ModifiedAt)
	if final.Index != nil {
		data.Index = types.Int64Value(*final.Index)
	} else {
		data.Index = types.Int64Null()
	}
	data.SourceSectionID = sourceSectionID
	data.SessionID = sessionID

	tflog.Trace(ctx, "cloned chart section resource", map[string]interface{}{"id": final.ID, "source_section_id": data.SourceSectionID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartSectionCloneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ChartSectionCloneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	savedSourceSectionID := data.SourceSectionID
	savedSessionID := data.SessionID
	savedCreatedAt := data.CreatedAt
	savedUpdatedAt := data.UpdatedAt

	body := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	var result chartSectionAPIResponse
	err := r.client.Post(ctx, "/api/v1/charts/section/"+data.ID.ValueString(), body, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading cloned chart section", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Title = types.StringValue(result.Title)
	setStateOptionalString(&data.Description, result.Description)
	if result.Index != nil {
		data.Index = types.Int64Value(*result.Index)
	} else {
		data.Index = types.Int64Null()
	}
	data.SourceSectionID = savedSourceSectionID
	data.SessionID = savedSessionID
	data.CreatedAt = savedCreatedAt
	data.UpdatedAt = savedUpdatedAt

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartSectionCloneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ChartSectionCloneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	savedCreatedAt := data.CreatedAt
	savedSourceSectionID := data.SourceSectionID
	savedSessionID := data.SessionID

	body := chartSectionUpdateRequest{}
	setOptionalString(&body.Title, data.Title)
	setOptionalString(&body.Description, data.Description)
	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		v := data.Index.ValueInt64()
		body.Index = &v
	}

	var result chartSectionAPIResponse
	err := r.client.Patch(ctx, "/api/v1/charts/section/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating cloned chart section", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Title = types.StringValue(result.Title)
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.UpdatedAt, result.ModifiedAt)
	if result.Index != nil {
		data.Index = types.Int64Value(*result.Index)
	} else {
		data.Index = types.Int64Null()
	}
	data.CreatedAt = savedCreatedAt
	data.SourceSectionID = savedSourceSectionID
	data.SessionID = savedSessionID

	tflog.Trace(ctx, "updated cloned chart section resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartSectionCloneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ChartSectionCloneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/charts/section/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting cloned chart section", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted cloned chart section resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ChartSectionCloneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

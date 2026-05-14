// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &ChartResource{}
	_ resource.ResourceWithImportState = &ChartResource{}
)

func NewChartResource() resource.Resource {
	return &ChartResource{}
}

type ChartResource struct {
	client *client.Client
}

type ChartResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Title         types.String `tfsdk:"title"`
	Description   types.String `tfsdk:"description"`
	Index         types.Int64  `tfsdk:"index"`
	ChartType     types.String `tfsdk:"chart_type"`
	Series        types.String `tfsdk:"series"`
	SectionID     types.String `tfsdk:"section_id"`
	Metadata      types.String `tfsdk:"metadata"`
	CommonFilters types.String `tfsdk:"common_filters"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

type chartCreateRequest struct {
	Title         string           `json:"title"`
	Description   *string          `json:"description,omitempty"`
	Index         *int64           `json:"index,omitempty"`
	ChartType     string           `json:"chart_type"`
	Series        json.RawMessage  `json:"series"`
	SectionID     *string          `json:"section_id,omitempty"`
	Metadata      *json.RawMessage `json:"metadata,omitempty"`
	CommonFilters *json.RawMessage `json:"common_filters,omitempty"`
}

type chartUpdateRequest struct {
	Title         *string          `json:"title,omitempty"`
	Description   *string          `json:"description,omitempty"`
	Index         *int64           `json:"index,omitempty"`
	ChartType     *string          `json:"chart_type,omitempty"`
	Series        json.RawMessage  `json:"series,omitempty"`
	SectionID     *string          `json:"section_id,omitempty"`
	Metadata      *json.RawMessage `json:"metadata,omitempty"`
	CommonFilters *json.RawMessage `json:"common_filters,omitempty"`
}

type chartAPIResponse struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	Description   *string         `json:"description"`
	Index         *int64          `json:"index"`
	ChartType     string          `json:"chart_type"`
	Series        json.RawMessage `json:"series"`
	SectionID     *string         `json:"section_id"`
	Metadata      json.RawMessage `json:"metadata"`
	CommonFilters json.RawMessage `json:"common_filters"`
}

func (r *ChartResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chart"
}

func (r *ChartResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith workspace-scoped custom chart (`/api/v1/charts/*`). " +
			"Bulk chart read (`POST /api/v1/charts`) and preview (`POST /api/v1/charts/preview`) are not represented as Terraform resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the chart.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "The title of the chart.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the chart.",
				Optional:            true,
			},
			"index": schema.Int64Attribute{
				MarkdownDescription: "The display order index (0-100).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"chart_type": schema.StringAttribute{
				MarkdownDescription: "The chart type. Valid values: `line`, `bar`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("line", "bar")},
			},
			"series": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of chart series configurations.",
				Required:            true,
			},
			"section_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the chart section this chart belongs to.",
				Optional:            true,
			},
			"metadata": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded metadata object.",
				Optional:            true,
			},
			"common_filters": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded common filter configuration.",
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

func (r *ChartResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ChartResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := chartCreateRequest{
		Title:     data.Title.ValueString(),
		ChartType: data.ChartType.ValueString(),
		Series:    json.RawMessage(data.Series.ValueString()),
	}
	setOptionalString(&body.Description, data.Description)
	setOptionalString(&body.SectionID, data.SectionID)
	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		v := data.Index.ValueInt64()
		body.Index = &v
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		raw := json.RawMessage(data.Metadata.ValueString())
		body.Metadata = &raw
	}
	if !data.CommonFilters.IsNull() && !data.CommonFilters.IsUnknown() {
		raw := json.RawMessage(data.CommonFilters.ValueString())
		body.CommonFilters = &raw
	}

	// Preserve the plan's series value; the API expands series with null fields
	// and auto-generated IDs which would cause "inconsistent result after apply".
	planSeries := data.Series

	var result chartAPIResponse
	err := r.client.Post(ctx, "/api/v1/charts/create", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating chart", err.Error())
		return
	}

	mapChartResponseToState(&data, &result)
	data.Series = planSeries
	// The chart API does not return timestamps; set to null to avoid unknown values.
	data.CreatedAt = types.StringNull()
	data.UpdatedAt = types.StringNull()
	tflog.Trace(ctx, "created chart resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save values from state that the read endpoint doesn't return.
	savedCreatedAt := data.CreatedAt
	savedUpdatedAt := data.UpdatedAt
	savedSeries := data.Series
	savedSectionID := data.SectionID

	// Chart read uses POST and requires start_time/end_time.
	// Use a minimal 1-minute window to avoid server-side aggregation overhead.
	body := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	var result chartAPIResponse
	err := r.client.Post(ctx, "/api/v1/charts/"+data.ID.ValueString(), body, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading chart", err.Error())
		return
	}

	mapChartResponseToState(&data, &result)
	// Restore values the read endpoint doesn't return.
	data.CreatedAt = savedCreatedAt
	data.UpdatedAt = savedUpdatedAt
	data.Series = savedSeries
	data.SectionID = savedSectionID
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior ChartResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	savedCreatedAt := prior.CreatedAt
	savedUpdatedAt := prior.UpdatedAt

	body := chartUpdateRequest{}
	setOptionalString(&body.Title, data.Title)
	setOptionalString(&body.Description, data.Description)
	setOptionalString(&body.ChartType, data.ChartType)
	setOptionalString(&body.SectionID, data.SectionID)
	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		v := data.Index.ValueInt64()
		body.Index = &v
	}
	if !data.Series.IsNull() && !data.Series.IsUnknown() {
		body.Series = json.RawMessage(data.Series.ValueString())
	}
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		raw := json.RawMessage(data.Metadata.ValueString())
		body.Metadata = &raw
	}
	if !data.CommonFilters.IsNull() && !data.CommonFilters.IsUnknown() {
		raw := json.RawMessage(data.CommonFilters.ValueString())
		body.CommonFilters = &raw
	}

	planSeries := data.Series

	var result chartAPIResponse
	err := r.client.Patch(ctx, "/api/v1/charts/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating chart", err.Error())
		return
	}

	mapChartResponseToState(&data, &result)
	data.Series = planSeries
	data.CreatedAt = savedCreatedAt
	data.UpdatedAt = savedUpdatedAt
	tflog.Trace(ctx, "updated chart resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/charts/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting chart", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted chart resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ChartResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapChartResponseToState(data *ChartResourceModel, result *chartAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Title = types.StringValue(result.Title)
	data.ChartType = types.StringValue(result.ChartType)
	data.Series = jsonStringValue(result.Series)
	data.Metadata = jsonStringValue(result.Metadata)
	data.CommonFilters = jsonStringValue(result.CommonFilters)
	setStateOptionalString(&data.Description, result.Description)
	setStateOptionalString(&data.SectionID, result.SectionID)
	if result.Index != nil {
		data.Index = types.Int64Value(*result.Index)
	} else {
		data.Index = types.Int64Null()
	}
}

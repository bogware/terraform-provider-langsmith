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
	WorkspaceID   types.String `tfsdk:"workspace_id"`
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
	// LangSmith APIs are inconsistent about which key carries the workspace
	// identifier, so decode both spellings.
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
}

func (r *ChartResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chart"
}

func (r *ChartResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith custom chart.",
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
				MarkdownDescription: "The chart type. Valid values: `line`, `bar`, `table`, `kpi`, `top-k`, `pie`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("line", "bar", "table", "kpi", "top-k", "pie")},
			},
			"series": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of chart series configurations. Each series object supports the " +
					"following keys: `name` (display label), `filters` (legacy filter object), `metric` (legacy metric name), " +
					"`metric_definition` (structured metric: count, scalar, percentile, or ratio), " +
					"`group_by_definitions` (array of group-by definitions), `filter_definition` (structured filter, " +
					"e.g. by tracing project), and `feedback_key` (feedback key to aggregate).",
				Required: true,
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
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
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

	c := effectiveClient(r.client, data.WorkspaceID)
	var result chartAPIResponse
	err := c.Post(ctx, "/api/v1/charts/create", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating chart", err.Error())
		return
	}

	mapChartResponseToState(&data, &result)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(result.WorkspaceID, result.TenantID), &resp.Diagnostics)
	data.Series = planSeries
	// The chart API does not return timestamps; set to null to avoid unknown values.
	data.CreatedAt = types.StringNull()
	data.UpdatedAt = types.StringNull()
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	tflog.Trace(ctx, "created chart resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The chart read endpoint is lossy, so hold on to the values we already have:
	//
	//   - created_at / updated_at and section_id are not part of the response at all.
	//   - series *is* returned, but in the server's expanded form: every optional key
	//     is materialized as null and each entry is given a generated "id". Writing
	//     that over a configuration-derived value would produce a permanent phantom
	//     diff, so a prior state value always wins.
	//
	// These are only used when they actually hold a value. After an import there is
	// no prior state, and falling back to the API response is what keeps the required
	// `series` attribute from landing in state as null (see ImportState).
	priorCreatedAt := data.CreatedAt
	priorUpdatedAt := data.UpdatedAt
	priorSeries := data.Series
	priorSectionID := data.SectionID

	// Chart read uses POST and requires start_time/end_time.
	// Use a minimal 1-minute window to avoid server-side aggregation overhead.
	body := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	c := effectiveClient(r.client, data.WorkspaceID)
	var result chartAPIResponse
	err := c.Post(ctx, "/api/v1/charts/"+data.ID.ValueString(), body, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading chart", err.Error())
		return
	}

	mapChartResponseToState(&data, &result)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(result.WorkspaceID, result.TenantID), &resp.Diagnostics)
	// Prefer what we already knew for the attributes the read endpoint cannot
	// faithfully reproduce, but only when prior state actually has a value —
	// otherwise (import) keep whatever the API returned.
	data.CreatedAt = preferChartPriorState(priorCreatedAt, data.CreatedAt)
	data.UpdatedAt = preferChartPriorState(priorUpdatedAt, data.UpdatedAt)
	data.Series = preferChartPriorState(priorSeries, data.Series)
	data.SectionID = preferChartPriorState(priorSectionID, data.SectionID)
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	c := effectiveClient(r.client, data.WorkspaceID)
	var result chartAPIResponse
	err := c.Patch(ctx, "/api/v1/charts/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating chart", err.Error())
		return
	}

	mapChartResponseToState(&data, &result)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(result.WorkspaceID, result.TenantID), &resp.Diagnostics)
	data.Series = planSeries
	tflog.Trace(ctx, "updated chart resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChartResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ChartResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/charts/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting chart", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted chart resource", map[string]interface{}{"id": data.ID.ValueString()})
}

// ImportState imports an existing chart by its ID.
//
// Two attributes cannot round-trip through an import, because the LangSmith read
// endpoint (POST /api/v1/charts/{id}) does not reproduce them:
//
//   - section_id is simply not part of the response, so an imported chart always
//     has section_id null in state even when it does belong to a section. (The
//     create/update responses do return it; only the read is missing it.)
//   - series is returned, but expanded: the server materializes every optional key
//     as null and assigns each entry a generated "id", so the imported value is
//     semantically the same configuration but not byte-identical to the JSON in a
//     `series = jsonencode(...)` block.
//
// Consequently the first plan after an import shows a diff for these attributes.
// Applying it re-sends the configured values (an idempotent update) and state
// converges from then on. The acceptance tests reflect this with
// ImportStateVerifyIgnore: {"series", "section_id"}. Everything else — title,
// description, index, chart_type, metadata, common_filters, workspace_id —
// round-trips.
func (r *ChartResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// preferChartPriorState returns the prior state value when it holds one, and the
// value mapped from the API response otherwise. It exists so Read can protect
// configuration-derived attributes from the server's lossy/expanded chart
// representation without blanking those attributes during an import, where there
// is no prior state to protect.
func preferChartPriorState(prior, fromAPI types.String) types.String {
	if prior.IsNull() || prior.IsUnknown() {
		return fromAPI
	}
	return prior
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

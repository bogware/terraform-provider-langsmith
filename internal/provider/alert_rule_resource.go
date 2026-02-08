// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	_ resource.Resource                = &AlertRuleResource{}
	_ resource.ResourceWithImportState = &AlertRuleResource{}
)

// NewAlertRuleResource returns a new AlertRuleResource -- Marshal Dillon posting
// a new deputy to keep watch over your LangSmith projects.
func NewAlertRuleResource() resource.Resource {
	return &AlertRuleResource{}
}

// AlertRuleResource manages alert rules for monitoring LangSmith projects.
// Like the lookout on Boot Hill, it keeps a steady eye on your metrics and
// raises the alarm when something crosses the line.
type AlertRuleResource struct {
	client *client.Client
}

// AlertRuleResourceModel holds the Terraform state for an alert rule,
// from its name and thresholds down to the actions it fires when trouble rides in.
type AlertRuleResourceModel struct {
	ID                     types.String  `tfsdk:"id"`
	SessionID              types.String  `tfsdk:"session_id"`
	Name                   types.String  `tfsdk:"name"`
	Description            types.String  `tfsdk:"description"`
	Type                   types.String  `tfsdk:"type"`
	Aggregation            types.String  `tfsdk:"aggregation"`
	Attribute              types.String  `tfsdk:"attribute"`
	Operator               types.String  `tfsdk:"operator"`
	WindowMinutes          types.Int64   `tfsdk:"window_minutes"`
	Threshold              types.Float64 `tfsdk:"threshold"`
	ThresholdMultiplier    types.Float64 `tfsdk:"threshold_multiplier"`
	ThresholdWindowMinutes types.Int64   `tfsdk:"threshold_window_minutes"`
	Filter                 types.String  `tfsdk:"filter"`
	DenominatorFilter      types.String  `tfsdk:"denominator_filter"`
	Actions                types.String  `tfsdk:"actions"`
	CreatedAt              types.String  `tfsdk:"created_at"`
	UpdatedAt              types.String  `tfsdk:"updated_at"`
}

// alertRuleRequest is the payload we send to the API when staking a new alert
// or updating the watch orders on an existing one.
type alertRuleRequest struct {
	Rule    alertRuleBody   `json:"rule"`
	Actions json.RawMessage `json:"actions"`
}

// alertRuleBody carries the specifics of the rule itself -- what to watch,
// how to measure it, and when to holler.
type alertRuleBody struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Type                   string   `json:"type"`
	Aggregation            string   `json:"aggregation"`
	Attribute              string   `json:"attribute"`
	Operator               string   `json:"operator"`
	WindowMinutes          int64    `json:"window_minutes"`
	Threshold              *float64 `json:"threshold,omitempty"`
	ThresholdMultiplier    *float64 `json:"threshold_multiplier,omitempty"`
	ThresholdWindowMinutes *int64   `json:"threshold_window_minutes,omitempty"`
	Filter                 *string  `json:"filter,omitempty"`
	DenominatorFilter      *string  `json:"denominator_filter,omitempty"`
}

// alertRuleResponse is what the API sends back -- the full account of a rule
// and its actions, straight from the telegraph office.
type alertRuleResponse struct {
	Rule    alertRuleResponseBody `json:"rule"`
	Actions json.RawMessage       `json:"actions"`
}

// alertRuleResponseBody is the API's detailed record of the rule itself,
// including its ID and timestamps -- the wanted poster, if you will.
type alertRuleResponseBody struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Type                   string   `json:"type"`
	Aggregation            string   `json:"aggregation"`
	Attribute              string   `json:"attribute"`
	Operator               string   `json:"operator"`
	WindowMinutes          int64    `json:"window_minutes"`
	Threshold              *float64 `json:"threshold"`
	ThresholdMultiplier    *float64 `json:"threshold_multiplier"`
	ThresholdWindowMinutes *int64   `json:"threshold_window_minutes"`
	Filter                 *string  `json:"filter"`
	DenominatorFilter      *string  `json:"denominator_filter"`
	SessionID              string   `json:"session_id"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
}

func (r *AlertRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_rule"
}

func (r *AlertRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith alert rule for monitoring project metrics.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the alert rule.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "The project/session ID to attach the alert to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the alert rule.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the alert rule.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The alert rule type (`threshold` or `change`).",
				Required:            true,
			},
			"aggregation": schema.StringAttribute{
				MarkdownDescription: "The aggregation method (`avg`, `sum`, or `pct`).",
				Required:            true,
			},
			"attribute": schema.StringAttribute{
				MarkdownDescription: "The metric attribute to monitor (`latency`, `error_count`, `feedback_score`, `run_latency`, or `run_count`).",
				Required:            true,
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "The comparison operator (`gte` or `lte`).",
				Required:            true,
			},
			"window_minutes": schema.Int64Attribute{
				MarkdownDescription: "The monitoring window in minutes.",
				Required:            true,
			},
			"threshold": schema.Float64Attribute{
				MarkdownDescription: "The threshold value for threshold-type rules.",
				Optional:            true,
			},
			"threshold_multiplier": schema.Float64Attribute{
				MarkdownDescription: "The multiplier for change-type rules.",
				Optional:            true,
			},
			"threshold_window_minutes": schema.Int64Attribute{
				MarkdownDescription: "The comparison window in minutes for change-type rules.",
				Optional:            true,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "A run filter expression.",
				Optional:            true,
			},
			"denominator_filter": schema.StringAttribute{
				MarkdownDescription: "A denominator filter for `pct` aggregation.",
				Optional:            true,
			},
			"actions": schema.StringAttribute{
				MarkdownDescription: "A JSON-encoded array of action objects, e.g. `[{\"target\": \"email\", \"config\": {...}}]`.",
				Required:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the alert rule was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the alert rule was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *AlertRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AlertRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AlertRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := buildAlertRuleRequest(&data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/platform/alerts/%s", data.SessionID.ValueString())

	var result alertRuleResponse
	err := r.client.Post(ctx, apiPath, body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert rule", err.Error())
		return
	}

	// The create response returns session_id as null, so we preserve the
	// value from the plan. The GET endpoint returns it properly.
	sessionID := data.SessionID.ValueString()
	mapAlertRuleResponseToState(&data, &result)
	data.SessionID = types.StringValue(sessionID)
	tflog.Trace(ctx, "created alert rule resource", map[string]interface{}{"id": result.Rule.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AlertRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/platform/alerts/%s/%s",
		data.SessionID.ValueString(), data.ID.ValueString())

	var result alertRuleResponse
	err := r.client.Get(ctx, apiPath, nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert rule", err.Error())
		return
	}

	mapAlertRuleResponseToState(&data, &result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AlertRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := buildAlertRuleRequest(&data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/platform/alerts/%s/%s",
		data.SessionID.ValueString(), data.ID.ValueString())

	var result alertRuleResponse
	err := r.client.Patch(ctx, apiPath, body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert rule", err.Error())
		return
	}

	mapAlertRuleResponseToState(&data, &result)
	tflog.Trace(ctx, "updated alert rule resource", map[string]interface{}{"id": result.Rule.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AlertRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AlertRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/platform/alerts/%s/%s",
		data.SessionID.ValueString(), data.ID.ValueString())

	err := r.client.Delete(ctx, apiPath)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting alert rule", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted alert rule resource", map[string]interface{}{"id": data.ID.ValueString()})
}

// ImportState handles importing an alert rule resource.
// The import ID format is "session_id/alert_rule_id" -- two halves of the trail
// that lead us right to the outlaw we are looking for.
func (r *AlertRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'session_id/alert_rule_id', got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("session_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// buildAlertRuleRequest assembles the request body from the Terraform plan data,
// loading each optional field only if it has ridden into town with a real value.
// Think of it as packing the saddlebags before heading out on patrol.
func buildAlertRuleRequest(data *AlertRuleResourceModel) (*alertRuleRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	body := &alertRuleRequest{
		Rule: alertRuleBody{
			Name:          data.Name.ValueString(),
			Description:   data.Description.ValueString(),
			Type:          data.Type.ValueString(),
			Aggregation:   data.Aggregation.ValueString(),
			Attribute:     data.Attribute.ValueString(),
			Operator:      data.Operator.ValueString(),
			WindowMinutes: data.WindowMinutes.ValueInt64(),
		},
	}

	if !data.Threshold.IsNull() && !data.Threshold.IsUnknown() {
		v := data.Threshold.ValueFloat64()
		body.Rule.Threshold = &v
	}
	if !data.ThresholdMultiplier.IsNull() && !data.ThresholdMultiplier.IsUnknown() {
		v := data.ThresholdMultiplier.ValueFloat64()
		body.Rule.ThresholdMultiplier = &v
	}
	if !data.ThresholdWindowMinutes.IsNull() && !data.ThresholdWindowMinutes.IsUnknown() {
		v := data.ThresholdWindowMinutes.ValueInt64()
		body.Rule.ThresholdWindowMinutes = &v
	}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		v := data.Filter.ValueString()
		body.Rule.Filter = &v
	}
	if !data.DenominatorFilter.IsNull() && !data.DenominatorFilter.IsUnknown() {
		v := data.DenominatorFilter.ValueString()
		body.Rule.DenominatorFilter = &v
	}

	actionsJSON := data.Actions.ValueString()
	if !json.Valid([]byte(actionsJSON)) {
		diags.AddError(
			"Invalid Actions JSON",
			"The actions field must contain valid JSON. Even Festus could tell this ain't right.",
		)
		return nil, diags
	}
	body.Actions = json.RawMessage(actionsJSON)

	return body, diags
}

// mapAlertRuleResponseToState rounds up the API response values and brands them
// into the Terraform state model. Optional fields that came back empty get set to
// null -- no sense reporting ghost cattle to the marshal.
func mapAlertRuleResponseToState(data *AlertRuleResourceModel, result *alertRuleResponse) {
	data.ID = types.StringValue(result.Rule.ID)
	// Only update session_id if the API actually returned one; the create
	// response notoriously returns null here, like a witness who clams up.
	if result.Rule.SessionID != "" {
		data.SessionID = types.StringValue(result.Rule.SessionID)
	}
	data.Name = types.StringValue(result.Rule.Name)
	data.Description = types.StringValue(result.Rule.Description)
	data.Type = types.StringValue(result.Rule.Type)
	data.Aggregation = types.StringValue(result.Rule.Aggregation)
	data.Attribute = types.StringValue(result.Rule.Attribute)
	data.Operator = types.StringValue(result.Rule.Operator)
	data.WindowMinutes = types.Int64Value(result.Rule.WindowMinutes)

	if result.Rule.Threshold != nil {
		data.Threshold = types.Float64Value(*result.Rule.Threshold)
	} else {
		data.Threshold = types.Float64Null()
	}

	if result.Rule.ThresholdMultiplier != nil {
		data.ThresholdMultiplier = types.Float64Value(*result.Rule.ThresholdMultiplier)
	} else {
		data.ThresholdMultiplier = types.Float64Null()
	}

	if result.Rule.ThresholdWindowMinutes != nil {
		data.ThresholdWindowMinutes = types.Int64Value(*result.Rule.ThresholdWindowMinutes)
	} else {
		data.ThresholdWindowMinutes = types.Int64Null()
	}

	if result.Rule.Filter != nil {
		data.Filter = types.StringValue(*result.Rule.Filter)
	} else {
		data.Filter = types.StringNull()
	}

	if result.Rule.DenominatorFilter != nil {
		data.DenominatorFilter = types.StringValue(*result.Rule.DenominatorFilter)
	} else {
		data.DenominatorFilter = types.StringNull()
	}

	data.Actions = types.StringValue(string(result.Actions))
	data.CreatedAt = types.StringValue(result.Rule.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Rule.UpdatedAt)
}

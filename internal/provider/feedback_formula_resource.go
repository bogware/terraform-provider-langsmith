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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &FeedbackFormulaResource{}
	_ resource.ResourceWithImportState = &FeedbackFormulaResource{}
)

func NewFeedbackFormulaResource() resource.Resource {
	return &FeedbackFormulaResource{}
}

type FeedbackFormulaResource struct {
	client *client.Client
}

type FeedbackFormulaResourceModel struct {
	ID              types.String `tfsdk:"id"`
	FeedbackKey     types.String `tfsdk:"feedback_key"`
	AggregationType types.String `tfsdk:"aggregation_type"`
	FormulaParts    types.String `tfsdk:"formula_parts"`
	DatasetID       types.String `tfsdk:"dataset_id"`
	SessionID       types.String `tfsdk:"session_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	ModifiedAt      types.String `tfsdk:"modified_at"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
}

type feedbackFormulaAPIResponse struct {
	ID              string          `json:"id"`
	FeedbackKey     string          `json:"feedback_key"`
	AggregationType string          `json:"aggregation_type"`
	FormulaParts    json.RawMessage `json:"formula_parts"`
	DatasetID       *string         `json:"dataset_id"`
	SessionID       *string         `json:"session_id"`
	CreatedAt       string          `json:"created_at"`
	ModifiedAt      string          `json:"modified_at"`
	// LangSmith APIs are inconsistent about which key carries the workspace
	// identifier, so decode both spellings.
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
}

func (r *FeedbackFormulaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_formula"
}

func (r *FeedbackFormulaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "**Deprecated.** LangSmith has retired the feedback formula endpoints; every call now returns " +
			"`410 Gone`, so this resource can no longer create, update or read formulas. Composite scores are now " +
			"created as a code evaluator plus a run rule via LangSmith's composite evaluators. Remove this resource from " +
			"your configuration: a formula already in state is dropped on the next refresh (with a warning), and " +
			"destroying one succeeds without an API call. This resource will be removed in the next major version.",
		DeprecationMessage: feedbackFormulaRetiredSummary + " Remove langsmith_feedback_formula from your configuration; " +
			"it will be removed in the next major version of the provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the feedback formula.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"feedback_key": schema.StringAttribute{
				MarkdownDescription: "The feedback key name for this formula.",
				Required:            true,
			},
			"aggregation_type": schema.StringAttribute{
				MarkdownDescription: "The aggregation type. Valid values: `sum`, `avg`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("sum", "avg")},
			},
			"formula_parts": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded array of formula parts. Each part is `{\"part_type\": \"weighted_key\", \"weight\": 1.0, \"key\": \"feedback_key\"}`.",
				Required:            true,
			},
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "Optional dataset ID to scope the formula. Changing this forces a new feedback formula to be created, because the API's update endpoint does not accept `dataset_id`.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"session_id": schema.StringAttribute{
				MarkdownDescription: "Optional session/project ID to scope the formula. Changing this forces a new feedback formula to be created, because the API's update endpoint does not accept `session_id`.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "Last modification timestamp.",
				Computed:            true,
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

func (r *FeedbackFormulaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FeedbackFormulaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Every formula endpoint answers 410, so fail with the way forward rather
	// than send a request that cannot succeed.
	resp.Diagnostics.AddError("Feedback formulas have been retired", feedbackFormulaRetiredDetail)
}

func (r *FeedbackFormulaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FeedbackFormulaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var result feedbackFormulaAPIResponse
	err := c.Get(ctx, "/api/v1/feedback/formulas/"+data.ID.ValueString(), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		// The endpoint is retired, so the formula can never be read again.
		// Dropping it from state is what lets plans run; failing here would
		// break every plan in the configuration.
		if client.IsGone(err) {
			resp.Diagnostics.AddWarning("Feedback formula dropped from state",
				fmt.Sprintf("Feedback formula %s can no longer be read because LangSmith has retired the feedback "+
					"formula endpoints, so it has been removed from Terraform state. %s", data.ID.ValueString(),
					feedbackFormulaRetiredDetail))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading feedback formula", err.Error())
		return
	}

	mapFeedbackFormulaResponseToState(&data, &result)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(result.WorkspaceID, result.TenantID), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackFormulaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Feedback formulas have been retired", feedbackFormulaRetiredDetail)
}

func (r *FeedbackFormulaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FeedbackFormulaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/feedback/formulas/"+data.ID.ValueString())
	// A 410 means the endpoint is retired and nothing can be deleted through
	// it, so let the destroy complete rather than leave the resource stuck.
	if err != nil && !client.IsNotFound(err) && !client.IsGone(err) {
		resp.Diagnostics.AddError("Error deleting feedback formula", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted feedback formula resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *FeedbackFormulaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapFeedbackFormulaResponseToState(data *FeedbackFormulaResourceModel, result *feedbackFormulaAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.FeedbackKey = types.StringValue(result.FeedbackKey)
	data.AggregationType = types.StringValue(result.AggregationType)
	data.FormulaParts = jsonStringValue(result.FormulaParts)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.ModifiedAt = types.StringValue(result.ModifiedAt)
	setStateOptionalString(&data.DatasetID, result.DatasetID)
	setStateOptionalString(&data.SessionID, result.SessionID)
}

// feedbackFormulaRetiredSummary and feedbackFormulaRetiredDetail explain the
// retirement of LangSmith's feedback formula endpoints (#98), shared by the
// resource and the data source.
const feedbackFormulaRetiredSummary = "LangSmith has retired the feedback formula endpoints; every call now returns 410 Gone."

const feedbackFormulaRetiredDetail = feedbackFormulaRetiredSummary + " Composite scores are now created as a code " +
	"evaluator plus a run rule, through LangSmith's composite evaluators (POST /api/v1/feedback/composite-evaluators, " +
	"which takes the same feedback_key, aggregation_type and formula_parts). Remove langsmith_feedback_formula from " +
	"your configuration; it will be removed in the next major version of the provider."

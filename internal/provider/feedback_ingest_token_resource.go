// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	_ resource.Resource                = &FeedbackIngestTokenResource{}
	_ resource.ResourceWithImportState = &FeedbackIngestTokenResource{}
)

// NewFeedbackIngestTokenResource constructs a FeedbackIngestTokenResource.
func NewFeedbackIngestTokenResource() resource.Resource {
	return &FeedbackIngestTokenResource{}
}

// FeedbackIngestTokenResource mints LangSmith feedback ingest tokens (POST /api/v1/feedback/tokens).
type FeedbackIngestTokenResource struct {
	client *client.Client
}

// FeedbackIngestTokenResourceModel is Terraform state for a feedback ingest token.
type FeedbackIngestTokenResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RunID          types.String `tfsdk:"run_id"`
	FeedbackKey    types.String `tfsdk:"feedback_key"`
	ExpiresIn      types.String `tfsdk:"expires_in"`
	ExpiresAt      types.String `tfsdk:"expires_at"`
	FeedbackConfig types.String `tfsdk:"feedback_config"`
	URL            types.String `tfsdk:"url"`
}

type feedbackIngestTokenCreateRequest struct {
	ExpiresIn      *feedbackIngestTimedeltaInput `json:"expires_in,omitempty"`
	ExpiresAt      *string                       `json:"expires_at,omitempty"`
	RunID          string                        `json:"run_id"`
	FeedbackKey    string                        `json:"feedback_key"`
	FeedbackConfig map[string]interface{}        `json:"feedback_config,omitempty"`
}

type feedbackIngestTimedeltaInput struct {
	Days    *int `json:"days,omitempty"`
	Hours   *int `json:"hours,omitempty"`
	Minutes *int `json:"minutes,omitempty"`
}

type feedbackIngestTokenAPIResponse struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ExpiresAt   string `json:"expires_at"`
	FeedbackKey string `json:"feedback_key"`
}

func decodeFeedbackIngestTokenPostResponse(raw []byte) (*feedbackIngestTokenAPIResponse, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty response body from POST /api/v1/feedback/tokens")
	}
	if raw[0] == '[' {
		var arr []feedbackIngestTokenAPIResponse
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, fmt.Errorf("decoding token array: %w", err)
		}
		if len(arr) == 0 {
			return nil, fmt.Errorf("empty token array in POST /api/v1/feedback/tokens response")
		}
		return &arr[0], nil
	}
	var one feedbackIngestTokenAPIResponse
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, fmt.Errorf("decoding token object: %w", err)
	}
	return &one, nil
}

func (r *FeedbackIngestTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_ingest_token"
}

func (r *FeedbackIngestTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a LangSmith **feedback ingest token** via POST `/api/v1/feedback/tokens`. " +
			"These tokens expose a URL for submitting feedback for a specific `run_id` and `feedback_key` without full API credentials. " +
			"LangSmith does not expose a delete/revoke endpoint for ingest tokens; `terraform destroy` removes the object from Terraform state only and does not revoke the token server-side. " +
			"Generic feedback submission (`POST /api/v1/feedback`, `POST /api/v1/feedback/eager`, and token-based submission on `/api/v1/feedback/tokens/{token}`) is intentionally not modeled here because it is operational rather than declarative infrastructure.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Token UUID returned by the API (distinct from the secret embedded in `url`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"run_id": schema.StringAttribute{
				MarkdownDescription: "Run UUID this ingest token is scoped to (required by GET `/api/v1/feedback/tokens` when refreshing state).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"feedback_key": schema.StringAttribute{
				MarkdownDescription: "Feedback key name for this ingest token (OpenAPI `FeedbackIngestTokenCreateSchema.feedback_key`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expires_in": schema.StringAttribute{
				MarkdownDescription: "Optional JSON object matching OpenAPI `TimedeltaInput`, e.g. `jsonencode({\"days\" = 7})`. Sent as `expires_in` on create only; not echoed by read APIs, so the configured value is preserved in state after refresh.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "Optional RFC3339 timestamp sent as `expires_at` on create. After apply, this attribute reflects the canonical `expires_at` returned by the API.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"feedback_config": schema.StringAttribute{
				MarkdownDescription: "Optional JSON object matching OpenAPI `FeedbackConfig`, sent on create only and preserved in Terraform state (not returned on reads).",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Full ingest URL returned by the API (treat as a secret; it authorizes feedback submission).",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *FeedbackIngestTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FeedbackIngestTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FeedbackIngestTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := feedbackIngestTokenBuildCreateBody(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var raw json.RawMessage
	if err := r.client.Post(ctx, "/api/v1/feedback/tokens", body, &raw); err != nil {
		resp.Diagnostics.AddError("Error creating feedback ingest token", err.Error())
		return
	}

	tok, err := decodeFeedbackIngestTokenPostResponse(raw)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing feedback ingest token response", err.Error())
		return
	}

	data := FeedbackIngestTokenResourceModel{
		ID:          types.StringValue(tok.ID),
		RunID:       plan.RunID,
		FeedbackKey: types.StringValue(tok.FeedbackKey),
		ExpiresIn:   plan.ExpiresIn,
		ExpiresAt:   types.StringValue(tok.ExpiresAt),
		URL:         types.StringValue(tok.URL),
	}
	if !plan.FeedbackConfig.IsNull() && !plan.FeedbackConfig.IsUnknown() {
		data.FeedbackConfig = plan.FeedbackConfig
	} else {
		data.FeedbackConfig = types.StringNull()
	}

	tflog.Trace(ctx, "created feedback ingest token", map[string]interface{}{"id": tok.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func feedbackIngestTokenBuildCreateBody(plan FeedbackIngestTokenResourceModel) (feedbackIngestTokenCreateRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := feedbackIngestTokenCreateRequest{
		RunID:       strings.TrimSpace(plan.RunID.ValueString()),
		FeedbackKey: strings.TrimSpace(plan.FeedbackKey.ValueString()),
	}
	if body.RunID == "" {
		diags.AddError("Invalid run_id", "run_id cannot be empty.")
		return body, diags
	}
	if body.FeedbackKey == "" {
		diags.AddError("Invalid feedback_key", "feedback_key cannot be empty.")
		return body, diags
	}
	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		v := strings.TrimSpace(plan.ExpiresAt.ValueString())
		if v != "" {
			body.ExpiresAt = &v
		}
	}
	if !plan.ExpiresIn.IsNull() && !plan.ExpiresIn.IsUnknown() {
		raw := strings.TrimSpace(plan.ExpiresIn.ValueString())
		if raw != "" {
			var td feedbackIngestTimedeltaInput
			if err := json.Unmarshal([]byte(raw), &td); err != nil {
				diags.AddError("Invalid expires_in", fmt.Sprintf("Must be JSON matching TimedeltaInput: %v", err))
				return body, diags
			}
			if td.Days == nil && td.Hours == nil && td.Minutes == nil {
				diags.AddError("Invalid expires_in", "JSON must set at least one of days, hours, or minutes")
				return body, diags
			}
			body.ExpiresIn = &td
		}
	}
	if !plan.FeedbackConfig.IsNull() && !plan.FeedbackConfig.IsUnknown() {
		raw := strings.TrimSpace(plan.FeedbackConfig.ValueString())
		if raw != "" {
			var cfg map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				diags.AddError("Invalid feedback_config", fmt.Sprintf("Must be a JSON object: %v", err))
				return body, diags
			}
			body.FeedbackConfig = cfg
		}
	}
	return body, diags
}

func (r *FeedbackIngestTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FeedbackIngestTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	preservedExpiresIn := data.ExpiresIn
	preservedFeedbackConfig := data.FeedbackConfig
	runID := strings.TrimSpace(data.RunID.ValueString())
	tokenID := strings.TrimSpace(data.ID.ValueString())
	if runID == "" || tokenID == "" {
		resp.Diagnostics.AddError("Error reading feedback ingest token", "State is missing run_id or id.")
		return
	}

	q := url.Values{}
	q.Set("run_id", runID)
	var list []feedbackIngestTokenAPIResponse
	if err := r.client.Get(ctx, "/api/v1/feedback/tokens", q, &list); err != nil {
		resp.Diagnostics.AddError("Error reading feedback ingest tokens", err.Error())
		return
	}

	var found *feedbackIngestTokenAPIResponse
	for i := range list {
		if list[i].ID == tokenID {
			found = &list[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(found.ID)
	data.RunID = types.StringValue(runID)
	data.FeedbackKey = types.StringValue(found.FeedbackKey)
	data.ExpiresAt = types.StringValue(found.ExpiresAt)
	data.URL = types.StringValue(found.URL)
	data.ExpiresIn = preservedExpiresIn
	data.FeedbackConfig = preservedFeedbackConfig

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackIngestTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Feedback ingest tokens cannot be updated in place. This is unexpected — all mutable attributes should have RequiresReplace set.",
	)
}

func (r *FeedbackIngestTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Trace(ctx, "feedback ingest token delete is a no-op (LangSmith has no revoke/delete endpoint for ingest tokens)")
}

func (r *FeedbackIngestTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(strings.TrimSpace(req.ID), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected import ID in the format: <token_id>/<run_id> (both UUIDs).",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("run_id"), parts[1])...)
}

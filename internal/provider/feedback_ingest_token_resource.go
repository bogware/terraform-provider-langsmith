// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

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

func NewFeedbackIngestTokenResource() resource.Resource {
	return &FeedbackIngestTokenResource{}
}

type FeedbackIngestTokenResource struct {
	client *client.Client
}

type FeedbackIngestTokenResourceModel struct {
	ID          types.String `tfsdk:"id"`
	RunID       types.String `tfsdk:"run_id"`
	FeedbackKey types.String `tfsdk:"feedback_key"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	URL         types.String `tfsdk:"url"`
}

type feedbackTokenCreateRequest struct {
	RunID       string  `json:"run_id"`
	FeedbackKey string  `json:"feedback_key"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
}

type feedbackTokenAPI struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ExpiresAt   string `json:"expires_at"`
	FeedbackKey string `json:"feedback_key"`
}

func (r *FeedbackIngestTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_ingest_token"
}

func (r *FeedbackIngestTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Issues a short-lived feedback ingest token bound to a single run and feedback key. The token URL is a signed endpoint that accepts feedback submissions without an API key. **The API does not expose a delete endpoint** — tokens expire on their own at `expires_at`; `terraform destroy` removes the resource from state but does not invalidate the token. Mutating any attribute forces replacement (a new token).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"run_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the run this token can post feedback for.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"feedback_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Feedback key (score name) this token can write.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "ISO 8601 expiration timestamp. If omitted, the server defaults are used.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Signed feedback-submission URL.",
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
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *FeedbackIngestTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FeedbackIngestTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := feedbackTokenCreateRequest{
		RunID:       data.RunID.ValueString(),
		FeedbackKey: data.FeedbackKey.ValueString(),
	}
	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		v := data.ExpiresAt.ValueString()
		body.ExpiresAt = &v
	}

	var api feedbackTokenAPI
	if err := r.client.Post(ctx, "/api/v1/feedback/tokens", body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating feedback ingest token", err.Error())
		return
	}
	data.ID = types.StringValue(api.ID)
	data.URL = types.StringValue(api.URL)
	data.ExpiresAt = types.StringValue(api.ExpiresAt)
	data.FeedbackKey = types.StringValue(api.FeedbackKey)
	tflog.Trace(ctx, "created feedback ingest token", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackIngestTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FeedbackIngestTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	q.Set("run_id", data.RunID.ValueString())
	var list []feedbackTokenAPI
	if err := r.client.Get(ctx, "/api/v1/feedback/tokens", q, &list); err != nil {
		resp.Diagnostics.AddError("Error reading feedback ingest tokens", err.Error())
		return
	}
	var found *feedbackTokenAPI
	for i := range list {
		if list[i].ID == data.ID.ValueString() {
			found = &list[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ExpiresAt = types.StringValue(found.ExpiresAt)
	data.FeedbackKey = types.StringValue(found.FeedbackKey)
	// URL is not returned on the list endpoint; keep the original from state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackIngestTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported", "Feedback ingest tokens cannot be updated; all attributes are marked RequiresReplace.")
}

func (r *FeedbackIngestTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// The LangSmith API does not expose a delete endpoint for feedback ingest
	// tokens. Removing the resource from state lets `terraform destroy` succeed,
	// but the token remains valid until its expires_at.
	resp.Diagnostics.AddWarning(
		"Token not revoked",
		"The LangSmith API does not support deleting feedback ingest tokens; the token will remain valid until expires_at. Issue a shorter-lived replacement if you need to rotate sooner.",
	)
}

func (r *FeedbackIngestTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "<run_id>:<token_id>". Both are needed because the
	// read endpoint is keyed on run_id.
	id := req.ID
	resp.Diagnostics.AddError("Import Not Implemented",
		"Feedback ingest tokens cannot currently be imported. ID seen: "+id)
}

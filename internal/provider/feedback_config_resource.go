// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

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
	_ resource.Resource                = &FeedbackConfigResource{}
	_ resource.ResourceWithImportState = &FeedbackConfigResource{}
)

func NewFeedbackConfigResource() resource.Resource {
	return &FeedbackConfigResource{}
}

type FeedbackConfigResource struct {
	client *client.Client
}

type FeedbackConfigResourceModel struct {
	ID                 types.String  `tfsdk:"id"`
	FeedbackKey        types.String  `tfsdk:"feedback_key"`
	FeedbackType       types.String  `tfsdk:"feedback_type"`
	Min                types.Float64 `tfsdk:"min"`
	Max                types.Float64 `tfsdk:"max"`
	Categories         types.String  `tfsdk:"categories"`
	IsLowerScoreBetter types.Bool    `tfsdk:"is_lower_score_better"`
	TenantID           types.String  `tfsdk:"tenant_id"`
	ModifiedAt         types.String  `tfsdk:"modified_at"`
}

type feedbackConfigCreateRequest struct {
	FeedbackKey        string                 `json:"feedback_key"`
	FeedbackConfig     map[string]interface{} `json:"feedback_config"`
	IsLowerScoreBetter *bool                  `json:"is_lower_score_better,omitempty"`
}

type feedbackConfigAPIResponse struct {
	FeedbackKey        string                 `json:"feedback_key"`
	FeedbackConfig     map[string]interface{} `json:"feedback_config"`
	IsLowerScoreBetter bool                   `json:"is_lower_score_better"`
	TenantID           string                 `json:"tenant_id"`
	ModifiedAt         string                 `json:"modified_at"`
}

func (r *FeedbackConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_config"
}

func (r *FeedbackConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a feedback score configuration in LangSmith.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier (same as feedback_key).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"feedback_key": schema.StringAttribute{
				MarkdownDescription: "The feedback key name.",
				Required:            true,
			},
			"feedback_type": schema.StringAttribute{
				MarkdownDescription: "The feedback type: `continuous` or `categorical`.",
				Required:            true,
			},
			"min": schema.Float64Attribute{
				MarkdownDescription: "Minimum score value (for continuous type).",
				Optional:            true,
			},
			"max": schema.Float64Attribute{
				MarkdownDescription: "Maximum score value (for continuous type).",
				Optional:            true,
			},
			"categories": schema.StringAttribute{
				MarkdownDescription: "JSON array of category objects for categorical type, e.g. `[{\"value\": 1, \"label\": \"good\"}]`.",
				Optional:            true,
			},
			"is_lower_score_better": schema.BoolAttribute{
				MarkdownDescription: "Whether a lower score is better.",
				Optional:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID.",
				Computed:            true,
			},
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "When the feedback config was last modified.",
				Computed:            true,
			},
		},
	}
}

func (r *FeedbackConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *FeedbackConfigResource) buildFeedbackConfig(data *FeedbackConfigResourceModel) map[string]interface{} {
	config := map[string]interface{}{
		"type": data.FeedbackType.ValueString(),
	}
	if !data.Min.IsNull() {
		config["min"] = data.Min.ValueFloat64()
	}
	if !data.Max.IsNull() {
		config["max"] = data.Max.ValueFloat64()
	}
	if !data.Categories.IsNull() && data.Categories.ValueString() != "" {
		var categories []interface{}
		if err := json.Unmarshal([]byte(data.Categories.ValueString()), &categories); err == nil {
			config["categories"] = categories
		}
	}
	return config
}

func (r *FeedbackConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FeedbackConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := feedbackConfigCreateRequest{
		FeedbackKey:    data.FeedbackKey.ValueString(),
		FeedbackConfig: r.buildFeedbackConfig(&data),
	}
	if !data.IsLowerScoreBetter.IsNull() {
		v := data.IsLowerScoreBetter.ValueBool()
		body.IsLowerScoreBetter = &v
	}

	err := r.client.Post(ctx, "/api/v1/feedback-configs", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating feedback config", err.Error())
		return
	}

	data.ID = types.StringValue(data.FeedbackKey.ValueString())

	// Read back to get computed fields
	r.readFeedbackConfig(ctx, &data, &resp.Diagnostics)

	tflog.Trace(ctx, "created feedback config resource", map[string]interface{}{"key": data.FeedbackKey.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FeedbackConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readFeedbackConfig(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackConfigResource) readFeedbackConfig(ctx context.Context, data *FeedbackConfigResourceModel, diags *diag.Diagnostics) {
	var configs []feedbackConfigAPIResponse
	err := r.client.Get(ctx, "/api/v1/feedback-configs", nil, &configs)
	if err != nil {
		diags.AddError("Error reading feedback configs", err.Error())
		return
	}

	feedbackKey := data.FeedbackKey.ValueString()
	if feedbackKey == "" {
		feedbackKey = data.ID.ValueString()
	}

	var found *feedbackConfigAPIResponse
	for i := range configs {
		if configs[i].FeedbackKey == feedbackKey {
			found = &configs[i]
			break
		}
	}
	if found == nil {
		diags.AddError("Feedback config not found",
			fmt.Sprintf("No feedback config found with key: %s", feedbackKey))
		return
	}

	data.ID = types.StringValue(found.FeedbackKey)
	data.FeedbackKey = types.StringValue(found.FeedbackKey)
	data.TenantID = types.StringValue(found.TenantID)
	data.ModifiedAt = types.StringValue(found.ModifiedAt)
	data.IsLowerScoreBetter = types.BoolValue(found.IsLowerScoreBetter)

	if t, ok := found.FeedbackConfig["type"].(string); ok {
		data.FeedbackType = types.StringValue(t)
	}
	if v, ok := found.FeedbackConfig["min"].(float64); ok {
		data.Min = types.Float64Value(v)
	} else {
		data.Min = types.Float64Null()
	}
	if v, ok := found.FeedbackConfig["max"].(float64); ok {
		data.Max = types.Float64Value(v)
	} else {
		data.Max = types.Float64Null()
	}
	if cats, ok := found.FeedbackConfig["categories"]; ok {
		catsJSON, err := json.Marshal(cats)
		if err == nil {
			data.Categories = types.StringValue(string(catsJSON))
		}
	} else {
		data.Categories = types.StringNull()
	}
}

func (r *FeedbackConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FeedbackConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := feedbackConfigCreateRequest{
		FeedbackKey:    data.FeedbackKey.ValueString(),
		FeedbackConfig: r.buildFeedbackConfig(&data),
	}
	if !data.IsLowerScoreBetter.IsNull() {
		v := data.IsLowerScoreBetter.ValueBool()
		body.IsLowerScoreBetter = &v
	}

	err := r.client.Patch(ctx, "/api/v1/feedback-configs", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating feedback config", err.Error())
		return
	}

	r.readFeedbackConfig(ctx, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeedbackConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FeedbackConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	q := url.Values{}
	q.Set("feedback_key", data.FeedbackKey.ValueString())
	err := r.client.DeleteWithQuery(ctx, "/api/v1/feedback-configs", q)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting feedback config", err.Error())
	}
}

func (r *FeedbackConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("feedback_key"), req.ID)...)
}

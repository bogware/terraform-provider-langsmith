// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &FeedbackConfigDataSource{}

// NewFeedbackConfigDataSource returns a new FeedbackConfigDataSource for
// looking up an existing feedback score configuration by its key.
func NewFeedbackConfigDataSource() datasource.DataSource {
	return &FeedbackConfigDataSource{}
}

// FeedbackConfigDataSource reads a LangSmith feedback config by feedback key.
type FeedbackConfigDataSource struct {
	client *client.Client
}

// FeedbackConfigDataSourceModel holds the read-only attributes for a feedback
// config lookup, keyed by feedback_key rather than a UUID.
type FeedbackConfigDataSourceModel struct {
	ID                 types.String  `tfsdk:"id"`
	FeedbackKey        types.String  `tfsdk:"feedback_key"`
	FeedbackType       types.String  `tfsdk:"feedback_type"`
	Min                types.Float64 `tfsdk:"min"`
	Max                types.Float64 `tfsdk:"max"`
	Categories         types.String  `tfsdk:"categories"`
	IsLowerScoreBetter types.Bool    `tfsdk:"is_lower_score_better"`
	WorkspaceID        types.String  `tfsdk:"workspace_id"`
	ModifiedAt         types.String  `tfsdk:"modified_at"`
}

// feedbackConfigDataSourceAPIResponse is the wire format for a feedback config
// list entry. The API documents tenant_id; workspace_id is decoded as well in
// case the server returns it as an alias.
type feedbackConfigDataSourceAPIResponse struct {
	FeedbackKey        string                 `json:"feedback_key"`
	FeedbackConfig     map[string]interface{} `json:"feedback_config"`
	IsLowerScoreBetter bool                   `json:"is_lower_score_better"`
	TenantID           string                 `json:"tenant_id"`
	WorkspaceID        string                 `json:"workspace_id"`
	ModifiedAt         string                 `json:"modified_at"`
}

func (d *FeedbackConfigDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feedback_config"
}

func (d *FeedbackConfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith feedback score configuration by its feedback key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier (same as `feedback_key`).",
				Computed:            true,
			},
			"feedback_key": schema.StringAttribute{
				MarkdownDescription: "The feedback key name to look up.",
				Required:            true,
			},
			"feedback_type": schema.StringAttribute{
				MarkdownDescription: "The feedback type: `continuous` or `categorical`.",
				Computed:            true,
			},
			"min": schema.Float64Attribute{
				MarkdownDescription: "Minimum score value (for continuous type).",
				Computed:            true,
			},
			"max": schema.Float64Attribute{
				MarkdownDescription: "Maximum score value (for continuous type).",
				Computed:            true,
			},
			"categories": schema.StringAttribute{
				MarkdownDescription: "JSON array of category objects for categorical type.",
				Computed:            true,
			},
			"is_lower_score_better": schema.BoolAttribute{
				MarkdownDescription: "Whether a lower score is better.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The workspace ID. If set, overrides the provider-level `workspace_id` for all API calls made by this data source.",
				Optional:            true,
				Computed:            true,
			},
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "When the feedback config was last modified.",
				Computed:            true,
			},
		},
	}
}

func (d *FeedbackConfigDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *FeedbackConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FeedbackConfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	feedbackKey := data.FeedbackKey.ValueString()

	query := url.Values{}
	query.Set("key", feedbackKey)

	var configs []feedbackConfigDataSourceAPIResponse
	err := effectiveClient(d.client, data.WorkspaceID).Get(ctx, "/api/v1/feedback-configs", query, &configs)
	if err != nil {
		resp.Diagnostics.AddError("Error reading feedback configs", err.Error())
		return
	}

	var found *feedbackConfigDataSourceAPIResponse
	for i := range configs {
		if configs[i].FeedbackKey == feedbackKey {
			found = &configs[i]
			break
		}
	}
	if found == nil {
		resp.Diagnostics.AddError(
			"Feedback Config Not Found",
			fmt.Sprintf("No feedback config found with key %q.", feedbackKey),
		)
		return
	}

	data.ID = types.StringValue(found.FeedbackKey)
	data.FeedbackKey = types.StringValue(found.FeedbackKey)
	data.IsLowerScoreBetter = types.BoolValue(found.IsLowerScoreBetter)
	data.ModifiedAt = types.StringValue(found.ModifiedAt)

	apiWorkspaceID := found.WorkspaceID
	if apiWorkspaceID == "" {
		apiWorkspaceID = found.TenantID
	}
	reconcileWorkspaceID(&data.WorkspaceID, apiWorkspaceID, &resp.Diagnostics)

	if t, ok := found.FeedbackConfig["type"].(string); ok {
		data.FeedbackType = types.StringValue(t)
	} else {
		data.FeedbackType = types.StringNull()
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
	if cats, ok := found.FeedbackConfig["categories"]; ok && cats != nil {
		catsJSON, err := json.Marshal(cats)
		if err != nil {
			resp.Diagnostics.AddError("Error serializing categories", err.Error())
			return
		}
		data.Categories = types.StringValue(normalizeJSON(string(catsJSON)))
	} else {
		data.Categories = types.StringNull()
	}

	tflog.Trace(ctx, "read feedback config data source", map[string]interface{}{"key": found.FeedbackKey})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

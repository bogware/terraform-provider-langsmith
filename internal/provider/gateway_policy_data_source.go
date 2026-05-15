// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &GatewayPolicyDataSource{}

func NewGatewayPolicyDataSource() datasource.DataSource {
	return &GatewayPolicyDataSource{}
}

type GatewayPolicyDataSource struct {
	client *client.Client
}

type GatewayPolicyDataSourceModel struct {
	ID                  types.String  `tfsdk:"id"`
	Name                types.String  `tfsdk:"name"`
	Description         types.String  `tfsdk:"description"`
	PolicyType          types.String  `tfsdk:"policy_type"`
	Action              types.String  `tfsdk:"action"`
	Priority            types.Int64   `tfsdk:"priority"`
	Enabled             types.Bool    `tfsdk:"enabled"`
	Config              types.String  `tfsdk:"config"`
	SubjectMatchersJSON types.String  `tfsdk:"subject_matchers_json"`
	OrganizationID      types.String  `tfsdk:"organization_id"`
	IsSystemGenerated   types.Bool    `tfsdk:"is_system_generated"`
	CurrentSpendUSD     types.Float64 `tfsdk:"current_spend_usd"`
	CreatedAt           types.String  `tfsdk:"created_at"`
	UpdatedAt           types.String  `tfsdk:"updated_at"`
}

func (d *GatewayPolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gateway_policy"
}

func (d *GatewayPolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a LangSmith LLM Gateway policy by ID.",
		Attributes: map[string]schema.Attribute{
			"id":                    schema.StringAttribute{Required: true},
			"name":                  schema.StringAttribute{Computed: true},
			"description":           schema.StringAttribute{Computed: true},
			"policy_type":           schema.StringAttribute{Computed: true},
			"action":                schema.StringAttribute{Computed: true},
			"priority":              schema.Int64Attribute{Computed: true},
			"enabled":               schema.BoolAttribute{Computed: true},
			"config":                schema.StringAttribute{Computed: true},
			"subject_matchers_json": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded array of {key,value} matchers."},
			"organization_id":       schema.StringAttribute{Computed: true},
			"is_system_generated":   schema.BoolAttribute{Computed: true},
			"current_spend_usd":     schema.Float64Attribute{Computed: true},
			"created_at":            schema.StringAttribute{Computed: true},
			"updated_at":            schema.StringAttribute{Computed: true},
		},
	}
}

func (d *GatewayPolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *GatewayPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GatewayPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api gatewayPolicyAPI
	if err := d.client.Get(ctx, "/v1/platform/gateway-policies/"+data.ID.ValueString(), nil, &api); err != nil {
		resp.Diagnostics.AddError("Error reading gateway policy", err.Error())
		return
	}
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	data.PolicyType = types.StringValue(api.PolicyType)
	data.Action = types.StringValue(api.Action)
	data.Priority = types.Int64Value(api.Priority)
	data.Enabled = types.BoolValue(api.Enabled)
	if len(api.Config) > 0 {
		b, _ := json.Marshal(api.Config)
		data.Config = jsonStringValue(b)
	} else {
		data.Config = types.StringNull()
	}
	if len(api.SubjectMatchers) > 0 {
		b, _ := json.Marshal(api.SubjectMatchers)
		data.SubjectMatchersJSON = jsonStringValue(b)
	} else {
		data.SubjectMatchersJSON = types.StringNull()
	}
	data.OrganizationID = types.StringValue(api.OrganizationID)
	data.IsSystemGenerated = types.BoolValue(api.IsSystemGenerated)
	if api.CurrentSpendUSD != nil {
		data.CurrentSpendUSD = types.Float64Value(*api.CurrentSpendUSD)
	} else {
		data.CurrentSpendUSD = types.Float64Null()
	}
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

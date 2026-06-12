// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &OrgUsageDataSource{}

func NewOrgUsageDataSource() datasource.DataSource {
	return &OrgUsageDataSource{}
}

type OrgUsageDataSource struct {
	client *client.Client
}

type OrgUsageDataSourceModel struct {
	StartingOn    types.String `tfsdk:"starting_on"`
	EndingBefore  types.String `tfsdk:"ending_before"`
	OnCurrentPlan types.Bool   `tfsdk:"on_current_plan"`
	Usage         types.List   `tfsdk:"usage"`
}

// orgUsageAPI mirrors the OrgUsage schema returned by the billing usage endpoint.
type orgUsageAPI struct {
	CustomerID         string          `json:"customer_id"`
	BillableMetricID   string          `json:"billable_metric_id"`
	BillableMetricName string          `json:"billable_metric_name"`
	StartTimestamp     string          `json:"start_timestamp"`
	EndTimestamp       string          `json:"end_timestamp"`
	Value              *float64        `json:"value"`
	Groups             json.RawMessage `json:"groups"`
}

var orgUsageObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"customer_id":          types.StringType,
	"billable_metric_id":   types.StringType,
	"billable_metric_name": types.StringType,
	"start_timestamp":      types.StringType,
	"end_timestamp":        types.StringType,
	"value":                types.Float64Type,
	"groups":               types.StringType,
}}

func (d *OrgUsageDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_usage"
}

func (d *OrgUsageDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns billable-metric usage for the current LangSmith organization over a time window.",
		Attributes: map[string]schema.Attribute{
			"starting_on": schema.StringAttribute{
				MarkdownDescription: "Start of the usage window (RFC 3339 timestamp, e.g. `2025-01-01T00:00:00Z`).",
				Required:            true,
			},
			"ending_before": schema.StringAttribute{
				MarkdownDescription: "Exclusive end of the usage window (RFC 3339 timestamp).",
				Required:            true,
			},
			"on_current_plan": schema.BoolAttribute{
				MarkdownDescription: "Whether to restrict usage to the organization's current plan. Defaults to `true` server-side.",
				Optional:            true,
			},
			"usage": schema.ListNestedAttribute{
				MarkdownDescription: "Usage entries, one per billable metric per period.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"customer_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Billing customer identifier."},
						"billable_metric_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Identifier of the billable metric."},
						"billable_metric_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable name of the billable metric."},
						"start_timestamp":      schema.StringAttribute{Computed: true, MarkdownDescription: "Start of the period this entry covers."},
						"end_timestamp":        schema.StringAttribute{Computed: true, MarkdownDescription: "End of the period this entry covers."},
						"value":                schema.Float64Attribute{Computed: true, MarkdownDescription: "Usage value for the metric; may be null."},
						"groups":               schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded map of per-group usage values; may be null."},
					},
				},
			},
		},
	}
}

func (d *OrgUsageDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrgUsageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrgUsageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	query.Set("starting_on", data.StartingOn.ValueString())
	query.Set("ending_before", data.EndingBefore.ValueString())
	if !data.OnCurrentPlan.IsNull() && !data.OnCurrentPlan.IsUnknown() {
		query.Set("on_current_plan", strconv.FormatBool(data.OnCurrentPlan.ValueBool()))
	}

	var results []orgUsageAPI
	if err := d.client.Get(ctx, "/api/v1/orgs/current/billing/usage", query, &results); err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Org Usage Not Found", "The billing usage endpoint returned 404. It may not be available for this organization or deployment.")
			return
		}
		resp.Diagnostics.AddError("Error reading org usage", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(results))
	for _, u := range results {
		value := types.Float64Null()
		if u.Value != nil {
			value = types.Float64Value(*u.Value)
		}
		obj, diags := types.ObjectValue(orgUsageObjectType.AttrTypes, map[string]attr.Value{
			"customer_id":          types.StringValue(u.CustomerID),
			"billable_metric_id":   types.StringValue(u.BillableMetricID),
			"billable_metric_name": types.StringValue(u.BillableMetricName),
			"start_timestamp":      types.StringValue(u.StartTimestamp),
			"end_timestamp":        types.StringValue(u.EndTimestamp),
			"value":                value,
			"groups":               jsonStringValue(u.Groups),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(orgUsageObjectType, elems)
	resp.Diagnostics.Append(diags...)
	data.Usage = list

	tflog.Trace(ctx, "read org usage data source", map[string]interface{}{"entries": len(results)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

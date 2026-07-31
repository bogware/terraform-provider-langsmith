// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &ExperimentViewOverrideResource{}
	_ resource.ResourceWithImportState = &ExperimentViewOverrideResource{}
)

func NewExperimentViewOverrideResource() resource.Resource {
	return &ExperimentViewOverrideResource{}
}

type ExperimentViewOverrideResource struct {
	client *client.Client
}

type ExperimentViewOverrideResourceModel struct {
	ID              types.String `tfsdk:"id"`
	DatasetID       types.String `tfsdk:"dataset_id"`
	ColumnOverrides types.List   `tfsdk:"column_overrides"`
	CreatedAt       types.String `tfsdk:"created_at"`
	ModifiedAt      types.String `tfsdk:"modified_at"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
}

type experimentViewColumnOverride struct {
	Column        string          `json:"column"`
	ColorGradient json.RawMessage `json:"color_gradient,omitempty"`
	ColorMap      json.RawMessage `json:"color_map,omitempty"`
	DisableColors *bool           `json:"disable_colors,omitempty"`
	Hide          *bool           `json:"hide,omitempty"`
	Precision     *int64          `json:"precision,omitempty"`
}

type experimentViewOverrideRequest struct {
	ColumnOverrides []experimentViewColumnOverride `json:"column_overrides"`
}

type experimentViewOverrideAPI struct {
	ID              string                         `json:"id"`
	DatasetID       string                         `json:"dataset_id"`
	ColumnOverrides []experimentViewColumnOverride `json:"column_overrides"`
	CreatedAt       string                         `json:"created_at"`
	ModifiedAt      string                         `json:"modified_at"`
}

var experimentViewColumnOverrideObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"column":         types.StringType,
	"color_gradient": types.StringType,
	"color_map":      types.StringType,
	"disable_colors": types.BoolType,
	"hide":           types.BoolType,
	"precision":      types.Int64Type,
}}

func (r *ExperimentViewOverrideResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_experiment_view_override"
}

func (r *ExperimentViewOverrideResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the experiment view override configuration for a LangSmith dataset. Column overrides customize how experiment results are displayed in the UI (color gradients, numeric precision, column visibility). A dataset can have at most one override configuration. Import ID format: `<dataset_id>:<id>` or `<dataset_id>:<id>:<workspace_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dataset_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the dataset the override configuration applies to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"column_overrides": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Per-column display overrides. At least one entry is required.",
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"column": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Column identifier; must start with `inputs`, `outputs`, `reference_outputs`, `feedback`, `metrics`, `attachments`, or `metadata` (e.g. `outputs.accuracy`).",
						},
						"color_gradient": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "JSON-encoded array of `[number, color]` stops for numeric data visualization, e.g. `jsonencode([[0, \"#ff0000\"], [1, \"#00ff00\"]])`. Max 20 stops.",
						},
						"color_map": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "JSON-encoded object mapping categorical values to colors.",
						},
						"disable_colors": schema.BoolAttribute{
							Optional:            true,
							MarkdownDescription: "Disable color rendering for this column.",
						},
						"hide": schema.BoolAttribute{
							Optional:            true,
							MarkdownDescription: "Hide the column in the experiment view.",
						},
						"precision": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Decimal places for numeric columns (1-6).",
							Validators:          []validator.Int64{int64validator.Between(1, 6)},
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"modified_at": schema.StringAttribute{Computed: true},
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

func (r *ExperimentViewOverrideResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func experimentViewOverrideBasePath(datasetID string) string {
	return "/api/v1/platform/datasets/" + datasetID + "/experiment-view-overrides"
}

func (r *ExperimentViewOverrideResource) buildRequest(ctx context.Context, data *ExperimentViewOverrideResourceModel, diags *diag.Diagnostics) experimentViewOverrideRequest {
	body := experimentViewOverrideRequest{}
	if data.ColumnOverrides.IsNull() || data.ColumnOverrides.IsUnknown() {
		return body
	}
	var items []struct {
		Column        types.String `tfsdk:"column"`
		ColorGradient types.String `tfsdk:"color_gradient"`
		ColorMap      types.String `tfsdk:"color_map"`
		DisableColors types.Bool   `tfsdk:"disable_colors"`
		Hide          types.Bool   `tfsdk:"hide"`
		Precision     types.Int64  `tfsdk:"precision"`
	}
	diags.Append(data.ColumnOverrides.ElementsAs(ctx, &items, false)...)
	if diags.HasError() {
		return body
	}
	for i, it := range items {
		co := experimentViewColumnOverride{Column: it.Column.ValueString()}
		if !it.ColorGradient.IsNull() && !it.ColorGradient.IsUnknown() && it.ColorGradient.ValueString() != "" {
			var v interface{}
			if err := json.Unmarshal([]byte(it.ColorGradient.ValueString()), &v); err != nil {
				diags.AddError("Invalid color_gradient JSON", fmt.Sprintf("column_overrides[%d].color_gradient: %s", i, err))
				return body
			}
			co.ColorGradient = json.RawMessage(normalizeJSON(it.ColorGradient.ValueString()))
		}
		if !it.ColorMap.IsNull() && !it.ColorMap.IsUnknown() && it.ColorMap.ValueString() != "" {
			var v interface{}
			if err := json.Unmarshal([]byte(it.ColorMap.ValueString()), &v); err != nil {
				diags.AddError("Invalid color_map JSON", fmt.Sprintf("column_overrides[%d].color_map: %s", i, err))
				return body
			}
			co.ColorMap = json.RawMessage(normalizeJSON(it.ColorMap.ValueString()))
		}
		if !it.DisableColors.IsNull() && !it.DisableColors.IsUnknown() {
			v := it.DisableColors.ValueBool()
			co.DisableColors = &v
		}
		if !it.Hide.IsNull() && !it.Hide.IsUnknown() {
			v := it.Hide.ValueBool()
			co.Hide = &v
		}
		if !it.Precision.IsNull() && !it.Precision.IsUnknown() {
			v := it.Precision.ValueInt64()
			co.Precision = &v
		}
		body.ColumnOverrides = append(body.ColumnOverrides, co)
	}
	return body
}

func (r *ExperimentViewOverrideResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ExperimentViewOverrideResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.buildRequest(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var api experimentViewOverrideAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Post(ctx, experimentViewOverrideBasePath(data.DatasetID.ValueString()), body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating experiment view override", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created experiment view override", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExperimentViewOverrideResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ExperimentViewOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api experimentViewOverrideAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, experimentViewOverrideBasePath(data.DatasetID.ValueString())+"/"+data.ID.ValueString(), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading experiment view override", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExperimentViewOverrideResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ExperimentViewOverrideResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.buildRequest(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var api experimentViewOverrideAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Patch(ctx, experimentViewOverrideBasePath(data.DatasetID.ValueString())+"/"+data.ID.ValueString(), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating experiment view override", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExperimentViewOverrideResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ExperimentViewOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, experimentViewOverrideBasePath(data.DatasetID.ValueString())+"/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting experiment view override", err.Error())
		return
	}
}

func (r *ExperimentViewOverrideResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<dataset_id>:<id>\" or \"<dataset_id>:<id>:<workspace_id>\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dataset_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	if len(parts) == 3 && parts[2] != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[2])...)
	}
}

func (r *ExperimentViewOverrideResource) mapResponse(api *experimentViewOverrideAPI, data *ExperimentViewOverrideResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(api.ID)
	if api.DatasetID != "" {
		data.DatasetID = types.StringValue(api.DatasetID)
	}
	elems := make([]attr.Value, 0, len(api.ColumnOverrides))
	for _, co := range api.ColumnOverrides {
		var disableColors, hide attr.Value = types.BoolNull(), types.BoolNull()
		if co.DisableColors != nil {
			disableColors = types.BoolValue(*co.DisableColors)
		}
		if co.Hide != nil {
			hide = types.BoolValue(*co.Hide)
		}
		var precision attr.Value = types.Int64Null()
		if co.Precision != nil {
			precision = types.Int64Value(*co.Precision)
		}
		ov, d := types.ObjectValue(experimentViewColumnOverrideObjectType.AttrTypes, map[string]attr.Value{
			"column":         types.StringValue(co.Column),
			"color_gradient": jsonStringValue(co.ColorGradient),
			"color_map":      jsonStringValue(co.ColorMap),
			"disable_colors": disableColors,
			"hide":           hide,
			"precision":      precision,
		})
		diags.Append(d...)
		elems = append(elems, ov)
	}
	list, d := types.ListValue(experimentViewColumnOverrideObjectType, elems)
	diags.Append(d...)
	data.ColumnOverrides = list
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.ModifiedAt = types.StringValue(api.ModifiedAt)
	reconcileWorkspaceID(&data.WorkspaceID, "", diags)
}

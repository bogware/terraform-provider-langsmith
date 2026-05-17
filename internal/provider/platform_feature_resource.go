// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

const platformFeaturesAPIPath = "/v1/platform/features"

var (
	_ resource.Resource                = &PlatformFeatureResource{}
	_ resource.ResourceWithImportState = &PlatformFeatureResource{}
)

// NewPlatformFeatureResource returns a resource for per-feature default and
// disabled model lists (LangSmith platform features API).
func NewPlatformFeatureResource() resource.Resource {
	return &PlatformFeatureResource{}
}

// PlatformFeatureResource manages GET/PUT/DELETE on
// `/v1/platform/features/{feature}/default-model` and disabled-model routes.
type PlatformFeatureResource struct {
	client *client.Client
}

// PlatformFeatureResourceModel is Terraform state for one feature gate.
type PlatformFeatureResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Feature        types.String `tfsdk:"feature"`
	DefaultModel   types.String `tfsdk:"default_model"`
	DisabledModels types.List   `tfsdk:"disabled_models"`
}

type platformFeatureSnapshot struct {
	Feature        string   `json:"feature"`
	DefaultModel   *string  `json:"default_model"`
	DisabledModels []string `json:"disabled_models"`
}

type upsertDefaultModelBody struct {
	Model string `json:"model"`
}

type disableModelBody struct {
	Model string `json:"model"`
}

func (r *PlatformFeatureResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_feature"
}

func (r *PlatformFeatureResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages LangSmith **platform feature** model gates for a single feature name: the default model (`PUT`/`DELETE /v1/platform/features/{feature}/default-model`) " +
			"and the disabled-model list (`PUT /v1/platform/features/{feature}/disabled-models`, `DELETE .../disabled-models/{model}`). " +
			"Use `GET /v1/platform/features` via the `langsmith_platform_features` data source to discover feature keys and current settings. " +
			"Omit `default_model` and `disabled_models` in configuration to leave the API values unchanged on apply (the provider still refreshes them into state after read). " +
			"Set `disabled_models = []` to clear every disabled model the API reports for this feature. " +
			"Org-scoped API keys should set `organization_id` on the provider (or `LANGSMITH_ORGANIZATION_ID`) so requests include `X-Organization-Id` when required.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The feature name (same as `feature`), used as the stable resource address.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"feature": schema.StringAttribute{
				MarkdownDescription: "Platform feature key (path segment under `/v1/platform/features/{feature}/...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"default_model": schema.StringAttribute{
				MarkdownDescription: "Default model name for this feature. Omit on first apply to leave the API default unchanged; set to `null` to remove the default model.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"disabled_models": schema.ListAttribute{
				MarkdownDescription: "Sorted list of model names disabled for this feature. Omit on first apply to leave the API list unchanged; set to `[]` to remove all disabled models returned by the API.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *PlatformFeatureResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PlatformFeatureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PlatformFeatureResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.applyPlan(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.refreshFromAPI(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created platform feature resource", map[string]any{"feature": plan.Feature.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PlatformFeatureResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PlatformFeatureResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.refreshFromAPI(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlatformFeatureResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PlatformFeatureResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.applyPlan(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.refreshFromAPI(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "updated platform feature resource", map[string]any{"feature": plan.Feature.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PlatformFeatureResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PlatformFeatureResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	feature := data.Feature.ValueString()
	snap, err := r.readSnapshot(ctx, feature)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading platform feature before delete", err.Error())
		return
	}
	if snap.DefaultModel != nil && *snap.DefaultModel != "" {
		if err := r.client.Delete(ctx, platformFeatureDefaultModelPath(feature)); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error deleting default model", err.Error())
			return
		}
	}
	for _, m := range snap.DisabledModels {
		p := platformFeatureDisabledModelPath(feature, m)
		if err := r.client.Delete(ctx, p); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error removing disabled model "+m, err.Error())
			return
		}
	}
	tflog.Trace(ctx, "deleted platform feature resource", map[string]any{"feature": feature})
	resp.State.RemoveResource(ctx)
}

func (r *PlatformFeatureResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Feature name is required.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("feature"), types.StringValue(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
}

func platformFeatureDefaultModelPath(feature string) string {
	return "/v1/platform/features/" + url.PathEscape(feature) + "/default-model"
}

func platformFeatureDisabledModelsCollectionPath(feature string) string {
	return "/v1/platform/features/" + url.PathEscape(feature) + "/disabled-models"
}

func platformFeatureDisabledModelPath(feature, model string) string {
	return "/v1/platform/features/" + url.PathEscape(feature) + "/disabled-models/" + url.PathEscape(model)
}

func (r *PlatformFeatureResource) readSnapshot(ctx context.Context, feature string) (*platformFeatureSnapshot, error) {
	var rows []platformFeatureSnapshot
	if err := r.client.Get(ctx, platformFeaturesAPIPath, nil, &rows); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].Feature == feature {
			out := rows[i]
			return &out, nil
		}
	}
	return &platformFeatureSnapshot{Feature: feature}, nil
}

func (r *PlatformFeatureResource) applyPlan(ctx context.Context, data *PlatformFeatureResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	feature := data.Feature.ValueString()
	snap, err := r.readSnapshot(ctx, feature)
	if err != nil {
		diags.AddError("Error reading platform feature", err.Error())
		return diags
	}

	wantDefault := resolveDesiredDefault(data.DefaultModel, snap.DefaultModel)
	if err := r.syncDefaultModel(ctx, feature, wantDefault, snap.DefaultModel); err != nil {
		diags.AddError("Error syncing default model", err.Error())
		return diags
	}

	var wantDisabled []string
	if data.DisabledModels.IsUnknown() || data.DisabledModels.IsNull() {
		wantDisabled = sortedStrings(snap.DisabledModels)
	} else {
		var elems []string
		diags.Append(data.DisabledModels.ElementsAs(ctx, &elems, false)...)
		if diags.HasError() {
			return diags
		}
		sort.Strings(elems)
		wantDisabled = dedupeSortedStrings(elems)
	}

	if err := r.syncDisabledModels(ctx, feature, wantDisabled, snap.DisabledModels); err != nil {
		diags.AddError("Error syncing disabled models", err.Error())
		return diags
	}
	return diags
}

func resolveDesiredDefault(plan types.String, current *string) *string {
	if plan.IsUnknown() {
		return cloneStringPtr(current)
	}
	if plan.IsNull() {
		return nil
	}
	v := plan.ValueString()
	return &v
}

func dedupeSortedStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	prev := ""
	for _, s := range in {
		if s == prev {
			continue
		}
		out = append(out, s)
		prev = s
	}
	return out
}

func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

func ptrStringEq(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	case *a == *b:
		return true
	default:
		return false
	}
}

func (r *PlatformFeatureResource) syncDefaultModel(ctx context.Context, feature string, want, current *string) error {
	if ptrStringEq(want, current) {
		return nil
	}
	if want == nil || (want != nil && *want == "") {
		if current == nil || *current == "" {
			return nil
		}
		err := r.client.Delete(ctx, platformFeatureDefaultModelPath(feature))
		if err != nil && !client.IsNotFound(err) {
			return fmt.Errorf("delete default model: %w", err)
		}
		return nil
	}
	body := upsertDefaultModelBody{Model: *want}
	if err := r.client.Put(ctx, platformFeatureDefaultModelPath(feature), body, nil); err != nil {
		return fmt.Errorf("put default model: %w", err)
	}
	return nil
}

func (r *PlatformFeatureResource) syncDisabledModels(ctx context.Context, feature string, want, current []string) error {
	curSet := stringSet(current)
	wantSet := stringSet(want)
	for m := range curSet {
		if _, keep := wantSet[m]; !keep {
			p := platformFeatureDisabledModelPath(feature, m)
			if err := r.client.Delete(ctx, p); err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("delete disabled model %q: %w", m, err)
			}
		}
	}
	for m := range wantSet {
		if _, exists := curSet[m]; !exists {
			body := disableModelBody{Model: m}
			if err := r.client.Put(ctx, platformFeatureDisabledModelsCollectionPath(feature), body, nil); err != nil {
				return fmt.Errorf("disable model %q: %w", m, err)
			}
		}
	}
	return nil
}

func stringSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		if s != "" {
			m[s] = struct{}{}
		}
	}
	return m
}

func (r *PlatformFeatureResource) refreshFromAPI(ctx context.Context, data *PlatformFeatureResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	feature := data.Feature.ValueString()
	snap, err := r.readSnapshot(ctx, feature)
	if err != nil {
		diags.AddError("Error reading platform feature", err.Error())
		return diags
	}
	data.ID = types.StringValue(feature)
	if snap.DefaultModel != nil && *snap.DefaultModel != "" {
		data.DefaultModel = types.StringValue(*snap.DefaultModel)
	} else {
		data.DefaultModel = types.StringNull()
	}
	listVals, ldiags := types.ListValue(types.StringType, stringSliceToAttrValues(sortedStrings(snap.DisabledModels)))
	diags.Append(ldiags...)
	if diags.HasError() {
		return diags
	}
	data.DisabledModels = listVals
	return diags
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return dedupeSortedStrings(out)
}

func stringSliceToAttrValues(ss []string) []attr.Value {
	out := make([]attr.Value, len(ss))
	for i, s := range ss {
		out[i] = types.StringValue(s)
	}
	return out
}

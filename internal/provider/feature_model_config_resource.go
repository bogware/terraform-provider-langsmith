// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
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
	_ resource.Resource                = &FeatureModelConfigResource{}
	_ resource.ResourceWithImportState = &FeatureModelConfigResource{}
)

func NewFeatureModelConfigResource() resource.Resource {
	return &FeatureModelConfigResource{}
}

type FeatureModelConfigResource struct {
	client *client.Client
}

type FeatureModelConfigResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Feature        types.String `tfsdk:"feature"`
	DefaultModel   types.String `tfsdk:"default_model"`
	DisabledModels types.Set    `tfsdk:"disabled_models"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
}

type featureModelConfigAPI struct {
	Feature        string   `json:"feature"`
	DefaultModel   string   `json:"default_model"`
	DisabledModels []string `json:"disabled_models"`
}

type featureModelRequest struct {
	Model string `json:"model"`
}

func (r *FeatureModelConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feature_model_config"
}

func (r *FeatureModelConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the model configuration for a LangSmith platform feature: the default model and the set of disabled models. One resource per feature. Deleting the resource clears the default model and re-enables every model it had disabled. Import ID format: `<feature>` or `<feature>:<workspace_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"feature": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Platform feature name the configuration applies to. Cannot be changed after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"default_model": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Default model for the feature. Removing the attribute clears the default.",
			},
			"disabled_models": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Models disabled for the feature. Models removed from the set are re-enabled.",
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

func (r *FeatureModelConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func featureModelConfigBasePath(feature string) string {
	return "/v1/platform/features/" + url.PathEscape(feature)
}

// stringSetElements converts a types.Set of strings to a Go slice. Null and
// unknown sets yield nil.
func stringSetElements(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(set.ElementsAs(ctx, &out, false)...)
	return out
}

func (r *FeatureModelConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FeatureModelConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c := effectiveClient(r.client, data.WorkspaceID)
	feature := data.Feature.ValueString()

	if !data.DefaultModel.IsNull() && !data.DefaultModel.IsUnknown() {
		body := featureModelRequest{Model: data.DefaultModel.ValueString()}
		if err := c.Put(ctx, featureModelConfigBasePath(feature)+"/default-model", body, nil); err != nil {
			resp.Diagnostics.AddError("Error setting feature default model", err.Error())
			return
		}
	}
	for _, model := range stringSetElements(ctx, data.DisabledModels, &resp.Diagnostics) {
		if err := c.Put(ctx, featureModelConfigBasePath(feature)+"/disabled-models", featureModelRequest{Model: model}, nil); err != nil {
			resp.Diagnostics.AddError("Error disabling model for feature", err.Error())
			// Persist partial state so the configuration applied so far is
			// tracked (and tainted) instead of orphaned.
			r.persistPartialCreate(ctx, &data, resp)
			return
		}
	}
	if resp.Diagnostics.HasError() {
		// The default model may already have been set above; persist partial
		// state so it is tracked (and tainted) instead of orphaned.
		r.persistPartialCreate(ctx, &data, resp)
		return
	}

	data.ID = types.StringValue(feature)
	r.refresh(ctx, c, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		// The remote configuration was fully applied; persist partial state so
		// it is tracked (and tainted) instead of orphaned when the post-create
		// refresh fails.
		r.persistPartialCreate(ctx, &data, resp)
		return
	}
	tflog.Trace(ctx, "created feature model config", map[string]interface{}{"feature": feature})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// persistPartialCreate saves partial state on a create error path so whatever
// configuration was already applied remotely is tracked (and tainted) instead
// of orphaned. Every potentially-unknown field is resolved to a known value
// first, since Terraform rejects unknown values in state.
func (r *FeatureModelConfigResource) persistPartialCreate(ctx context.Context, data *FeatureModelConfigResourceModel, resp *resource.CreateResponse) {
	data.ID = types.StringValue(data.Feature.ValueString())
	if data.DefaultModel.IsUnknown() {
		data.DefaultModel = types.StringNull()
	}
	if data.DisabledModels.IsUnknown() {
		data.DisabledModels = types.SetNull(types.StringType)
	}
	reconcileWorkspaceID(&data.WorkspaceID, "", &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *FeatureModelConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FeatureModelConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.refresh(ctx, effectiveClient(r.client, data.WorkspaceID), &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FeatureModelConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state FeatureModelConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c := effectiveClient(r.client, plan.WorkspaceID)
	feature := plan.Feature.ValueString()

	// Reconcile default model.
	if !plan.DefaultModel.IsNull() && !plan.DefaultModel.IsUnknown() {
		if state.DefaultModel.IsNull() || state.DefaultModel.ValueString() != plan.DefaultModel.ValueString() {
			body := featureModelRequest{Model: plan.DefaultModel.ValueString()}
			if err := c.Put(ctx, featureModelConfigBasePath(feature)+"/default-model", body, nil); err != nil {
				resp.Diagnostics.AddError("Error setting feature default model", err.Error())
				return
			}
		}
	} else if !state.DefaultModel.IsNull() {
		if err := c.Delete(ctx, featureModelConfigBasePath(feature)+"/default-model"); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error clearing feature default model", err.Error())
			return
		}
	}

	// Reconcile disabled models.
	desired := stringSetElements(ctx, plan.DisabledModels, &resp.Diagnostics)
	current := stringSetElements(ctx, state.DisabledModels, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	desiredSet := make(map[string]bool, len(desired))
	for _, m := range desired {
		desiredSet[m] = true
	}
	currentSet := make(map[string]bool, len(current))
	for _, m := range current {
		currentSet[m] = true
	}
	for _, m := range desired {
		if !currentSet[m] {
			if err := c.Put(ctx, featureModelConfigBasePath(feature)+"/disabled-models", featureModelRequest{Model: m}, nil); err != nil {
				resp.Diagnostics.AddError("Error disabling model for feature", err.Error())
				return
			}
		}
	}
	for _, m := range current {
		if !desiredSet[m] {
			if err := c.Delete(ctx, featureModelConfigBasePath(feature)+"/disabled-models/"+url.PathEscape(m)); err != nil && !client.IsNotFound(err) {
				resp.Diagnostics.AddError("Error re-enabling model for feature", err.Error())
				return
			}
		}
	}

	plan.ID = types.StringValue(feature)
	r.refresh(ctx, c, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FeatureModelConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FeatureModelConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c := effectiveClient(r.client, data.WorkspaceID)
	feature := data.Feature.ValueString()

	if !data.DefaultModel.IsNull() {
		if err := c.Delete(ctx, featureModelConfigBasePath(feature)+"/default-model"); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error clearing feature default model", err.Error())
			return
		}
	}
	for _, m := range stringSetElements(ctx, data.DisabledModels, &resp.Diagnostics) {
		if err := c.Delete(ctx, featureModelConfigBasePath(feature)+"/disabled-models/"+url.PathEscape(m)); err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error re-enabling model for feature", err.Error())
			return
		}
	}
}

func (r *FeatureModelConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if parts[0] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<feature>\" or \"<feature>:<workspace_id>\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("feature"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	if len(parts) == 2 && parts[1] != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[1])...)
	}
}

// refresh fetches the feature configuration list and maps the entry matching
// data.Feature into the model. A missing entry means the feature has no
// configuration, which maps to a null default_model and no disabled models.
func (r *FeatureModelConfigResource) refresh(ctx context.Context, c *client.Client, data *FeatureModelConfigResourceModel, diags *diag.Diagnostics) {
	var list []featureModelConfigAPI
	if err := c.Get(ctx, "/v1/platform/features", nil, &list); err != nil {
		diags.AddError("Error reading feature model configurations", err.Error())
		return
	}
	api := featureModelConfigAPI{Feature: data.Feature.ValueString()}
	for _, entry := range list {
		if entry.Feature == data.Feature.ValueString() {
			api = entry
			break
		}
	}
	r.mapResponse(ctx, &api, data, diags)
}

func (r *FeatureModelConfigResource) mapResponse(ctx context.Context, api *featureModelConfigAPI, data *FeatureModelConfigResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(api.Feature)
	data.Feature = types.StringValue(api.Feature)
	if api.DefaultModel != "" {
		data.DefaultModel = types.StringValue(api.DefaultModel)
	} else {
		data.DefaultModel = types.StringNull()
	}
	if len(api.DisabledModels) > 0 {
		sv, d := types.SetValueFrom(ctx, types.StringType, api.DisabledModels)
		diags.Append(d...)
		data.DisabledModels = sv
	} else if data.DisabledModels.IsNull() || data.DisabledModels.IsUnknown() || len(data.DisabledModels.Elements()) > 0 {
		// No disabled models remotely: null unless the user configured an
		// explicit empty set, which is semantically identical.
		data.DisabledModels = types.SetNull(types.StringType)
	}
	reconcileWorkspaceID(&data.WorkspaceID, "", diags)
}

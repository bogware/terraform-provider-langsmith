// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

var (
	_ resource.Resource                = &ComparativeExperimentResource{}
	_ resource.ResourceWithImportState = &ComparativeExperimentResource{}
)

// NewComparativeExperimentResource returns a resource that ties a set of
// experiments against a shared reference dataset into a durable comparison.
func NewComparativeExperimentResource() resource.Resource {
	return &ComparativeExperimentResource{}
}

// ComparativeExperimentResource manages a LangSmith comparative experiment.
type ComparativeExperimentResource struct {
	client *client.Client
}

// ComparativeExperimentResourceModel maps the Terraform schema for a
// comparative experiment.
type ComparativeExperimentResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ReferenceDatasetID types.String `tfsdk:"reference_dataset_id"`
	ExperimentIDs      types.List   `tfsdk:"experiment_ids"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Extra              types.String `tfsdk:"extra"`
	CreatedAt          types.String `tfsdk:"created_at"`
	ModifiedAt         types.String `tfsdk:"modified_at"`
	WorkspaceID        types.String `tfsdk:"workspace_id"`
}

// comparativeExperimentCreateRequest is sent to POST /api/v1/datasets/comparative.
type comparativeExperimentCreateRequest struct {
	ExperimentIDs      []string        `json:"experiment_ids"`
	ReferenceDatasetID string          `json:"reference_dataset_id,omitempty"`
	Name               *string         `json:"name,omitempty"`
	Description        *string         `json:"description,omitempty"`
	Extra              json.RawMessage `json:"extra,omitempty"`
}

// comparativeExperimentAPIResponse decodes both the ComparativeExperimentBase
// returned by the create endpoint and the ComparativeExperiment items returned
// by the per-dataset read endpoint.
type comparativeExperimentAPIResponse struct {
	ID                 string          `json:"id"`
	Name               *string         `json:"name"`
	Description        *string         `json:"description"`
	WorkspaceID        string          `json:"workspace_id"`
	TenantID           string          `json:"tenant_id"`
	CreatedAt          string          `json:"created_at"`
	ModifiedAt         string          `json:"modified_at"`
	ReferenceDatasetID string          `json:"reference_dataset_id"`
	Extra              json.RawMessage `json:"extra"`
}

func (r *ComparativeExperimentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_comparative_experiment"
}

func (r *ComparativeExperimentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ties a set of experiments against a shared reference dataset into a durable comparative experiment. " +
			"The LangSmith API exposes only create and delete for comparative experiments, so every configurable field forces replacement when changed.\n\n" +
			"Comparative experiments are read back through their reference dataset, so import requires the composite ID " +
			"`<reference_dataset_id>/<comparative_experiment_id>` (for example " +
			"`terraform import langsmith_comparative_experiment.example 11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the comparative experiment.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"reference_dataset_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the dataset the compared experiments share as a reference. Required to read the comparative experiment back, " +
					"which is why it forms the first half of the import ID.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"experiment_ids": schema.ListAttribute{
				MarkdownDescription: "UUIDs of the experiments (sessions) to include in the comparison.",
				Required:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			// name, description and extra are not updatable through the API
			// (there is no PATCH/PUT for comparative experiments), so they must
			// force replacement — otherwise a change to any of them produces an
			// in-place plan that hard-fails in Update with no clean recovery.
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the comparative experiment. Changing this forces a new resource to be created.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description of the comparative experiment. Changing this forces a new resource to be created.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"extra": schema.StringAttribute{
				MarkdownDescription: "Arbitrary JSON-encoded metadata stored alongside the comparative experiment. Changing this forces a new resource to be created.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
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
				MarkdownDescription: "The workspace ID of the resource. If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
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

func (r *ComparativeExperimentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ComparativeExperimentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ComparativeExperimentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var experimentIDs []string
	resp.Diagnostics.Append(data.ExperimentIDs.ElementsAs(ctx, &experimentIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := comparativeExperimentCreateRequest{
		ExperimentIDs:      experimentIDs,
		ReferenceDatasetID: data.ReferenceDatasetID.ValueString(),
	}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !data.Extra.IsNull() && !data.Extra.IsUnknown() && data.Extra.ValueString() != "" {
		if !json.Valid([]byte(data.Extra.ValueString())) {
			resp.Diagnostics.AddError("Invalid extra JSON", "The extra attribute must contain valid JSON.")
			return
		}
		body.Extra = json.RawMessage(data.Extra.ValueString())
	}

	apiClient := effectiveClient(r.client, data.WorkspaceID)
	var created comparativeExperimentAPIResponse
	if err := apiClient.Post(ctx, "/api/v1/datasets/comparative", body, &created); err != nil {
		resp.Diagnostics.AddError("Error creating comparative experiment", err.Error())
		return
	}

	r.mapResponseToModel(&created, &data, apiClient, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created comparative experiment", map[string]interface{}{"id": created.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ComparativeExperimentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComparativeExperimentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no single-GET endpoint for a comparative experiment: the only way
	// to read one back is to list every comparative experiment under its
	// reference dataset and linear-search by ID. Hence reference_dataset_id is
	// part of the import ID — without it this URL collapses to
	// "/api/v1/datasets//comparative".
	//
	// NOTE: GET /api/v1/datasets/{dataset_id}/comparative is absent from the
	// published LangSmith OpenAPI spec, but it is real and returns 200 (verified
	// against the live API). Do not "correct" this path to a spec-derived one.
	apiClient := effectiveClient(r.client, data.WorkspaceID)
	var experiments []comparativeExperimentAPIResponse
	if err := apiClient.Get(ctx, "/api/v1/datasets/"+data.ReferenceDatasetID.ValueString()+"/comparative", nil, &experiments); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading comparative experiments", err.Error())
		return
	}

	wanted := data.ID.ValueString()
	for i := range experiments {
		if experiments[i].ID == wanted {
			r.mapResponseToModel(&experiments[i], &data, apiClient, &resp.Diagnostics)
			if resp.Diagnostics.HasError() {
				return
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	// The comparative experiment no longer exists under this dataset.
	resp.State.RemoveResource(ctx)
}

func (r *ComparativeExperimentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The LangSmith API has no update endpoint for comparative experiments, so
	// every configurable attribute (reference_dataset_id, experiment_ids, name,
	// description, extra, workspace_id) carries RequiresReplace. That makes this
	// method unreachable in practice; it exists only to satisfy the
	// resource.Resource interface. If it ever fires, a plan modifier is missing.
	resp.Diagnostics.AddError(
		"Update not supported",
		"Comparative experiments cannot be updated in place. All changes force replacement.",
	)
}

func (r *ComparativeExperimentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComparativeExperimentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/datasets/comparative/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting comparative experiment", err.Error())
		return
	}
}

// ImportState accepts the composite ID "<reference_dataset_id>/<comparative_experiment_id>".
// A bare comparative experiment ID is not enough: Read lists the comparative
// experiments of a reference dataset and searches for the ID, so without
// reference_dataset_id in state the read URL degrades to
// "/api/v1/datasets//comparative" and the import always fails.
func (r *ComparativeExperimentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format \"<reference_dataset_id>/<comparative_experiment_id>\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("reference_dataset_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// mapResponseToModel copies API response fields into the Terraform model. It
// preserves the configured experiment_ids in state because the API does not
// echo them back.
func (r *ComparativeExperimentResource) mapResponseToModel(api *comparativeExperimentAPIResponse, data *ComparativeExperimentResourceModel, c *client.Client, diags *diag.Diagnostics) {
	data.ID = types.StringValue(api.ID)
	if api.ReferenceDatasetID != "" {
		data.ReferenceDatasetID = types.StringValue(api.ReferenceDatasetID)
	}
	if api.Name != nil {
		data.Name = types.StringValue(*api.Name)
	} else {
		data.Name = types.StringNull()
	}
	if api.Description != nil {
		data.Description = types.StringValue(*api.Description)
	} else {
		data.Description = types.StringNull()
	}
	// Preserve the user's configured extra JSON; only adopt the API value when
	// the user did not set one. The API may re-serialize extra (key reordering /
	// whitespace), which would otherwise trip "inconsistent result after apply"
	// against the planned config value and cause refresh drift.
	if data.Extra.IsNull() || data.Extra.IsUnknown() {
		data.Extra = jsonStringValue(api.Extra)
	}
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.ModifiedAt = types.StringValue(api.ModifiedAt)

	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), diags)
}

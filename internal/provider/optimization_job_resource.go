// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = &OptimizationJobResource{}
	_ resource.ResourceWithImportState = &OptimizationJobResource{}
)

// NewOptimizationJobResource manages a prompt optimization job on a hub repo.
func NewOptimizationJobResource() resource.Resource {
	return &OptimizationJobResource{}
}

// OptimizationJobResource manages
// /api/v1/repos/{owner}/{repo}/optimization-jobs.
type OptimizationJobResource struct {
	client *client.Client
}

// OptimizationJobResourceModel is the Terraform state for an optimization job.
type OptimizationJobResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Owner       types.String `tfsdk:"owner"`
	Repo        types.String `tfsdk:"repo"`
	Algorithm   types.String `tfsdk:"algorithm"`
	Config      types.String `tfsdk:"config"`
	Status      types.String `tfsdk:"status"`
	Results     types.String `tfsdk:"results"`
	RepoID      types.String `tfsdk:"repo_id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type optimizationJobAPI struct {
	ID          string          `json:"id"`
	RepoID      string          `json:"repo_id"`
	Status      string          `json:"status"`
	Algorithm   string          `json:"algorithm"`
	Config      json.RawMessage `json:"config"`
	Results     json.RawMessage `json:"results"`
	WorkspaceID string          `json:"workspace_id"`
	TenantID    string          `json:"tenant_id"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type optimizationJobCreateRequest struct {
	Algorithm string          `json:"algorithm"`
	Config    json.RawMessage `json:"config"`
}

type optimizationJobUpdateRequest struct {
	Status *string `json:"status,omitempty"`
}

func (r *OptimizationJobResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_optimization_job"
}

func (r *OptimizationJobResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Runs a prompt optimization job against a LangSmith Hub repo.\n\n" +
			"A job is a unit of work rather than a steady-state object: the server advances `status` from `created` through `running` to `successful` or `failed` on its own, so that attribute will drift between plans and Terraform will show it changing. " +
			"`algorithm` and `config` define the work and cannot be changed once submitted — editing either forces a new job, which is the honest representation of re-running an optimization.\n\n" +
			"Read the job's log stream with the `langsmith_optimization_job_logs` data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "UUID of the job.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the hub repo (`-` for the current workspace).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"repo": schema.StringAttribute{
				MarkdownDescription: "Handle of the hub repo to optimize.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"algorithm": schema.StringAttribute{
				MarkdownDescription: "Optimization algorithm: `promptim` or `demo`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("promptim", "demo"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded configuration for the algorithm.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Server-driven lifecycle status: `created`, `running`, `successful` or `failed`. Set it explicitly only to cancel or force a state; otherwise it tracks whatever the server reports.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("created", "running", "successful", "failed"),
				},
			},
			"results": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded results, populated as the job progresses.",
				Computed:            true,
			},
			"repo_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the repo the job runs against.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for this resource's API calls. Changing it forces a new job.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the job was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "When the job was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *OptimizationJobResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OptimizationJobResource) basePath(data *OptimizationJobResourceModel) string {
	return fmt.Sprintf("/api/v1/repos/%s/%s/optimization-jobs", data.Owner.ValueString(), data.Repo.ValueString())
}

func (r *OptimizationJobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OptimizationJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := optimizationJobCreateRequest{
		Algorithm: data.Algorithm.ValueString(),
		Config:    json.RawMessage(data.Config.ValueString()),
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	planConfig := data.Config

	var api optimizationJobAPI
	if err := c.Post(ctx, r.basePath(&data), body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating optimization job", err.Error())
		return
	}

	r.mapResponse(&data, &api, planConfig)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), &resp.Diagnostics)

	tflog.Trace(ctx, "created optimization job", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OptimizationJobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OptimizationJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)

	var api optimizationJobAPI
	if err := c.Get(ctx, r.basePath(&data)+"/"+data.ID.ValueString(), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading optimization job", err.Error())
		return
	}

	r.mapResponse(&data, &api, data.Config)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OptimizationJobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OptimizationJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// status is the only thing the update endpoint accepts; everything else on
	// this resource forces replacement.
	body := optimizationJobUpdateRequest{}
	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		v := data.Status.ValueString()
		body.Status = &v
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	planConfig := data.Config

	var api optimizationJobAPI
	if err := c.Patch(ctx, r.basePath(&data)+"/"+data.ID.ValueString(), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating optimization job", err.Error())
		return
	}

	r.mapResponse(&data, &api, planConfig)
	finalizeWorkspaceID(&data.WorkspaceID, c, firstNonEmpty(api.WorkspaceID, api.TenantID), &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OptimizationJobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OptimizationJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	if err := c.Delete(ctx, r.basePath(&data)+"/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting optimization job", err.Error())
	}
}

// ImportState parses "<owner>/<repo>/<job_id>": Read needs the repo coordinates
// as well as the job ID, so a bare ID cannot address the job.
func (r *OptimizationJobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"<owner>/<repo>/<job_id>\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("owner"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

// mapResponse copies the API job onto state. planConfig is preserved because the
// server echoes a normalised copy of the submitted config.
func (r *OptimizationJobResource) mapResponse(data *OptimizationJobResourceModel, api *optimizationJobAPI, planConfig types.String) {
	data.ID = types.StringValue(api.ID)
	data.RepoID = types.StringValue(api.RepoID)
	data.Status = types.StringValue(api.Status)
	data.Algorithm = types.StringValue(api.Algorithm)
	data.Results = jsonEmptyArrayIsNull(api.Results)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
	data.Config = jsonPreserveConfigSubset(api.Config, planConfig)
}

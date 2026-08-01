// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &IssuesAgentResource{}
	_ resource.ResourceWithImportState = &IssuesAgentResource{}
)

func NewIssuesAgentResource() resource.Resource {
	return &IssuesAgentResource{}
}

type IssuesAgentResource struct {
	client *client.Client
}

type IssuesAgentResourceModel struct {
	ID                          types.String `tfsdk:"id"`
	SessionID                   types.String `tfsdk:"session_id"`
	GithubRepoURL               types.String `tfsdk:"github_repo_url"`
	GithubBaseBranch            types.String `tfsdk:"github_base_branch"`
	GithubRepoSubdir            types.String `tfsdk:"github_repo_subdir"`
	ContextHubRepoHandle        types.String `tfsdk:"context_hub_repo_handle"`
	Priorities                  types.List   `tfsdk:"priorities"`
	CronEnabled                 types.Bool   `tfsdk:"cron_enabled"`
	AgentOverviewAccepted       types.Bool   `tfsdk:"agent_overview_accepted"`
	Overview                    types.String `tfsdk:"overview"`
	UserInstructions            types.String `tfsdk:"user_instructions"`
	SessionLCUSpendLimitMonthly types.String `tfsdk:"session_lcu_spend_limit_monthly"`
	SessionAgentOverviewRepoID  types.String `tfsdk:"session_agent_overview_repo_id"`
	CronSchedule                types.String `tfsdk:"cron_schedule"`
	LatestRunID                 types.String `tfsdk:"latest_run_id"`
	LatestThreadID              types.String `tfsdk:"latest_thread_id"`
	SessionName                 types.String `tfsdk:"session_name"`
	TenantName                  types.String `tfsdk:"tenant_name"`
	CreatedAt                   types.String `tfsdk:"created_at"`
	UpdatedAt                   types.String `tfsdk:"updated_at"`
	WorkspaceID                 types.String `tfsdk:"workspace_id"`
}

type issuesAgentCreateRequest struct {
	ContextHubRepoHandle *string  `json:"context_hub_repo_handle,omitempty"`
	GithubBaseBranch     *string  `json:"github_base_branch,omitempty"`
	GithubRepoSubdir     *string  `json:"github_repo_subdir,omitempty"`
	GithubRepoURL        *string  `json:"github_repo_url,omitempty"`
	Priorities           []string `json:"priorities,omitempty"`
}

type issuesAgentPatchRequest struct {
	AgentOverviewAccepted       *bool    `json:"agent_overview_accepted,omitempty"`
	ContextHubRepoHandle        *string  `json:"context_hub_repo_handle,omitempty"`
	CronEnabled                 *bool    `json:"cron_enabled,omitempty"`
	GithubBaseBranch            *string  `json:"github_base_branch,omitempty"`
	GithubRepoSubdir            *string  `json:"github_repo_subdir,omitempty"`
	GithubRepoURL               *string  `json:"github_repo_url,omitempty"`
	Priorities                  []string `json:"priorities,omitempty"`
	SessionLCUSpendLimitMonthly *string  `json:"session_lcu_spend_limit_monthly,omitempty"`
	UserInstructions            *string  `json:"user_instructions,omitempty"`
}

// issuesAgentSaveOverviewRequest is the body of
// PATCH /v1/platform/sessions/{session_id}/issues-agent/overview.
type issuesAgentSaveOverviewRequest struct {
	Content string `json:"content"`
}

// issuesAgentSaveOverviewResponse is what the overview endpoint returns: the
// commit written to the backing Prompt Hub repo plus that repo's ID. The
// content itself is never returned by any endpoint.
type issuesAgentSaveOverviewResponse struct {
	CommitHash                 *string `json:"commit_hash"`
	SessionAgentOverviewRepoID *string `json:"session_agent_overview_repo_id"`
}

func (b *issuesAgentPatchRequest) isEmpty() bool {
	return b.AgentOverviewAccepted == nil && b.ContextHubRepoHandle == nil && b.CronEnabled == nil &&
		b.GithubBaseBranch == nil && b.GithubRepoSubdir == nil && b.GithubRepoURL == nil &&
		b.Priorities == nil && b.SessionLCUSpendLimitMonthly == nil && b.UserInstructions == nil
}

type issuesAgentAPI struct {
	AgentOverviewAccepted       *bool    `json:"agent_overview_accepted"`
	ContextHubRepoHandle        *string  `json:"context_hub_repo_handle"`
	CreatedAt                   *string  `json:"created_at"`
	CronEnabled                 *bool    `json:"cron_enabled"`
	CronSchedule                *string  `json:"cron_schedule"`
	GithubBaseBranch            *string  `json:"github_base_branch"`
	GithubRepoSubdir            *string  `json:"github_repo_subdir"`
	GithubRepoURL               *string  `json:"github_repo_url"`
	ID                          *string  `json:"id"`
	LatestRunID                 *string  `json:"latest_run_id"`
	LatestThreadID              *string  `json:"latest_thread_id"`
	Priorities                  []string `json:"priorities"`
	SessionAgentOverviewRepoID  *string  `json:"session_agent_overview_repo_id"`
	SessionID                   *string  `json:"session_id"`
	SessionLCUSpendLimitMonthly *string  `json:"session_lcu_spend_limit_monthly"`
	SessionName                 *string  `json:"session_name"`
	TenantID                    *string  `json:"tenant_id"`
	TenantName                  *string  `json:"tenant_name"`
	UpdatedAt                   *string  `json:"updated_at"`
	UserInstructions            *string  `json:"user_instructions"`
}

func (r *IssuesAgentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_issues_agent"
}

func (r *IssuesAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "**Beta:** Manages the LangSmith Issues Agent configuration attached to a tracing project (session). Creating the resource configures the agent and enqueues its initial scan; destroying it removes the agent config, its issues, and the agent-overview hub repo. The underlying API is in active development and may change without notice. At most one issues agent can exist per session.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** UUID of the issues agent config.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"session_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "**Beta:** UUID of the tracing project (tracer session) the agent watches. Changing this forces a new agent.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"github_repo_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "**Beta:** URL of the GitHub repository the agent proposes fixes against. Changing it clears fix-related fields server-side.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"github_base_branch": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "**Beta:** Base branch fix PRs are opened against.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"github_repo_subdir": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "**Beta:** Subdirectory of the repository containing the agent's code.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"context_hub_repo_handle": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "**Beta:** Handle of the LangSmith Hub repo providing extra context to the agent.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"priorities": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "**Beta:** Ordered list of issue priorities the agent focuses on.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"cron_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "**Beta:** Whether the recurring (cron) scan is enabled. Applied via a follow-up PATCH after create, since the create endpoint does not accept it.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"agent_overview_accepted": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "**Beta:** Whether the generated Agent Overview has been accepted. Applied via a follow-up PATCH after create.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"overview": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "**Beta:** Content of the Agent Overview document. Saved through the dedicated `PATCH /v1/platform/sessions/{session_id}/issues-agent/overview` endpoint (neither create nor update accepts it), which creates or updates the private Prompt Hub repo backing the overview and links it to the agent config — see `session_agent_overview_repo_id`. **Write-only:** no API endpoint returns the overview content, so Terraform cannot detect drift on it. The value is preserved from state on refresh, is not populated on import, and is re-sent only when the configured content changes. Removing the attribute leaves the last saved overview in place server-side (the API exposes no way to delete it), and the agent itself may rewrite the document on its next scan.",
			},
			"user_instructions": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "**Beta:** Free-form user preferences the agent treats as authoritative context. Removing the attribute clears the instructions (an empty string is sent to the API). Applied via a follow-up PATCH after create.",
			},
			"session_lcu_spend_limit_monthly": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "**Beta:** Per-project monthly Engine LCU spend cap, serialized as a decimal string to preserve precision (e.g. `\"100\"`). `0` blocks all new runs. Removing the attribute clears the cap (a negative value is sent to the API). Applied via a follow-up PATCH after create.",
			},
			"session_agent_overview_repo_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** UUID of the server-managed agent-overview hub repo.",
			},
			"cron_schedule": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** Server-assigned cron schedule for recurring scans.",
			},
			"latest_run_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** ID of the latest agent run on LangSmith Deployments; null until the first trigger.",
			},
			"latest_thread_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** Thread ID of the latest agent run; null until the first trigger.",
			},
			"session_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** Name of the tracing project, resolved server-side.",
			},
			"tenant_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** Workspace (tenant) display name, resolved server-side.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** Creation timestamp.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "**Beta:** Last update timestamp.",
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "**Beta:** If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
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

func (r *IssuesAgentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IssuesAgentResource) basePath(sessionID string) string {
	return "/api/v1/platform/sessions/" + sessionID + "/issues-agent"
}

func (r *IssuesAgentResource) overviewPath(sessionID string) string {
	return r.basePath(sessionID) + "/overview"
}

// saveOverview pushes the Agent Overview content through the dedicated overview
// endpoint (the create/update endpoints do not accept it) and backfills the
// server-managed overview repo ID onto api, since saving the overview is what
// creates that repo — the create response predates it.
func (r *IssuesAgentResource) saveOverview(ctx context.Context, c *client.Client, sessionID, content string, api *issuesAgentAPI) error {
	var out issuesAgentSaveOverviewResponse
	if err := c.Patch(ctx, r.overviewPath(sessionID), issuesAgentSaveOverviewRequest{Content: content}, &out); err != nil {
		return err
	}
	if out.SessionAgentOverviewRepoID != nil && *out.SessionAgentOverviewRepoID != "" {
		api.SessionAgentOverviewRepoID = out.SessionAgentOverviewRepoID
	}
	fields := map[string]interface{}{"session_id": sessionID}
	if out.CommitHash != nil {
		fields["commit_hash"] = *out.CommitHash
	}
	tflog.Trace(ctx, "saved issues agent overview", fields)
	return nil
}

func issuesAgentStringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func issuesAgentBoolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func issuesAgentPriorities(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(list.ElementsAs(ctx, &out, false)...)
	return out
}

func (r *IssuesAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IssuesAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := issuesAgentCreateRequest{
		ContextHubRepoHandle: issuesAgentStringPtr(data.ContextHubRepoHandle),
		GithubBaseBranch:     issuesAgentStringPtr(data.GithubBaseBranch),
		GithubRepoSubdir:     issuesAgentStringPtr(data.GithubRepoSubdir),
		GithubRepoURL:        issuesAgentStringPtr(data.GithubRepoURL),
		Priorities:           issuesAgentPriorities(ctx, data.Priorities, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var api issuesAgentAPI
	if err := c.Post(ctx, r.basePath(data.SessionID.ValueString()), body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating issues agent", err.Error())
		return
	}

	// The create endpoint does not accept cron / overview / instructions /
	// spend-limit settings, so apply them with a follow-up PATCH when set.
	patch := issuesAgentPatchRequest{
		AgentOverviewAccepted:       issuesAgentBoolPtr(data.AgentOverviewAccepted),
		CronEnabled:                 issuesAgentBoolPtr(data.CronEnabled),
		SessionLCUSpendLimitMonthly: issuesAgentStringPtr(data.SessionLCUSpendLimitMonthly),
		UserInstructions:            issuesAgentStringPtr(data.UserInstructions),
	}
	if !patch.isEmpty() {
		if err := c.Patch(ctx, r.basePath(data.SessionID.ValueString()), patch, &api); err != nil {
			resp.Diagnostics.AddError(
				"Error applying issues agent settings after create",
				"The agent was created but the follow-up PATCH for cron/overview/instructions/spend-limit settings failed: "+err.Error(),
			)
			// Persist partial state so the created agent is tracked (and
			// tainted) instead of orphaned when the follow-up PATCH fails.
			// mapResponse resolves the still-unknown computed fields from the
			// create response (api still holds the POST result).
			r.mapResponse(&api, &data, &resp.Diagnostics)
			// This PATCH failed before the overview was ever attempted, so the
			// overview does not exist server-side. Do not record it as if it did.
			data.Overview = types.StringNull()
			r.resolveUnknowns(&data)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// The overview has its own endpoint; save it once the agent exists.
	if !data.Overview.IsNull() && !data.Overview.IsUnknown() {
		if err := r.saveOverview(ctx, c, data.SessionID.ValueString(), data.Overview.ValueString(), &api); err != nil {
			resp.Diagnostics.AddError(
				"Error saving issues agent overview after create",
				"The agent was created but the follow-up PATCH that saves the agent overview failed: "+err.Error(),
			)
			// Persist partial state so the created agent is tracked (and
			// tainted) instead of orphaned.
			r.mapResponse(&api, &data, &resp.Diagnostics)
			// The overview save is what failed, so it was not persisted remotely.
			data.Overview = types.StringNull()
			r.resolveUnknowns(&data)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		// Persist partial state so the created agent is tracked (and tainted)
		// instead of orphaned when response mapping fails.
		r.resolveUnknowns(&data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	tflog.Trace(ctx, "created issues agent", map[string]interface{}{"session_id": data.SessionID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IssuesAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IssuesAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api issuesAgentAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, r.basePath(data.SessionID.ValueString()), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading issues agent", err.Error())
		return
	}
	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IssuesAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IssuesAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	var state IssuesAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := issuesAgentPatchRequest{
		AgentOverviewAccepted: issuesAgentBoolPtr(data.AgentOverviewAccepted),
		ContextHubRepoHandle:  issuesAgentStringPtr(data.ContextHubRepoHandle),
		CronEnabled:           issuesAgentBoolPtr(data.CronEnabled),
		GithubBaseBranch:      issuesAgentStringPtr(data.GithubBaseBranch),
		GithubRepoSubdir:      issuesAgentStringPtr(data.GithubRepoSubdir),
		GithubRepoURL:         issuesAgentStringPtr(data.GithubRepoURL),
		Priorities:            issuesAgentPriorities(ctx, data.Priorities, &resp.Diagnostics),
		UserInstructions:      issuesAgentStringPtr(data.UserInstructions),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	// user_instructions: the API clears on "" (must not send null). When the
	// attribute is removed from config, send "" explicitly.
	if data.UserInstructions.IsNull() && !state.UserInstructions.IsNull() && !state.UserInstructions.IsUnknown() {
		empty := ""
		body.UserInstructions = &empty
	}
	// session_lcu_spend_limit_monthly is tri-state: absent = unchanged,
	// negative = clear. When the attribute is removed from config, send a
	// negative value to clear the cap.
	if !data.SessionLCUSpendLimitMonthly.IsNull() && !data.SessionLCUSpendLimitMonthly.IsUnknown() {
		v := data.SessionLCUSpendLimitMonthly.ValueString()
		body.SessionLCUSpendLimitMonthly = &v
	} else if !state.SessionLCUSpendLimitMonthly.IsNull() && !state.SessionLCUSpendLimitMonthly.IsUnknown() {
		clearLimit := "-1"
		body.SessionLCUSpendLimitMonthly = &clearLimit
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	var api issuesAgentAPI
	if err := c.Patch(ctx, r.basePath(data.SessionID.ValueString()), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating issues agent", err.Error())
		return
	}

	// overview lives on its own endpoint and is write-only, so push it only when
	// the configured content actually changed — otherwise every unrelated update
	// would write a redundant commit to the overview hub repo. Removing the
	// attribute leaves the last saved overview in place: the API has no delete
	// for it.
	if !data.Overview.IsNull() && !data.Overview.IsUnknown() && !data.Overview.Equal(state.Overview) {
		if err := r.saveOverview(ctx, c, data.SessionID.ValueString(), data.Overview.ValueString(), &api); err != nil {
			resp.Diagnostics.AddError("Error saving issues agent overview", err.Error())
			return
		}
	}

	r.mapResponse(&api, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IssuesAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IssuesAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, r.basePath(data.SessionID.ValueString())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting issues agent", err.Error())
		return
	}
}

func (r *IssuesAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The agent is keyed by its session, so import by session_id (the tracing
	// project UUID); Read fills in everything else.
	resource.ImportStatePassthroughID(ctx, path.Root("session_id"), req, resp)
}

// issuesAgentSetString maps an optional+computed string attribute: a non-empty
// API value wins; otherwise an unknown value collapses to null and a known
// (configured) value is preserved.
func issuesAgentSetString(dst *types.String, api *string) {
	if api != nil && *api != "" {
		*dst = types.StringValue(*api)
		return
	}
	if dst.IsUnknown() {
		*dst = types.StringNull()
	}
}

// issuesAgentComputedString maps a computed-only nullable string: empty or
// absent values become null.
func issuesAgentComputedString(api *string) types.String {
	if api != nil && *api != "" {
		return types.StringValue(*api)
	}
	return types.StringNull()
}

func issuesAgentSetBool(dst *types.Bool, api *bool) {
	if api != nil {
		*dst = types.BoolValue(*api)
		return
	}
	if dst.IsUnknown() {
		*dst = types.BoolValue(false)
	}
}

// resolveUnknowns nulls every model field that is still unknown, so partial
// state persisted on a create error path never contains unknown values
// (Terraform rejects unknown values in state).
func (r *IssuesAgentResource) resolveUnknowns(data *IssuesAgentResourceModel) {
	for _, s := range []*types.String{
		&data.ID, &data.GithubRepoURL, &data.GithubBaseBranch, &data.GithubRepoSubdir,
		&data.ContextHubRepoHandle, &data.Overview, &data.UserInstructions,
		&data.SessionLCUSpendLimitMonthly,
		&data.SessionAgentOverviewRepoID, &data.CronSchedule, &data.LatestRunID,
		&data.LatestThreadID, &data.SessionName, &data.TenantName,
		&data.CreatedAt, &data.UpdatedAt, &data.WorkspaceID,
	} {
		if s.IsUnknown() {
			*s = types.StringNull()
		}
	}
	for _, b := range []*types.Bool{&data.CronEnabled, &data.AgentOverviewAccepted} {
		if b.IsUnknown() {
			*b = types.BoolNull()
		}
	}
	if data.Priorities.IsUnknown() {
		data.Priorities = types.ListNull(types.StringType)
	}
}

func (r *IssuesAgentResource) mapResponse(api *issuesAgentAPI, data *IssuesAgentResourceModel, diags *diag.Diagnostics) {
	if api.ID != nil && *api.ID != "" {
		data.ID = types.StringValue(*api.ID)
	}
	// session_id is required config; keep the configured value.
	//
	// overview is deliberately untouched: the API never returns the overview
	// content (the GET exposes only session_agent_overview_repo_id), so it is
	// write-only. Leaving data.Overview alone preserves the value already held
	// by the plan (Create/Update) or by prior state (Read) instead of nulling it
	// out on every refresh, which would produce a permanent phantom diff.

	issuesAgentSetString(&data.GithubRepoURL, api.GithubRepoURL)
	issuesAgentSetString(&data.GithubBaseBranch, api.GithubBaseBranch)
	issuesAgentSetString(&data.GithubRepoSubdir, api.GithubRepoSubdir)
	issuesAgentSetString(&data.ContextHubRepoHandle, api.ContextHubRepoHandle)
	issuesAgentSetBool(&data.CronEnabled, api.CronEnabled)
	issuesAgentSetBool(&data.AgentOverviewAccepted, api.AgentOverviewAccepted)

	if len(api.Priorities) > 0 {
		elems := make([]attr.Value, 0, len(api.Priorities))
		for _, p := range api.Priorities {
			elems = append(elems, types.StringValue(p))
		}
		list, d := types.ListValue(types.StringType, elems)
		diags.Append(d...)
		data.Priorities = list
	} else if data.Priorities.IsUnknown() {
		data.Priorities = types.ListNull(types.StringType)
	}

	// user_instructions is user-owned: a non-empty API value wins, otherwise
	// preserve the configured value (null or explicit "").
	if api.UserInstructions != nil && *api.UserInstructions != "" {
		data.UserInstructions = types.StringValue(*api.UserInstructions)
	} else if data.UserInstructions.IsUnknown() {
		data.UserInstructions = types.StringNull()
	}

	// The spend limit is NUMERIC(28,6) serialized as a string; the server may
	// re-render the configured number (e.g. "100" -> "100.000000"). Keep the
	// configured representation when the values are numerically equal.
	if api.SessionLCUSpendLimitMonthly != nil && *api.SessionLCUSpendLimitMonthly != "" {
		apiVal := *api.SessionLCUSpendLimitMonthly
		if !data.SessionLCUSpendLimitMonthly.IsNull() && !data.SessionLCUSpendLimitMonthly.IsUnknown() {
			cur, errCur := strconv.ParseFloat(data.SessionLCUSpendLimitMonthly.ValueString(), 64)
			srv, errSrv := strconv.ParseFloat(apiVal, 64)
			if errCur == nil && errSrv == nil && cur == srv {
				// keep the configured representation
			} else {
				data.SessionLCUSpendLimitMonthly = types.StringValue(apiVal)
			}
		} else {
			data.SessionLCUSpendLimitMonthly = types.StringValue(apiVal)
		}
	} else {
		data.SessionLCUSpendLimitMonthly = types.StringNull()
	}

	data.SessionAgentOverviewRepoID = issuesAgentComputedString(api.SessionAgentOverviewRepoID)
	data.CronSchedule = issuesAgentComputedString(api.CronSchedule)
	data.LatestRunID = issuesAgentComputedString(api.LatestRunID)
	data.LatestThreadID = issuesAgentComputedString(api.LatestThreadID)
	data.SessionName = issuesAgentComputedString(api.SessionName)
	data.TenantName = issuesAgentComputedString(api.TenantName)
	data.CreatedAt = issuesAgentComputedString(api.CreatedAt)
	data.UpdatedAt = issuesAgentComputedString(api.UpdatedAt)

	apiTenant := ""
	if api.TenantID != nil {
		apiTenant = *api.TenantID
	}
	reconcileWorkspaceID(&data.WorkspaceID, apiTenant, diags)
}

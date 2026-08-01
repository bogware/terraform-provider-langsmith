// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	_ resource.Resource                = &HubDirectoryResource{}
	_ resource.ResourceWithImportState = &HubDirectoryResource{}
)

// NewHubDirectoryResource manages the file contents of a directory-style hub
// repository.
func NewHubDirectoryResource() resource.Resource {
	return &HubDirectoryResource{}
}

// HubDirectoryResource manages
// /api/v1/platform/hub/repos/{owner}/{repo}/directories.
type HubDirectoryResource struct {
	client *client.Client
}

// HubDirectoryResourceModel is the Terraform state for a hub directory.
type HubDirectoryResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Owner        types.String `tfsdk:"owner"`
	Repo         types.String `tfsdk:"repo"`
	Files        types.String `tfsdk:"files"`
	SkipWebhooks types.Bool   `tfsdk:"skip_webhooks"`
	CommitHash   types.String `tfsdk:"commit_hash"`
	CommitID     types.String `tfsdk:"commit_id"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
}

type hubDirectoryAPIResponse struct {
	CommitHash string          `json:"commit_hash"`
	CommitID   string          `json:"commit_id"`
	Files      json.RawMessage `json:"files"`
}

type hubDirectoryCommitRequest struct {
	Files        json.RawMessage `json:"files"`
	ParentCommit string          `json:"parent_commit,omitempty"`
	SkipWebhooks bool            `json:"skip_webhooks"`
}

type hubDirectoryCommitResponse struct {
	Commit hubDirectoryCommit `json:"commit"`
}

type hubDirectoryCommit struct {
	CommitHash string `json:"commit_hash"`
	ID         string `json:"id"`
}

func (r *HubDirectoryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_directory"
}

func (r *HubDirectoryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the file contents of a directory-style LangSmith Hub repository. Every apply that changes `files` writes a new commit, so the repo's history records each change Terraform made.\n\n" +
			"`owner` and `repo` identify the repository and cannot be changed in place. Destroying this resource deletes the directory repository.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the directory, in the form `<owner>/<repo>`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the hub repo (`-` for the current workspace).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"repo": schema.StringAttribute{
				MarkdownDescription: "Handle of the hub repo.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"files": schema.StringAttribute{
				MarkdownDescription: "JSON-encoded object mapping file paths to their contents. This is the whole directory: a path omitted here is not present after the commit.",
				Required:            true,
			},
			"skip_webhooks": schema.BoolAttribute{
				// Optional-only so an imported directory, whose state has no value
				// for this, does not plan a change and write a pointless commit.
				MarkdownDescription: "Suppress webhook delivery for commits written by Terraform. Defaults to `false` when omitted.",
				Optional:            true,
			},
			"commit_hash": schema.StringAttribute{
				MarkdownDescription: "Hash of the most recent commit.",
				Computed:            true,
			},
			"commit_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the most recent commit.",
				Computed:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for this resource's API calls. Changing it forces replacement.",
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

func (r *HubDirectoryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *HubDirectoryResource) basePath(data *HubDirectoryResourceModel) string {
	return fmt.Sprintf("/api/v1/platform/hub/repos/%s/%s/directories",
		data.Owner.ValueString(), data.Repo.ValueString())
}

// commit writes the configured files. parentCommit is empty for the first
// commit and carries the known head otherwise, so the server can reject a write
// that raced with a change made outside Terraform.
func (r *HubDirectoryResource) commit(ctx context.Context, c *client.Client, data *HubDirectoryResourceModel, parentCommit string) error {
	body := hubDirectoryCommitRequest{
		Files:        json.RawMessage(data.Files.ValueString()),
		ParentCommit: parentCommit,
		SkipWebhooks: data.SkipWebhooks.ValueBool(),
	}
	var result hubDirectoryCommitResponse
	if err := c.Post(ctx, r.basePath(data)+"/commits", body, &result); err != nil {
		return err
	}
	data.CommitHash = types.StringValue(result.Commit.CommitHash)
	data.CommitID = types.StringValue(result.Commit.ID)
	return nil
}

func (r *HubDirectoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HubDirectoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	if err := r.commit(ctx, c, &data, ""); err != nil {
		resp.Diagnostics.AddError("Error creating hub directory commit", err.Error())
		return
	}

	data.ID = types.StringValue(data.Owner.ValueString() + "/" + data.Repo.ValueString())
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	tflog.Trace(ctx, "created hub directory", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HubDirectoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HubDirectoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)

	var api hubDirectoryAPIResponse
	if err := c.Get(ctx, r.basePath(&data), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading hub directory", err.Error())
		return
	}

	data.ID = types.StringValue(data.Owner.ValueString() + "/" + data.Repo.ValueString())
	data.CommitHash = types.StringValue(api.CommitHash)
	data.CommitID = types.StringValue(api.CommitID)
	// The server may return the files with added metadata; keep the configured
	// form when it only adds to what was written.
	data.Files = jsonPreserveConfigSubset(api.Files, data.Files)
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HubDirectoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HubDirectoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state HubDirectoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	if err := r.commit(ctx, c, &data, state.CommitHash.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error updating hub directory", err.Error())
		return
	}

	data.ID = state.ID
	finalizeWorkspaceID(&data.WorkspaceID, c, "", &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HubDirectoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HubDirectoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := effectiveClient(r.client, data.WorkspaceID)
	if err := c.Delete(ctx, r.basePath(&data)); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting hub directory", err.Error())
	}
}

// ImportState parses "<owner>/<repo>": Read addresses the directory by repo
// coordinates, not by an opaque ID.
func (r *HubDirectoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"<owner>/<repo>\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("owner"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

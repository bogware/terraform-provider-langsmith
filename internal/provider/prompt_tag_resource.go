// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
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
	_ resource.Resource                = &PromptTagResource{}
	_ resource.ResourceWithImportState = &PromptTagResource{}
)

// NewPromptTagResource returns a resource for managing named tags on prompt
// commits -- like branding cattle for the production herd or the staging pen.
func NewPromptTagResource() resource.Resource {
	return &PromptTagResource{}
}

// PromptTagResource manages named version tags on prompt repo commits.
type PromptTagResource struct {
	client *client.Client
}

// PromptTagResourceModel maps the Terraform schema for a prompt tag.
type PromptTagResourceModel struct {
	ID         types.String `tfsdk:"id"`
	RepoHandle types.String `tfsdk:"repo_handle"`
	TagName    types.String `tfsdk:"tag_name"`
	CommitHash types.String `tfsdk:"commit_hash"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

// promptTagCreateRequest is sent to POST /api/v1/repos/-/{repo}/tags.
type promptTagCreateRequest struct {
	TagName  string `json:"tag_name"`
	CommitID string `json:"commit_id"`
}

// promptTagUpdateRequest is sent to PATCH /api/v1/repos/-/{repo}/tags/{tag_name}.
type promptTagUpdateRequest struct {
	CommitID string `json:"commit_id"`
}

// promptTagAPIResponse is the shape of a tag from the API.
type promptTagAPIResponse struct {
	ID         string `json:"id"`
	RepoID     string `json:"repo_id"`
	CommitID   string `json:"commit_id"`
	CommitHash string `json:"commit_hash"`
	TagName    string `json:"tag_name"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// promptCommitListItem is used when we need the commit UUID from a hash.
type promptCommitListItem struct {
	ID         string `json:"id"`
	CommitHash string `json:"commit_hash"`
}

// promptCommitListResponse wraps the list of commits.
type promptCommitListResponse struct {
	Commits []promptCommitListItem `json:"commits"`
	Total   int                    `json:"total"`
}

func (r *PromptTagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt_tag"
}

func (r *PromptTagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a named version tag on a LangSmith prompt repo. Tags like `production` or `staging` point to specific commits, letting you promote prompt versions through environments.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the tag.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"repo_handle": schema.StringAttribute{
				MarkdownDescription: "The handle of the prompt repo.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tag_name": schema.StringAttribute{
				MarkdownDescription: "The name of the tag (e.g., `production`, `staging`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"commit_hash": schema.StringAttribute{
				MarkdownDescription: "The commit hash that this tag points to. Update this to promote a different version.",
				Required:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the tag was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "When the tag was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *PromptTagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// resolveCommitID looks up the commit UUID from a commit hash.
func (r *PromptTagResource) resolveCommitID(ctx context.Context, repoHandle, commitHash string) (string, error) {
	var listResp promptCommitListResponse
	err := r.client.Get(ctx, fmt.Sprintf("/commits/-/%s", repoHandle), nil, &listResp)
	if err != nil {
		return "", fmt.Errorf("listing commits: %w", err)
	}

	for _, c := range listResp.Commits {
		if c.CommitHash == commitHash {
			return c.ID, nil
		}
	}

	return "", fmt.Errorf("commit hash %q not found in repo %q", commitHash, repoHandle)
}

func (r *PromptTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PromptTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	commitID, err := r.resolveCommitID(ctx, data.RepoHandle.ValueString(), data.CommitHash.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving commit hash", err.Error())
		return
	}

	body := promptTagCreateRequest{
		TagName:  data.TagName.ValueString(),
		CommitID: commitID,
	}

	var result promptTagAPIResponse
	err = r.client.Post(ctx, fmt.Sprintf("/api/v1/repos/-/%s/tags", data.RepoHandle.ValueString()), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating prompt tag", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.CommitHash = types.StringValue(result.CommitHash)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	tflog.Trace(ctx, "created prompt tag", map[string]interface{}{"id": result.ID, "tag": result.TagName})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PromptTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result promptTagAPIResponse
	err := r.client.Get(ctx, fmt.Sprintf("/api/v1/repos/-/%s/tags/%s", data.RepoHandle.ValueString(), data.TagName.ValueString()), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading prompt tag", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.CommitHash = types.StringValue(result.CommitHash)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PromptTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	commitID, err := r.resolveCommitID(ctx, data.RepoHandle.ValueString(), data.CommitHash.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving commit hash", err.Error())
		return
	}

	body := promptTagUpdateRequest{
		CommitID: commitID,
	}

	var result promptTagAPIResponse
	err = r.client.Patch(ctx, fmt.Sprintf("/api/v1/repos/-/%s/tags/%s", data.RepoHandle.ValueString(), data.TagName.ValueString()), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating prompt tag", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.CommitHash = types.StringValue(result.CommitHash)
	data.CreatedAt = types.StringValue(result.CreatedAt)
	data.UpdatedAt = types.StringValue(result.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PromptTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, fmt.Sprintf("/api/v1/repos/-/%s/tags/%s", data.RepoHandle.ValueString(), data.TagName.ValueString()))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting prompt tag", err.Error())
	}
}

func (r *PromptTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: repo_handle/tag_name
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: repo_handle/tag_name")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo_handle"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag_name"), parts[1])...)
}

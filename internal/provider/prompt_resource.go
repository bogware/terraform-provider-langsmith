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
	_ resource.Resource                = &PromptResource{}
	_ resource.ResourceWithImportState = &PromptResource{}
)

// NewPromptResource saddles up a fresh PromptResource, ready to ride.
func NewPromptResource() resource.Resource {
	return &PromptResource{}
}

// PromptResource manages prompt repos in the LangSmith Hub --
// the general store of reusable prompt templates.
type PromptResource struct {
	client *client.Client
}

// PromptResourceModel maps the Terraform schema to Go types for a prompt repo.
type PromptResourceModel struct {
	ID             types.String `tfsdk:"id"`
	RepoHandle     types.String `tfsdk:"repo_handle"`
	Manifest       types.String `tfsdk:"manifest"`
	IsPublic       types.Bool   `tfsdk:"is_public"`
	Description    types.String `tfsdk:"description"`
	Readme         types.String `tfsdk:"readme"`
	Tags           types.List   `tfsdk:"tags"`
	IsArchived     types.Bool   `tfsdk:"is_archived"`
	Owner          types.String `tfsdk:"owner"`
	FullName       types.String `tfsdk:"full_name"`
	CommitHash     types.String `tfsdk:"commit_hash"`
	TenantID       types.String `tfsdk:"tenant_id"`
	NumCommits     types.Int64  `tfsdk:"num_commits"`
	NumLikes       types.Int64  `tfsdk:"num_likes"`
	NumViews       types.Int64  `tfsdk:"num_views"`
	NumDownloads   types.Int64  `tfsdk:"num_downloads"`
	LastCommitHash types.String `tfsdk:"last_commit_hash"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

// promptCreateRequest is the payload for staking a new claim in the Hub.
type promptCreateRequest struct {
	RepoHandle  string   `json:"repo_handle"`
	IsPublic    bool     `json:"is_public"`
	Description string   `json:"description,omitempty"`
	Readme      string   `json:"readme,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// promptUpdateRequest carries the fields that can be amended after the initial filing.
type promptUpdateRequest struct {
	Description *string  `json:"description,omitempty"`
	Readme      *string  `json:"readme,omitempty"`
	IsPublic    *bool    `json:"is_public,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsArchived  *bool    `json:"is_archived,omitempty"`
}

// promptCommitRequest is the payload for branding a new version of the prompt.
type promptCommitRequest struct {
	Manifest json.RawMessage `json:"manifest"`
}

// promptCommitResponse wraps the commit the API sends back after a successful brand.
type promptCommitResponse struct {
	Commit struct {
		ID         string          `json:"id"`
		CommitHash string          `json:"commit_hash"`
		Manifest   json.RawMessage `json:"manifest"`
	} `json:"commit"`
}

// promptLatestCommitResponse is the shape of GET /commits/-/{repo}/latest.
type promptLatestCommitResponse struct {
	CommitHash string          `json:"commit_hash"`
	Manifest   json.RawMessage `json:"manifest"`
}

// promptAPIResponse is what the LangSmith API sends back when you come asking about a prompt.
// Like Miss Kitty keeping the books, every field the API knows gets tallied here.
type promptAPIResponse struct {
	Repo struct {
		ID             string   `json:"id"`
		RepoHandle     string   `json:"repo_handle"`
		Description    string   `json:"description"`
		Readme         string   `json:"readme"`
		IsPublic       bool     `json:"is_public"`
		IsArchived     bool     `json:"is_archived"`
		Tags           []string `json:"tags"`
		TenantID       string   `json:"tenant_id"`
		NumCommits     int64    `json:"num_commits"`
		NumLikes       int64    `json:"num_likes"`
		NumViews       int64    `json:"num_views"`
		NumDownloads   int64    `json:"num_downloads"`
		LastCommitHash *string  `json:"last_commit_hash"`
		CreatedAt      string   `json:"created_at"`
		UpdatedAt      string   `json:"updated_at"`
	} `json:"repo"`
	Owner    string `json:"owner"`
	FullName string `json:"full_name"`
}

func (r *PromptResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (r *PromptResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a prompt (repo) in the LangSmith Hub.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the prompt repo.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"repo_handle": schema.StringAttribute{
				MarkdownDescription: "The name/handle of the prompt repo.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"manifest": schema.StringAttribute{
				MarkdownDescription: "JSON string of the prompt manifest (LangChain serialization format). This is the actual prompt content — the template, messages, and variables. Setting this creates a new commit in the prompt repo.",
				Optional:            true,
				Computed:            true,
			},
			"is_public": schema.BoolAttribute{
				MarkdownDescription: "Whether the prompt is publicly accessible.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the prompt.",
				Optional:            true,
			},
			"readme": schema.StringAttribute{
				MarkdownDescription: "README content for the prompt.",
				Optional:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags for the prompt.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"is_archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the prompt has been archived -- put out to pasture, so to speak.",
				Optional:            true,
				Computed:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "The owner of the prompt repo.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"full_name": schema.StringAttribute{
				MarkdownDescription: "The full name of the prompt (owner/repo_handle).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"commit_hash": schema.StringAttribute{
				MarkdownDescription: "The hash of the current commit — the latest brand on the cattle.",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID that owns this prompt.",
				Computed:            true,
			},
			"num_commits": schema.Int64Attribute{
				MarkdownDescription: "The number of commits in the prompt repo.",
				Computed:            true,
			},
			"num_likes": schema.Int64Attribute{
				MarkdownDescription: "The number of likes on the prompt.",
				Computed:            true,
			},
			"num_views": schema.Int64Attribute{
				MarkdownDescription: "The number of views on the prompt.",
				Computed:            true,
			},
			"num_downloads": schema.Int64Attribute{
				MarkdownDescription: "The number of downloads of the prompt.",
				Computed:            true,
			},
			"last_commit_hash": schema.StringAttribute{
				MarkdownDescription: "The hash of the last commit -- the latest brand on the cattle.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the prompt was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "When the prompt was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *PromptResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PromptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PromptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := promptCreateRequest{
		RepoHandle: data.RepoHandle.ValueString(),
		IsPublic:   data.IsPublic.ValueBool(),
	}
	if !data.Description.IsNull() {
		body.Description = data.Description.ValueString()
	}
	if !data.Readme.IsNull() {
		body.Readme = data.Readme.ValueString()
	}
	if !data.Tags.IsNull() {
		var tags []string
		resp.Diagnostics.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Tags = tags
	}

	var result promptAPIResponse
	err := r.client.Post(ctx, "/api/v1/repos", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating prompt", err.Error())
		return
	}

	data.ID = types.StringValue(result.Repo.ID)
	data.Owner = types.StringValue(result.Owner)
	data.FullName = types.StringValue(result.FullName)
	data.CreatedAt = types.StringValue(result.Repo.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Repo.UpdatedAt)

	// If the trail boss brought a manifest, commit it to the repo right away.
	if !data.Manifest.IsNull() && !data.Manifest.IsUnknown() {
		commitBody := promptCommitRequest{
			Manifest: json.RawMessage(data.Manifest.ValueString()),
		}
		var commitResult promptCommitResponse
		err := r.client.Post(ctx, fmt.Sprintf("/commits/-/%s", data.RepoHandle.ValueString()), commitBody, &commitResult)
		if err != nil {
			resp.Diagnostics.AddError("Error creating prompt commit", err.Error())
			return
		}
		data.CommitHash = types.StringValue(commitResult.Commit.CommitHash)
		data.LastCommitHash = types.StringValue(commitResult.Commit.CommitHash)
		data.NumCommits = types.Int64Value(1)
	} else {
		data.Manifest = types.StringNull()
		data.CommitHash = types.StringNull()
		data.LastCommitHash = types.StringNull()
		data.NumCommits = types.Int64Value(0)
	}

	// Set remaining computed fields that the create response may not populate.
	data.IsArchived = types.BoolValue(result.Repo.IsArchived)
	data.TenantID = types.StringValue(result.Repo.TenantID)
	data.NumLikes = types.Int64Value(result.Repo.NumLikes)
	data.NumViews = types.Int64Value(result.Repo.NumViews)
	data.NumDownloads = types.Int64Value(result.Repo.NumDownloads)

	tflog.Trace(ctx, "created prompt resource", map[string]interface{}{"id": result.Repo.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner := data.Owner.ValueString()
	repoHandle := data.RepoHandle.ValueString()
	if data.FullName.ValueString() != "" {
		parts := strings.SplitN(data.FullName.ValueString(), "/", 2)
		if len(parts) == 2 {
			owner = parts[0]
			repoHandle = parts[1]
		}
	}

	var result promptAPIResponse
	err := r.client.Get(ctx, fmt.Sprintf("/api/v1/repos/%s/%s", owner, repoHandle), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading prompt", err.Error())
		return
	}

	data.ID = types.StringValue(result.Repo.ID)
	data.RepoHandle = types.StringValue(result.Repo.RepoHandle)
	data.IsPublic = types.BoolValue(result.Repo.IsPublic)
	data.IsArchived = types.BoolValue(result.Repo.IsArchived)
	data.Owner = types.StringValue(result.Owner)
	data.FullName = types.StringValue(result.FullName)
	data.TenantID = types.StringValue(result.Repo.TenantID)
	data.NumCommits = types.Int64Value(result.Repo.NumCommits)
	data.NumLikes = types.Int64Value(result.Repo.NumLikes)
	data.NumViews = types.Int64Value(result.Repo.NumViews)
	data.NumDownloads = types.Int64Value(result.Repo.NumDownloads)
	data.CreatedAt = types.StringValue(result.Repo.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Repo.UpdatedAt)

	// Last commit hash may be nil if nobody's ridden through yet.
	if result.Repo.LastCommitHash != nil {
		data.LastCommitHash = types.StringValue(*result.Repo.LastCommitHash)
	} else {
		data.LastCommitHash = types.StringNull()
	}

	if result.Repo.Description != "" {
		data.Description = types.StringValue(result.Repo.Description)
	} else {
		data.Description = types.StringNull()
	}
	if result.Repo.Readme != "" {
		data.Readme = types.StringValue(result.Repo.Readme)
	} else {
		data.Readme = types.StringNull()
	}
	if len(result.Repo.Tags) > 0 {
		tags, diags := types.ListValueFrom(ctx, types.StringType, result.Repo.Tags)
		resp.Diagnostics.Append(diags...)
		data.Tags = tags
	} else {
		data.Tags = types.ListNull(types.StringType)
	}

	// Ride over to the commits corral and fetch the latest manifest.
	if result.Repo.NumCommits > 0 {
		var latestCommit promptLatestCommitResponse
		commitErr := r.client.Get(ctx, fmt.Sprintf("/commits/-/%s/latest", repoHandle), nil, &latestCommit)
		if commitErr != nil {
			resp.Diagnostics.AddWarning("Error reading prompt manifest", commitErr.Error())
		} else {
			data.CommitHash = types.StringValue(latestCommit.CommitHash)
			if len(latestCommit.Manifest) > 0 && string(latestCommit.Manifest) != "null" {
				data.Manifest = types.StringValue(string(latestCommit.Manifest))
			} else {
				data.Manifest = types.StringNull()
			}
		}
	} else {
		data.CommitHash = types.StringNull()
		// Only null out manifest if user hasn't set it in config.
		if data.Manifest.IsUnknown() {
			data.Manifest = types.StringNull()
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PromptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state PromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner := state.Owner.ValueString()
	repoHandle := state.RepoHandle.ValueString()

	body := promptUpdateRequest{}
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		body.Description = &desc
	}
	if !data.Readme.IsNull() {
		readme := data.Readme.ValueString()
		body.Readme = &readme
	}
	isPublic := data.IsPublic.ValueBool()
	body.IsPublic = &isPublic
	if !data.Tags.IsNull() {
		var tags []string
		resp.Diagnostics.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Tags = tags
	}
	// A man can archive a prompt same as he can hang up his spurs.
	if !data.IsArchived.IsNull() && !data.IsArchived.IsUnknown() {
		v := data.IsArchived.ValueBool()
		body.IsArchived = &v
	}

	err := r.client.Patch(ctx, fmt.Sprintf("/api/v1/repos/%s/%s", owner, repoHandle), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating prompt", err.Error())
		return
	}

	// If the manifest has changed, commit the new version.
	if !data.Manifest.IsNull() && !data.Manifest.IsUnknown() &&
		data.Manifest.ValueString() != state.Manifest.ValueString() {
		commitBody := promptCommitRequest{
			Manifest: json.RawMessage(data.Manifest.ValueString()),
		}
		var commitResult promptCommitResponse
		commitErr := r.client.Post(ctx, fmt.Sprintf("/commits/-/%s", repoHandle), commitBody, &commitResult)
		if commitErr != nil {
			resp.Diagnostics.AddError("Error creating prompt commit", commitErr.Error())
			return
		}
		data.CommitHash = types.StringValue(commitResult.Commit.CommitHash)
		data.LastCommitHash = types.StringValue(commitResult.Commit.CommitHash)
	}

	// PATCH doesn't return the full resource, so we ride back to the API for the latest state.
	var result promptAPIResponse
	err = r.client.Get(ctx, fmt.Sprintf("/api/v1/repos/%s/%s", owner, data.RepoHandle.ValueString()), nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading prompt after update", err.Error())
		return
	}

	data.ID = types.StringValue(result.Repo.ID)
	data.Owner = types.StringValue(result.Owner)
	data.FullName = types.StringValue(result.FullName)
	data.IsArchived = types.BoolValue(result.Repo.IsArchived)
	data.TenantID = types.StringValue(result.Repo.TenantID)
	data.NumCommits = types.Int64Value(result.Repo.NumCommits)
	data.NumLikes = types.Int64Value(result.Repo.NumLikes)
	data.NumViews = types.Int64Value(result.Repo.NumViews)
	data.NumDownloads = types.Int64Value(result.Repo.NumDownloads)
	data.CreatedAt = types.StringValue(result.Repo.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Repo.UpdatedAt)

	// Even after an update, the last commit hash might still be a no-show.
	if result.Repo.LastCommitHash != nil {
		data.LastCommitHash = types.StringValue(*result.Repo.LastCommitHash)
	} else {
		data.LastCommitHash = types.StringNull()
	}

	// Fetch the latest manifest if we haven't just committed one.
	if data.CommitHash.IsNull() || data.CommitHash.IsUnknown() {
		if result.Repo.NumCommits > 0 {
			var latestCommit promptLatestCommitResponse
			commitErr := r.client.Get(ctx, fmt.Sprintf("/commits/-/%s/latest", repoHandle), nil, &latestCommit)
			if commitErr == nil {
				data.CommitHash = types.StringValue(latestCommit.CommitHash)
				if len(latestCommit.Manifest) > 0 && string(latestCommit.Manifest) != "null" {
					data.Manifest = types.StringValue(string(latestCommit.Manifest))
				}
			}
		} else {
			data.CommitHash = types.StringNull()
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner := data.Owner.ValueString()
	repoHandle := data.RepoHandle.ValueString()

	err := r.client.Delete(ctx, fmt.Sprintf("/api/v1/repos/%s/%s", owner, repoHandle))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting prompt", err.Error())
	}
}

func (r *PromptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: owner/repo_handle")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("owner"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo_handle"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("full_name"), req.ID)...)
}

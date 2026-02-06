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
	_ resource.Resource                = &PromptResource{}
	_ resource.ResourceWithImportState = &PromptResource{}
)

func NewPromptResource() resource.Resource {
	return &PromptResource{}
}

type PromptResource struct {
	client *client.Client
}

type PromptResourceModel struct {
	ID          types.String `tfsdk:"id"`
	RepoHandle  types.String `tfsdk:"repo_handle"`
	IsPublic    types.Bool   `tfsdk:"is_public"`
	Description types.String `tfsdk:"description"`
	Readme      types.String `tfsdk:"readme"`
	Tags        types.List   `tfsdk:"tags"`
	Owner       types.String `tfsdk:"owner"`
	FullName    types.String `tfsdk:"full_name"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type promptCreateRequest struct {
	RepoHandle  string   `json:"repo_handle"`
	IsPublic    bool     `json:"is_public"`
	Description string   `json:"description,omitempty"`
	Readme      string   `json:"readme,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type promptUpdateRequest struct {
	Description *string  `json:"description,omitempty"`
	Readme      *string  `json:"readme,omitempty"`
	IsPublic    *bool    `json:"is_public,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type promptAPIResponse struct {
	Repo struct {
		ID          string   `json:"id"`
		RepoHandle  string   `json:"repo_handle"`
		Description string   `json:"description"`
		Readme      string   `json:"readme"`
		IsPublic    bool     `json:"is_public"`
		Tags        []string `json:"tags"`
		CreatedAt   string   `json:"created_at"`
		UpdatedAt   string   `json:"updated_at"`
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
	data.Owner = types.StringValue(result.Owner)
	data.FullName = types.StringValue(result.FullName)
	data.CreatedAt = types.StringValue(result.Repo.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Repo.UpdatedAt)

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

	err := r.client.Patch(ctx, fmt.Sprintf("/api/v1/repos/%s/%s", owner, repoHandle), body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating prompt", err.Error())
		return
	}

	// Re-read to get updated state
	var result promptAPIResponse
	err = r.client.Get(ctx, fmt.Sprintf("/api/v1/repos/%s/%s", owner, data.RepoHandle.ValueString()), nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading prompt after update", err.Error())
		return
	}

	data.ID = types.StringValue(result.Repo.ID)
	data.Owner = types.StringValue(result.Owner)
	data.FullName = types.StringValue(result.FullName)
	data.CreatedAt = types.StringValue(result.Repo.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Repo.UpdatedAt)

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

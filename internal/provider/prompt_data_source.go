// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &PromptDataSource{}

func NewPromptDataSource() datasource.DataSource {
	return &PromptDataSource{}
}

type PromptDataSource struct {
	client *client.Client
}

type PromptDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	RepoHandle  types.String `tfsdk:"repo_handle"`
	Description types.String `tfsdk:"description"`
	Readme      types.String `tfsdk:"readme"`
	IsPublic    types.Bool   `tfsdk:"is_public"`
	IsArchived  types.Bool   `tfsdk:"is_archived"`
	TenantID    types.String `tfsdk:"tenant_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

// promptDataSourceAPIResponse mirrors what GET /api/v1/repos/-/{handle}
// actually returns — everything nested under a top-level "repo" object.
// Without this nesting the struct decoded to all-zero values (the bug fixed
// in 0.9.0).
type promptDataSourceAPIResponse struct {
	Repo struct {
		ID          string  `json:"id"`
		RepoHandle  string  `json:"repo_handle"`
		Description *string `json:"description"`
		Readme      *string `json:"readme"`
		IsPublic    bool    `json:"is_public"`
		IsArchived  bool    `json:"is_archived"`
		TenantID    string  `json:"tenant_id"`
		CreatedAt   string  `json:"created_at"`
		UpdatedAt   string  `json:"updated_at"`
	} `json:"repo"`
}

func (d *PromptDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (d *PromptDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to look up a LangSmith prompt (repo) by handle.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the prompt repo.",
				Computed:            true,
			},
			"repo_handle": schema.StringAttribute{
				MarkdownDescription: "The handle (name) of the prompt repo.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the prompt.",
				Computed:            true,
			},
			"readme": schema.StringAttribute{
				MarkdownDescription: "The readme content.",
				Computed:            true,
			},
			"is_public": schema.BoolAttribute{
				MarkdownDescription: "Whether the prompt is publicly visible.",
				Computed:            true,
			},
			"is_archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the prompt is archived.",
				Computed:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The tenant ID.",
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

func (d *PromptDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *PromptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PromptDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result promptDataSourceAPIResponse
	err := d.client.Get(ctx, "/api/v1/repos/-/"+data.RepoHandle.ValueString(), nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading prompt", err.Error())
		return
	}

	data.ID = types.StringValue(result.Repo.ID)
	data.RepoHandle = types.StringValue(result.Repo.RepoHandle)
	data.IsPublic = types.BoolValue(result.Repo.IsPublic)
	data.IsArchived = types.BoolValue(result.Repo.IsArchived)
	data.TenantID = types.StringValue(result.Repo.TenantID)
	data.CreatedAt = types.StringValue(result.Repo.CreatedAt)
	data.UpdatedAt = types.StringValue(result.Repo.UpdatedAt)

	if result.Repo.Description != nil {
		data.Description = types.StringValue(*result.Repo.Description)
	} else {
		data.Description = types.StringNull()
	}
	if result.Repo.Readme != nil {
		data.Readme = types.StringValue(*result.Repo.Readme)
	} else {
		data.Readme = types.StringNull()
	}

	tflog.Trace(ctx, "read prompt data source", map[string]interface{}{"id": result.Repo.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

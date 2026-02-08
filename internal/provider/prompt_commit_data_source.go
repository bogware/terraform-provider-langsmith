// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &PromptCommitDataSource{}

// NewPromptCommitDataSource returns a data source for reading a specific commit
// from a prompt repo -- checking the brand on a particular head of cattle.
func NewPromptCommitDataSource() datasource.DataSource {
	return &PromptCommitDataSource{}
}

// PromptCommitDataSource reads a commit from a LangSmith prompt repo by hash,
// tag name, or "latest".
type PromptCommitDataSource struct {
	client *client.Client
}

// PromptCommitDataSourceModel holds the attributes for a prompt commit lookup.
type PromptCommitDataSourceModel struct {
	RepoHandle types.String `tfsdk:"repo_handle"`
	Ref        types.String `tfsdk:"ref"`
	CommitHash types.String `tfsdk:"commit_hash"`
	Manifest   types.String `tfsdk:"manifest"`
}

// promptCommitDataSourceAPIResponse is the API shape for GET /commits/-/{repo}/{ref}.
type promptCommitDataSourceAPIResponse struct {
	CommitHash string          `json:"commit_hash"`
	Manifest   json.RawMessage `json:"manifest"`
}

func (d *PromptCommitDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt_commit"
}

func (d *PromptCommitDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to read a specific commit from a LangSmith prompt repo by hash, tag name, or `latest`.",
		Attributes: map[string]schema.Attribute{
			"repo_handle": schema.StringAttribute{
				MarkdownDescription: "The handle of the prompt repo.",
				Required:            true,
			},
			"ref": schema.StringAttribute{
				MarkdownDescription: "The commit reference: a commit hash, tag name, or `latest` (default).",
				Optional:            true,
			},
			"commit_hash": schema.StringAttribute{
				MarkdownDescription: "The full SHA hash of the resolved commit.",
				Computed:            true,
			},
			"manifest": schema.StringAttribute{
				MarkdownDescription: "JSON string of the prompt manifest (LangChain serialization format).",
				Computed:            true,
			},
		},
	}
}

func (d *PromptCommitDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *PromptCommitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PromptCommitDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ref := "latest"
	if !data.Ref.IsNull() && !data.Ref.IsUnknown() && data.Ref.ValueString() != "" {
		ref = data.Ref.ValueString()
	}

	var result promptCommitDataSourceAPIResponse
	err := d.client.Get(ctx, fmt.Sprintf("/commits/-/%s/%s", data.RepoHandle.ValueString(), ref), nil, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error reading prompt commit", err.Error())
		return
	}

	data.CommitHash = types.StringValue(result.CommitHash)

	if len(result.Manifest) > 0 && string(result.Manifest) != "null" {
		data.Manifest = types.StringValue(string(result.Manifest))
	} else {
		data.Manifest = types.StringNull()
	}

	tflog.Trace(ctx, "read prompt commit data source", map[string]interface{}{
		"repo_handle": data.RepoHandle.ValueString(),
		"ref":         ref,
		"commit_hash": result.CommitHash,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

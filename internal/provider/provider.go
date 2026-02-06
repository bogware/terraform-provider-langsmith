// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ provider.Provider = &LangSmithProvider{}

// LangSmithProvider defines the provider implementation. This is the marshal's
// office — where all resources and data sources report for duty.
type LangSmithProvider struct {
	version string
}

// LangSmithProviderModel describes the provider configuration: API key, base
// URL, and tenant ID. The credentials every lawman carries on the frontier.
type LangSmithProviderModel struct {
	APIKey   types.String `tfsdk:"api_key"`
	APIURL   types.String `tfsdk:"api_url"`
	TenantID types.String `tfsdk:"tenant_id"`
}

func (p *LangSmithProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "langsmith"
	resp.Version = p.version
}

func (p *LangSmithProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The LangSmith provider allows you to manage LangSmith resources such as projects, datasets, annotation queues, prompts, and more.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The LangSmith API key. Can also be set with the `LANGSMITH_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "The LangSmith API base URL. Defaults to `https://api.smith.langchain.com`. Can also be set with the `LANGSMITH_API_URL` environment variable.",
				Optional:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The LangSmith workspace/tenant ID. Required for org-scoped API keys. Can also be set with the `LANGSMITH_TENANT_ID` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *LangSmithProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data LangSmithProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("LANGSMITH_API_KEY")
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The LangSmith API key must be set in the provider configuration or via the LANGSMITH_API_KEY environment variable.",
		)
		return
	}

	apiURL := "https://api.smith.langchain.com"
	if envURL := os.Getenv("LANGSMITH_API_URL"); envURL != "" {
		apiURL = envURL
	}
	if !data.APIURL.IsNull() {
		apiURL = data.APIURL.ValueString()
	}

	tenantID := os.Getenv("LANGSMITH_TENANT_ID")
	if !data.TenantID.IsNull() {
		tenantID = data.TenantID.ValueString()
	}

	c := client.NewClient(apiURL, apiKey, tenantID)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *LangSmithProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewDatasetResource,
		NewExampleResource,
		NewAnnotationQueueResource,
		NewServiceAccountResource,
		NewServiceKeyResource,
		NewPromptResource,
		NewRunRuleResource,
		NewWebhookResource,
		NewFeedbackConfigResource,
		NewWorkspaceResource,
		NewTagKeyResource,
		NewTagValueResource,
		NewBulkExportDestinationResource,
		NewBulkExportResource,
		NewModelPriceMapResource,
		NewUsageLimitResource,
		NewPlaygroundSettingsResource,
	}
}

func (p *LangSmithProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewDatasetDataSource,
		NewWorkspaceDataSource,
		NewInfoDataSource,
		NewOrganizationDataSource,
	}
}

// New returns a provider factory function, ready to pin on the badge and start
// serving Terraform requests.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LangSmithProvider{
			version: version,
		}
	}
}

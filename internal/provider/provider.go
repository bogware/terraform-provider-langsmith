// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ provider.Provider = &LangSmithProvider{}

// LangSmithProvider defines the provider implementation.
type LangSmithProvider struct {
	version string
}

// LangSmithProviderModel describes the provider configuration.
type LangSmithProviderModel struct {
	APIKey         types.String `tfsdk:"api_key"`
	APIURL         types.String `tfsdk:"api_url"`
	TenantID       types.String `tfsdk:"tenant_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
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
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The LangSmith organization ID sent as `X-Organization-Id` on API requests. Required for organization-scoped resources such as `langsmith_org_chart` and `langsmith_org_chart_section`. Can also be set with the `LANGSMITH_ORGANIZATION_ID` environment variable.",
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

	apiKey := strings.TrimSpace(os.Getenv("LANGSMITH_API_KEY"))
	if !data.APIKey.IsNull() {
		apiKey = strings.TrimSpace(data.APIKey.ValueString())
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

	organizationID := os.Getenv("LANGSMITH_ORGANIZATION_ID")
	if !data.OrganizationID.IsNull() {
		organizationID = strings.TrimSpace(data.OrganizationID.ValueString())
	}

	userAgent := fmt.Sprintf("terraform-provider-langsmith/%s", p.version)

	c := client.NewClient(apiURL, apiKey, tenantID, organizationID, userAgent)

	// Validate the API key by making a lightweight request.
	var info struct {
		Version string `json:"version"`
	}
	if err := c.Get(ctx, "/api/v1/info", nil, &info); err != nil {
		resp.Diagnostics.AddError(
			"Unable to connect to LangSmith API",
			fmt.Sprintf("Could not validate API credentials against %s: %s", apiURL, err),
		)
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *LangSmithProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewDatasetResource,
		NewExampleResource,
		NewAnnotationQueueResource,
		NewAnnotationQueueReviewerResource,
		NewServiceAccountResource,
		NewServiceKeyResource,
		NewAPIKeyResource,
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
		NewSecretResource,
		NewSettingsResource,
		NewTTLSettingsResource,
		NewAlertRuleResource,
		NewOrgRoleResource,
		NewSSOSettingsResource,
		NewWorkspaceMemberResource,
		NewPromptTagResource,
		NewOrgMemberResource,
		NewFilterViewResource,
		NewTaggingResource,
		NewFeedbackFormulaResource,
		NewChartSectionResource,
		NewChartResource,
		NewOrgChartSectionResource,
		NewOrgChartResource,
		NewAccessPolicyResource,
		NewGatewayPolicyResource,
		NewSCIMTokenResource,
		NewFeedbackIngestTokenResource,
		NewPlatformFeatureResource,
		NewEvaluatorResource,
		NewFleetMCPServerResource,
	}
}

func (p *LangSmithProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewProjectAgentVersionsDataSource,
		NewDatasetDataSource,
		NewWorkspaceDataSource,
		NewTenantsDataSource,
		NewInfoDataSource,
		NewOrganizationDataSource,
		NewOrganizationPendingInvitesDataSource,
		NewOrganizationPermissionsDataSource,
		NewOrganizationsDataSource,
		NewPromptCommitDataSource,
		NewPromptDataSource,
		NewAnnotationQueueDataSource,
		NewOrgRoleDataSource,
		NewRunRuleDataSource,
		NewTagKeyDataSource,
		NewServiceAccountDataSource,
		NewSettingsDataSource,
		NewUserDataSource,
		NewToolDataSource,
		NewSSOSettingsBySlugDataSource,
		NewFeedbackIngestTokensDataSource,
		NewAuditLogsDataSource,
		NewPlatformFeaturesDataSource,
		NewEvaluatorDataSource,
	}
}

// New returns a provider factory function.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LangSmithProvider{
			version: version,
		}
	}
}

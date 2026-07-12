// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
	APIKey      types.String `tfsdk:"api_key"`
	APIURL      types.String `tfsdk:"api_url"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
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
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The LangSmith workspace ID. Required for org-scoped API keys. Can also be set with the `LANGSMITH_WORKSPACE_ID` environment variable.\n\n" +
					"To manage several workspaces from one configuration, prefer the per-resource `workspace_id` attribute over a provider alias: a provider block cannot consume a value that is unknown at plan time (such as the ID of a workspace created in the same apply), whereas a resource can.",
				Optional: true,
			},
		},
	}
}

func getApiKey(data LangSmithProviderModel, resp *provider.ConfigureResponse) string {
	apiKey := strings.TrimSpace(os.Getenv("LANGSMITH_API_KEY"))
	if !data.APIKey.IsNull() {
		apiKey = strings.TrimSpace(data.APIKey.ValueString())
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The LangSmith API key must be set in the provider configuration or via the LANGSMITH_API_KEY environment variable.",
		)
	}
	return apiKey
}

func getApiUrl(data LangSmithProviderModel) string {
	apiURL := "https://api.smith.langchain.com"
	if envURL := os.Getenv("LANGSMITH_API_URL"); envURL != "" {
		apiURL = envURL
	}
	if !data.APIURL.IsNull() {
		apiURL = data.APIURL.ValueString()
	}
	apiURL = strings.TrimRight(apiURL, "/")
	return apiURL
}

func getWorkspaceId(data LangSmithProviderModel) string {
	workspaceID := os.Getenv("LANGSMITH_WORKSPACE_ID")
	if !data.WorkspaceID.IsNull() {
		workspaceID = data.WorkspaceID.ValueString()
	}
	return workspaceID
}

// checkUnknownConfig rejects provider configuration values that are not known at
// plan time. A provider is configured before the resources it depends on are
// applied, so a value such as `api_key = langsmith_api_key.ci.key` or
// `workspace_id = langsmith_workspace.prod.id` is still unknown here. Without
// this check an unknown value silently degrades to the empty string, which
// either drops the X-Tenant-Id header (mis-scoping every call to the default
// workspace) or surfaces as a misleading "Missing API Key" error.
func checkUnknownConfig(data LangSmithProviderModel, resp *provider.ConfigureResponse) {
	const remedy = "Either set the value statically, supply it via the environment variable, or apply the resource that produces it first (for example with -target). " +
		"To manage a workspace created in the same apply, set the per-resource `workspace_id` attribute instead of configuring a provider alias."

	if data.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown LangSmith API key",
			"The provider cannot be configured because `api_key` is not known at plan time. "+remedy,
		)
	}
	if data.APIURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Unknown LangSmith API URL",
			"The provider cannot be configured because `api_url` is not known at plan time. "+remedy,
		)
	}
	if data.WorkspaceID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspace_id"),
			"Unknown LangSmith workspace ID",
			"The provider cannot be configured because `workspace_id` is not known at plan time. "+remedy,
		)
	}
}

func (p *LangSmithProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data LangSmithProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Reject unknown values before they degrade to "" further down.
	checkUnknownConfig(data, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := getApiKey(data, resp)
	apiURL := getApiUrl(data)
	workspaceId := getWorkspaceId(data)
	if apiURL == "" || apiKey == "" {
		return
	}

	// The API key travels in the X-API-Key header. Over plain http it is sent in
	// cleartext, so warn rather than fail (self-hosted instances behind a trusted
	// network may legitimately use http, and forbidding it outright would break them).
	if strings.HasPrefix(strings.ToLower(apiURL), "http://") {
		resp.Diagnostics.AddAttributeWarning(
			path.Root("api_url"),
			"Insecure API URL",
			"api_url uses http://, so the API key is transmitted in cleartext. Use https:// unless this is a self-hosted instance on a trusted network.",
		)
	}

	userAgent := fmt.Sprintf("terraform-provider-langsmith/%s", p.version)

	c := client.NewClient(apiURL, apiKey, workspaceId, userAgent)

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
		NewSecretResource,
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
		NewChartSectionCloneResource,
		NewAccessPolicyResource,
		NewSCIMTokenResource,
		NewEvaluatorResource,
		NewGatewayPolicyResource,
		NewToolResource,
		NewHubEnvironmentResource,
		NewPersonalAccessTokenResource,
		NewFeedbackIngestTokenResource,
		NewDatasetShareResource,
		NewDatasetSplitResource,
		NewAnnotationQueueReviewerResource,
		NewRepoOwnerResource,
		NewInsightsConfigResource,
		NewExperimentViewOverrideResource,
		NewFeatureModelConfigResource,
		NewMCPVendorSettingsResource,
		NewAgentBuilderIntegrationsResource,
		NewIssuesAgentResource,
		NewDataPlaneResource,
		NewDatasetVersionTagResource,
		NewRunShareResource,
		NewWorkspaceHandleResource,
		NewOrganizationSettingsResource,
		// API parity additions (OpenAPI gap analysis)
		NewAPIKeyResource,
		NewWorkspaceTTLSettingsResource,
		NewComparativeExperimentResource,
		NewRoleAccessPoliciesResource,

		// Platform completeness (1.0)
		NewSandboxRegistryResource,
	}
}

func (p *LangSmithProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewDatasetDataSource,
		NewWorkspaceDataSource,
		NewInfoDataSource,
		NewOrganizationDataSource,
		NewPromptCommitDataSource,
		NewPromptDataSource,
		NewAnnotationQueueDataSource,
		NewOrgRoleDataSource,
		NewRunRuleDataSource,
		NewTagKeyDataSource,
		NewServiceAccountDataSource,
		NewUserDataSource,
		NewChartDataSource,
		NewChartSectionDataSource,
		NewOrgChartDataSource,
		NewOrgChartSectionDataSource,
		NewChartPreviewDataSource,
		NewOrgChartPreviewDataSource,
		NewEvaluatorDataSource,
		NewToolDataSource,
		NewGatewayPolicyDataSource,
		NewMCPVendorDataSource,
		NewAuditLogDataSource,
		NewDataPlanesDataSource,
		NewWorkspacesDataSource,
		NewPermissionsDataSource,
		NewProjectsDataSource,
		NewDatasetsDataSource,
		NewPromptsDataSource,
		NewEvaluatorsDataSource,
		NewRunRulesDataSource,
		NewPlaygroundSettingsDataSource,
		NewWorkspaceMembersDataSource,
		NewOrgMembersDataSource,
		NewUsageLimitsDataSource,
		NewSecretNamesDataSource,
		NewExampleDataSource,
		NewFeedbackConfigDataSource,
		NewFilterViewDataSource,
		NewSSOSettingsDataSource,
		NewHubEnvironmentsDataSource,
		NewBulkExportDataSource,
		NewTagValueDataSource,
		NewWorkspaceStatsDataSource,
		NewOrgUsageDataSource,
		NewEvaluatorSpendDataSource,
		NewIssuesDataSource,
		// API parity additions (OpenAPI gap analysis)
		NewBulkExportDestinationDataSource,
		NewToolsDataSource,
		NewMCPVendorsDataSource,
		NewRepoOwnersDataSource,
		NewPromptRepoTagsDataSource,
		NewRunRuleLogsDataSource,

		// Platform completeness (1.0): discovery data sources
		NewAccessPoliciesDataSource,
		NewBulkExportDestinationsDataSource,
		NewBulkExportsDataSource,
		NewExamplesDataSource,
		NewFeedbackFormulasDataSource,
		NewGatewayPoliciesDataSource,
		NewRepoTagsDataSource,
		NewSandboxRegistriesDataSource,
		NewWorkspaceTagsDataSource,
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

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
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
	APIKey        types.String `tfsdk:"api_key"`
	APIURL        types.String `tfsdk:"api_url"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	SelfHosted    types.Bool   `tfsdk:"self_hosted"`
	PathOverrides types.Map    `tfsdk:"path_overrides"`
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
			"self_hosted": schema.BoolAttribute{
				MarkdownDescription: "Set to `true` when `api_url` points at a self-hosted LangSmith instance. Can also be set with the `LANGSMITH_SELF_HOSTED` environment variable. Defaults to `false` (LangSmith Cloud).\n\n" +
					"Self-hosted deployments serve the API under a `/api` path prefix, whereas Cloud serves it at the root of the `api.` subdomain. The provider already uses the `/api`-prefixed form for every endpoint that has one, so this flag only affects the few families Cloud serves at the root with no `/api` equivalent (workspace TTL settings, data planes). Leave it unset for Cloud.",
				Optional: true,
			},
			"path_overrides": schema.MapAttribute{
				MarkdownDescription: "Rewrite API request paths by prefix. This is an escape hatch for a deployment whose routing does not match what the provider assumes — you should not need it against LangSmith Cloud.\n\n" +
					"Each key is a path prefix to match and each value replaces it; both must begin with `/`. The **longest** matching key wins, and the rewrite is applied last, after the built-in `self_hosted` rules, so it can override them:\n\n" +
					"```hcl\npath_overrides = {\n  # send the platform family to the un-prefixed form instead\n  \"/api/v1/platform/\" = \"/v1/platform/\"\n}\n```\n\n" +
					"Can also be set with the `LANGSMITH_PATH_OVERRIDES` environment variable as a JSON object (for example `{\"/api/v1/platform/\":\"/v1/platform/\"}`). A non-null attribute overrides the environment variable.",
				ElementType: types.StringType,
				Optional:    true,
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

// getPathOverrides resolves the prefix-rewrite map from the attribute, falling
// back to LANGSMITH_PATH_OVERRIDES (a JSON object). Both keys and values must be
// absolute paths: a rewrite that produced a relative path would silently corrupt
// every URL built from it, so this is rejected rather than normalised.
func getPathOverrides(ctx context.Context, data LangSmithProviderModel, resp *provider.ConfigureResponse) map[string]string {
	overrides := map[string]string{}

	if !data.PathOverrides.IsNull() {
		resp.Diagnostics.Append(data.PathOverrides.ElementsAs(ctx, &overrides, false)...)
		if resp.Diagnostics.HasError() {
			return nil
		}
	} else if raw := strings.TrimSpace(os.Getenv("LANGSMITH_PATH_OVERRIDES")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
			resp.Diagnostics.AddError(
				"Invalid LANGSMITH_PATH_OVERRIDES",
				fmt.Sprintf("LANGSMITH_PATH_OVERRIDES must be a JSON object mapping path prefixes to replacements, for example {\"/api/v1/platform/\":\"/v1/platform/\"}: %s", err),
			)
			return nil
		}
	}

	for prefix, replacement := range overrides {
		switch {
		case prefix == "":
			resp.Diagnostics.AddAttributeError(
				path.Root("path_overrides"),
				"Empty path override prefix",
				"A path_overrides key must be a path prefix such as \"/api/v1/platform/\"; the empty string matches every request.",
			)
		case !strings.HasPrefix(prefix, "/"), !strings.HasPrefix(replacement, "/"):
			resp.Diagnostics.AddAttributeError(
				path.Root("path_overrides"),
				"Invalid path override",
				fmt.Sprintf("Both sides of a path_overrides entry must begin with \"/\"; got %q = %q.", prefix, replacement),
			)
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func getSelfHosted(data LangSmithProviderModel) bool {
	if !data.SelfHosted.IsNull() {
		return data.SelfHosted.ValueBool()
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LANGSMITH_SELF_HOSTED"))) {
	case "1", "true", "yes":
		return true
	}
	return false
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
	if data.SelfHosted.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("self_hosted"),
			"Unknown self_hosted value",
			"The provider cannot be configured because `self_hosted` is not known at plan time. "+remedy,
		)
	}
	if data.PathOverrides.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("path_overrides"),
			"Unknown path_overrides value",
			"The provider cannot be configured because `path_overrides` is not known at plan time. "+remedy,
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
	selfHosted := getSelfHosted(data)
	pathOverrides := getPathOverrides(ctx, data, resp)
	if resp.Diagnostics.HasError() {
		return
	}
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

	c := client.NewClient(apiURL, apiKey, workspaceId, userAgent, selfHosted, pathOverrides)

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

		// API parity (1.2)
		NewOAuthClientResource,
		NewOptimizationJobResource,
		NewHubDirectoryResource,
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

		// API parity (1.2)
		NewDatasetVersionsDataSource,
		NewSharedTokensDataSource,
		NewSessionAgentVersionsDataSource,
		NewMCPVendorDetailsDataSource,
		NewOAuthAuthorizedAppsDataSource,
		NewInfoHealthDataSource,
		NewOptimizationJobLogsDataSource,
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

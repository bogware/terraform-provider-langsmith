# INSTALL.md — Standing up LangSmith as a platform

This is an executable specification for provisioning a complete LangSmith platform with
`terraform-provider-langsmith`. It is written to be followed literally by an AI agent or an
engineer, in order, without further context.

Read [Rules](#2-rules-that-are-not-negotiable) before writing any HCL. Most failures in this
provider come from violating one of those five rules, not from getting an attribute wrong.

---

## 1. Preconditions (these cannot be Terraformed)

You must have all of these **before** the first `terraform apply`. None can be created by this
provider, and no workaround exists.

| Precondition | Why it is out of band |
|---|---|
| A LangSmith **organization** exists | Org creation is tied to signup and billing. |
| Its **plan/tier** and seat count are set | Billing is a Stripe flow, deliberately not modeled. |
| A **bootstrap API key** | Chicken-and-egg: the provider needs a credential to create credentials. |

Create the bootstrap key by hand in the LangSmith UI (Settings → API Keys). Then:

```bash
export LANGSMITH_API_KEY="lsv2_..."       # required
export LANGSMITH_WORKSPACE_ID="<uuid>"    # required IF the key is org-scoped
export LANGSMITH_API_URL="https://..."    # only for self-hosted
```

**Which key do you need?** It depends on how much of the platform you intend to manage:

- **Creating workspaces, org roles, org members, SSO, SCIM** → the key must be **org-scoped**, and
  the identity behind it must be an **organization admin**. A workspace-scoped service key returns
  `403` on every `/orgs/current/*` endpoint.
- **Only managing content inside one existing workspace** (projects, datasets, prompts, …) → a
  workspace-scoped key is sufficient. Set `LANGSMITH_WORKSPACE_ID` to that workspace.

> Verify your key before you build anything on it:
> ```bash
> curl -s -o /dev/null -w '%{http_code}\n' \
>   -H "X-API-Key: $LANGSMITH_API_KEY" \
>   https://api.smith.langchain.com/api/v1/orgs/current   # 200 = org-scoped and usable
> ```

---

## 2. Rules that are not negotiable

**Rule 1 — Set `workspace_id` explicitly on every workspace-scoped resource.**
If you omit it, the resource is silently created in the provider-level workspace. In a
multi-workspace configuration that means resources land in the *wrong workspace* with no error.

**Rule 2 — Never point a `provider` block at a workspace you are creating in the same apply.**
Terraform configures providers *before* applying the resources they depend on, so
`workspace_id = langsmith_workspace.prod.id` inside a `provider` block is still unknown at that
moment. The provider rejects this with an explicit error. Use the per-resource `workspace_id`
attribute instead — it *does* accept a value that is unknown at plan time and known at apply time.
This is what makes a one-apply standup possible.

```hcl
# WRONG — provider config cannot depend on a resource being created in this apply
provider "langsmith" {
  alias        = "prod"
  workspace_id = langsmith_workspace.prod.id   # ERROR: unknown at plan time
}

# RIGHT — the resource takes the reference
resource "langsmith_project" "traces" {
  workspace_id = langsmith_workspace.prod.id   # resolved by the time Create runs
  name         = "production"
}
```

Provider aliases remain correct for workspaces whose IDs are **already known** (a variable, or a
pre-existing workspace).

**Rule 3 — `workspace_id` forces replacement.** Objects do not move between workspaces. Changing
`workspace_id` destroys and recreates the resource. Treat it as part of the resource's identity.

**Rule 4 — Secrets are returned exactly once.** `langsmith_api_key.key`,
`langsmith_service_key.key`, `langsmith_personal_access_token.key`, `langsmith_scim_token.token`,
`langsmith_secret.value`, `langsmith_sandbox_registry.password`. They are readable only in the apply
that creates them, and the provider cannot detect drift on them. Capture them into a secret manager
in the same apply, or you will have to recreate the resource to see the value again.

**Rule 5 — Your Terraform state will contain secrets in plaintext.** This is inherent to Terraform,
not specific to this provider. Use a remote backend with encryption at rest and restricted access.
Do not commit `*.tfstate` or `*.tfvars`.

---

## 3. Dependency layers

Build in this order. Terraform resolves the ordering itself from the references, so you do not need
`depends_on` — but you must understand the layering to know what is reachable.

```
L0  OUT OF BAND ......... organization, tier/billing, bootstrap API key        (§1)
L1  ORGANIZATION ........ organization_settings, sso_settings, scim_token,
                          ttl_settings (org-wide retention), model_price_map
L2  IDENTITY & RBAC ..... org_role  ──id──┐
                          access_policy ──┴──► role_access_policies   (ABAC only)
                          org_role.id ───────► org_member (invite by email)
                                     └───────► api_key / service_key / service_account
L3  WORKSPACE ........... workspace ──id──► (everything below, via workspace_id)
                          workspace_handle, workspace_ttl_settings, usage_limit,
                          secret, playground_settings, feature_model_config,
                          hub_environment, mcp_vendor_settings, sandbox_registry,
                          agent_builder_integrations, bulk_export_destination
L4  MEMBERSHIP .......... org_member.user_id ──► workspace_member(user_id, role_id)
                          workspace_member.id ─► annotation_queue_reviewer
L5  CONTENT ............. project ──► alert_rule, filter_view, insights_config, bulk_export
                          dataset ──► example, dataset_split, dataset_share,
                                      dataset_version_tag, experiment_view_override
                          prompt  ──► prompt_tag, repo_owner, webhook
                          tool, annotation_queue
                          tag_key ──► tag_value ──► tagging
L6  AUTOMATION .......... evaluator + project + dataset ──► run_rule
                          chart_section ──► chart      (org_chart_section ──► org_chart)
                          feedback_config ──► feedback_formula
```

**Org-scoped resources** (these ignore `workspace_id`; they act on `/orgs/current/*` and require an
org-scoped admin key): `workspace`, `org_role`, `org_member`, `organization_settings`,
`sso_settings`, `scim_token`, `personal_access_token`, `data_plane`.

---

## 4. Reference configuration

A complete, ordered standup. Substitute your own values; the structure is the point.

### 4.1 Provider

```hcl
terraform {
  required_providers {
    langsmith = {
      source  = "bogware/langsmith"
      version = "~> 1.0"
    }
  }
}

# Org-scoped key, and deliberately NO workspace_id: this configuration creates
# workspaces, so each resource selects its own workspace explicitly (Rule 1/2).
provider "langsmith" {
  api_key = var.langsmith_org_api_key # sensitive; from env or a secret manager
}
```

### 4.2 L1 — Organization

```hcl
resource "langsmith_organization_settings" "this" {
  # Adopts the existing org settings on create; destroy is state-only.
  invites_enabled = true
}

resource "langsmith_ttl_settings" "org" {
  # Org-wide default trace retention.
  default_trace_tier = "longlived"
}

# SSO (enterprise). Metadata comes from your IdP, out of band.
resource "langsmith_sso_settings" "okta" {
  metadata_url              = var.idp_metadata_url
  default_workspace_role_id = langsmith_org_role.developer.id
}
```

### 4.3 L2 — Roles, policies, members, credentials

```hcl
data "langsmith_permissions" "all" {} # the catalog of grantable permissions

resource "langsmith_org_role" "developer" {
  display_name = "developer"
  permissions  = jsonencode(["sessions:read", "datasets:read", "datasets:write"])

  # A restricted role may hold ONLY the permissions explicitly granted here.
  is_restricted = true
}

# Invite a human to the organization.
resource "langsmith_org_member" "alice" {
  email   = "alice@example.com"
  role_id = langsmith_org_role.developer.id
}
```

> **Invite acceptance is asynchronous and out of band.** `langsmith_org_member.user_id` is `null`
> until the invitee accepts. Because `langsmith_workspace_member` requires a `user_id`, putting a
> *newly invited* human into a workspace is inherently a **two-phase apply**: invite → they accept →
> apply again. Users who already exist (or are provisioned by SSO/SCIM) work in a single apply.

### 4.4 L3 — Workspace, created and populated in ONE apply

```hcl
resource "langsmith_workspace" "prod" {
  display_name = "production"
}

locals {
  # Reference this in every workspace-scoped resource below (Rule 1).
  ws = langsmith_workspace.prod.id
}

resource "langsmith_usage_limit" "prod" {
  workspace_id = local.ws
  limit_type   = "monthly_traces"
  limit_value  = 10000000
}

resource "langsmith_secret" "openai" {
  workspace_id = local.ws
  key          = "OPENAI_API_KEY"
  value        = var.openai_api_key # write-only; drift is undetectable (Rule 4)
}

# A scoped key for CI. `key` is readable ONLY in this apply.
resource "langsmith_api_key" "ci" {
  workspace_id = local.ws
  role_id      = langsmith_org_role.developer.id
  description  = "ci"
}
```

### 4.5 L5/L6 — Content and automation

```hcl
resource "langsmith_project" "traces" {
  workspace_id = local.ws
  name         = "production"
}

resource "langsmith_dataset" "golden" {
  workspace_id = local.ws
  name         = "golden-set"
}

resource "langsmith_prompt" "judge" {
  workspace_id = local.ws
  repo_handle  = "llm-judge"
  is_public    = false

  # The manifest is a LangChain serialized object. Keep it in a file for anything
  # real: manifest = file("${path.module}/prompts/judge.json")
  manifest = jsonencode({
    lc   = 1
    type = "constructor"
    id   = ["langchain", "prompts", "chat", "ChatPromptTemplate"]
    kwargs = {
      input_variables = ["question"]
      messages = [{
        lc   = 1
        type = "constructor"
        id   = ["langchain", "prompts", "chat", "HumanMessagePromptTemplate"]
        kwargs = {
          prompt = {
            lc   = 1
            type = "constructor"
            id   = ["langchain", "prompts", "prompt", "PromptTemplate"]
            kwargs = {
              input_variables = ["question"]
              template        = "Grade this answer: {question}"
              template_format = "f-string"
            }
          }
        }
      }]
    }
  })

  # The server tags the repo with the manifest type on commit; declare it so the
  # post-apply refresh plan is empty.
  tags = ["ChatPromptTemplate"]
}

resource "langsmith_evaluator" "correctness" {
  workspace_id = local.ws
  name         = "correctness"
  type         = "llm"

  llm_evaluator = {
    prompt_repo_handle = langsmith_prompt.judge.repo_handle
    commit_hash_or_tag = langsmith_prompt.judge.commit_hash
  }
}

# Sample 10% of production traces, score them, and file the results into the dataset.
resource "langsmith_run_rule" "eval" {
  workspace_id      = local.ws
  display_name      = "score-and-collect"
  sampling_rate     = 0.1
  session_id        = langsmith_project.traces.id # NOTE: a project is a "session" in the API
  is_enabled        = true
  evaluator_id      = langsmith_evaluator.correctness.id
  add_to_dataset_id = langsmith_dataset.golden.id
}

resource "langsmith_alert_rule" "latency" {
  workspace_id   = local.ws
  session_id     = langsmith_project.traces.id
  name           = "High latency"
  description    = "p50 latency over 5s"
  type           = "threshold"
  aggregation    = "avg"
  attribute      = "latency"
  operator       = "gte"
  threshold      = 5000
  window_minutes = 60

  # Alert destinations are inline JSON; they cannot reference langsmith_webhook.
  # This attribute is marked sensitive because such configs routinely carry tokens.
  actions = jsonencode([{ target = "webhook", config = { url = var.alert_webhook_url } }])
}
```

> **Naming seam:** a tracing project is called a **project** as a resource
> (`langsmith_project`) but a **session** everywhere it is referenced (`session_id`). This mirrors
> the API. Write `session_id = langsmith_project.x.id`.

---

## 5. Feature gates

These resources exist but return `403` unless the feature is enabled for your organization. Enabling
them is a commercial/support action, not a Terraform one.

| Resource | Requires | Error if missing |
|---|---|---|
| `access_policy`, `role_access_policies`, `access_policies` (DS) | ABAC | `ABAC is not enabled for this organization` |
| `gateway_policy`, `gateway_policies` (DS) | LLM Gateway | `LLM Gateway not enabled for this organization` |
| `issues_agent` | Engine | `Engine feature is not enabled for this tenant` |
| `scim_token` | SCIM (enterprise) | `403` |
| `org_role`, `workspace_member` | Team/Enterprise tier | `403` |
| `service_key`, `api_key` (org-wide) | Organization admin | `Only org admins can create org-wide keys` |
| `data_plane` | BYOC | `403` |

---

## 6. What is still not Terraformable

After a complete apply, these remain manual:

- **Organization creation, tier, billing, seats.**
- **The bootstrap credential** (§1).
- **Invite acceptance** — or provision users via SSO JIT / SCIM instead.
- **`langsmith_personal_access_token`** requires a *user*-scoped credential; a service key cannot
  create one.
- **GitHub App connection** for `langsmith_issues_agent`.
- **BYOC data plane**: `role_arn` / `external_id` come from AWS IAM, and there is **no delete
  endpoint** — `terraform destroy` removes it from state while the data plane keeps running. Contact
  support to deprovision.
- **LangGraph deployments** — no resource exists.

---

## 7. Verify

```bash
terraform init
terraform plan      # review: nothing unexpected should be replaced
terraform apply

# The strongest signal: a second plan must be EMPTY.
terraform plan -detailed-exitcode   # exit 0 = clean; exit 2 = drift or a phantom diff
```

A non-empty second plan is a bug — report it. It means a resource is not round-tripping its own
state.

---

## 8. Importing an existing platform

Most resources import by their opaque ID. Resources whose `Read` needs a **parent** take a composite
ID:

| Resource | Import ID |
|---|---|
| `langsmith_secret` | `<key>` or `<key>:<workspace_id>` |
| `langsmith_tagging` | `<tag_value_id>:<tagging_id>` |
| `langsmith_comparative_experiment` | `<reference_dataset_id>/<experiment_id>` |
| `langsmith_alert_rule` | `<session_id>/<rule_id>` |
| `langsmith_filter_view` | `<session_id>/<view_id>` |
| `langsmith_insights_config` | `<session_id>:<config_id>` |
| `langsmith_experiment_view_override` | `<dataset_id>:<id>[:<workspace_id>]` |
| `langsmith_tag_value` | `<tag_key_id>/<value_id>` |
| `langsmith_dataset_split` | `<dataset_id>:<split_name>` |
| `langsmith_dataset_version_tag` | `<dataset_id>:<tag>` |
| `langsmith_repo_owner` | `<owner>:<repo>:<identity_id>` |
| `langsmith_prompt` | `<owner>/<repo_handle>` |
| `langsmith_prompt_tag` | `<repo_handle>/<tag_name>` |
| `langsmith_feature_model_config` | `<feature>[:<workspace_id>]` |
| `langsmith_mcp_vendor_settings` | `<vendor_slug>[:<workspace_id>]` |
| `langsmith_annotation_queue_reviewer` | `<queue_id>:<identity_id>` |
| `langsmith_sandbox_registry` | `<name>` |
| `langsmith_optimization_job` | `<owner>/<repo>/<job_id>` |
| `langsmith_hub_directory` | `<owner>/<repo>` |

**Import cannot recover write-only secrets** (Rule 4). After importing a `langsmith_secret`, the
first plan will want to write `value` — that is expected, not drift. An imported
`langsmith_oauth_client` carries a null `client_secret` for the same reason: the API issues it once
at registration and never returns it again. Rotate it with the `rotate_secret` trigger if you need a
value you can capture.

`langsmith_data_plane` and `langsmith_feedback_ingest_token` refuse import by design.

---

## 9. Failure modes

| Symptom | Cause | Fix |
|---|---|---|
| Resource created in the wrong workspace | `workspace_id` omitted → fell back to the provider workspace | Set `workspace_id` explicitly (Rule 1) |
| `Unknown LangSmith workspace ID` at plan | A `provider` block references a workspace created in this apply | Use the per-resource `workspace_id` (Rule 2) |
| `403 Forbidden` on `/orgs/current/*` | Workspace-scoped key used for an org-scoped resource | Use an org-scoped admin key (§1) |
| Everything plans as replace after an upgrade | `workspace_id` is now `RequiresReplace` and your state has it null/differing | Confirm intent; `terraform state show` before applying |
| `409 already exists` on re-apply | An earlier create failed midway | Fixed in ≥0.11: partial state is persisted and the resource is tainted and replaced |
| Second plan is not empty | Phantom diff | Report it as a bug (§7) |

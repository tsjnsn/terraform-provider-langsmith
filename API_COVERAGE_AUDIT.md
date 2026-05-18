# LangSmith OpenAPI vs Terraform provider coverage

This document maps themes of the public LangSmith HTTP API ([OpenAPI reference](https://api.smith.langchain.com/openapi.json), OpenAPI **3.1.0**) to Terraform resources and data sources in this provider. Path inventories change over time; treat this file as orienting documentation, not a formal contract.

## Method

- **Terraform inventory:** registrations in [`internal/provider/provider.go`](internal/provider/provider.go) — **51** resources and **34** data sources (as of the last manual refresh of this count).
- **Mapping:** Each Terraform type corresponds to explicit HTTP calls in [`internal/provider/`](internal/provider/).
- **Registry docs:** [Terraform Registry — bogware/langsmith](https://registry.terraform.io/providers/bogware/langsmith/latest/docs).

### Provider attributes

| Attribute | Role |
|-----------|------|
| `organization_id` | Sent as `X-Organization-Id`; required for many organization-scoped API calls (org charts, audit logs, org directory listings, etc.). |

### Resource and data source index (by endpoint theme)

Supplements the thematic table below — useful when checking which type covers which route pattern.

| Kind | Terraform type | API / notes |
|------|----------------|-------------|
| Provider | — | Optional `organization_id` → `X-Organization-Id` |
| Resource | `langsmith_api_key` | `GET`/`POST` `/api/v1/api-key`, `DELETE` `/api/v1/api-key/{id}` |
| Resource | `langsmith_settings` | `GET` `/api/v1/settings`, `POST` `/api/v1/settings/handle` |
| Resource | `langsmith_platform_feature` | `/v1/platform/features/*` (default/disabled models per feature) |
| Resource | `langsmith_fleet_mcp_server` | `/v1/platform/fleet/mcp-servers` |
| Data source | `langsmith_settings` | `GET` `/api/v1/settings` |
| Data source | `langsmith_tenants` | `GET` `/api/v1/tenants` (workspace-scoped keys often return **403**; acceptance tests skip — see [`AGENTS.md`](AGENTS.md)) |
| Data source | `langsmith_organizations` | `GET` `/api/v1/orgs` |
| Data source | `langsmith_organization_permissions` | `GET` `/api/v1/orgs/permissions` |
| Data source | `langsmith_organization_pending_invites` | `GET` `/api/v1/orgs/pending` |
| Data source | `langsmith_platform_features` | `GET` `/v1/platform/features` |
| Data source | `langsmith_project_agent_versions` | `/v1/platform/sessions/{id}/agent-versions` |
| Data source | `langsmith_sso_settings_by_slug` | `GET` `/api/v1/sso/settings/{sso_login_slug}` |
| Data source | `langsmith_feedback_ingest_tokens` | `GET` `/api/v1/feedback/tokens` (listing; complements create-only resource `langsmith_feedback_ingest_token`) |

**Historical note:** A prior `langsmith_audit_logs` data source duplicated [`langsmith_audit_log`](./docs/data-sources/audit_log.md) on `GET /api/v1/audit-logs`; use `langsmith_audit_log` only.

## Control-plane coverage (by API theme)

Rows summarize whether Terraform exposes durable create/read/update/delete (where applicable) for each theme.

| Theme | Provider / outcome |
|--------|-------------------------|
| **Workspace / tenant** | **Implemented:** `langsmith_workspace`, `langsmith_settings` (resource + data source), `langsmith_tenants`, `langsmith_workspace` (lookup), `langsmith_secret`, workspace tagging (`langsmith_tag_key`, `langsmith_tag_value`, `langsmith_tagging`), `langsmith_workspace_member`. |
| **API keys / PATs** | **Implemented:** `langsmith_api_key` (`/api/v1/api-key`). Service keys: `langsmith_service_key` (+ `langsmith_service_account`). PATs: `langsmith_personal_access_token`. |
| **Organization directory & RBAC** | **Implemented:** `langsmith_org_member`, `langsmith_org_role` (+ data source), `langsmith_access_policy`, `langsmith_scim_token`. **Read-only:** `langsmith_organization`, `langsmith_organizations`, `langsmith_organization_permissions`, `langsmith_organization_pending_invites`. Accept/decline invite flows remain **interactive / out of scope**. |
| **Advanced org APIs** | **Partially addressed:** Core membership, roles, SSO, SCIM, charts, platform, gateway, evaluators, and org lists above. Add one-off admin routes only when a stable declarative contract exists. |
| **Workspace / org settings & TTL** | **Implemented:** `langsmith_settings`, `langsmith_ttl_settings`, `langsmith_playground_settings`, `langsmith_usage_limit`, `langsmith_model_price_map`. |
| **Projects, datasets, examples** | **Implemented:** `langsmith_project` (+ data source), `langsmith_dataset` (+ data source), `langsmith_example`, `langsmith_dataset_share`, `langsmith_dataset_split`. |
| **Runs automation** | **Implemented:** `langsmith_run_rule` (+ data source), `langsmith_annotation_queue` (+ data source), `langsmith_annotation_queue_reviewer`, `langsmith_filter_view`, `langsmith_alert_rule`. |
| **Prompts & Hub** | **Implemented:** `langsmith_prompt` (+ data source), `langsmith_prompt_commit`, `langsmith_prompt_tag`, `langsmith_webhook`, `langsmith_repo_owner`, `langsmith_hub_environment`. |
| **Feedback configuration** | **Implemented:** `langsmith_feedback_config`, `langsmith_feedback_formula`, `langsmith_feedback_ingest_token`, `langsmith_feedback_ingest_tokens`. **Not Terraform:** posting scores on traces — see **Out of scope**. |
| **Charts** | **Implemented:** workspace `langsmith_chart`, `langsmith_chart_section`, `langsmith_chart_section_clone`; org `langsmith_org_chart`, `langsmith_org_chart_section`; lookup/preview data sources. |
| **SSO (SAML)** | **Implemented:** `langsmith_sso_settings`; **read-only:** `langsmith_sso_settings_by_slug`. Interactive `/api/v1/sso/email-*` flows are **out of scope**. |
| **Audit** | **Implemented:** `langsmith_audit_log` (`GET /api/v1/audit-logs`). |
| **Bulk export** | **Implemented:** `langsmith_bulk_export_destination`, `langsmith_bulk_export`. |
| **Platform — tools** | **Implemented:** `langsmith_tool` (resource + data source, `/v1/platform/tools`). |
| **Platform — agent versions** | **Read-only:** `langsmith_project_agent_versions`. |
| **Platform — features / evaluators / gateway** | **Implemented:** `langsmith_platform_feature`, `langsmith_platform_features`, `langsmith_evaluator` (+ data source), `langsmith_gateway_policy` (+ data source). |
| **Platform — fleet MCP** | **Implemented:** `langsmith_fleet_mcp_server`. Other fleet GitHub App / usage / webhook routes: **won’t do** (see below). |
| **Tenants listing** | **Implemented with guardrails:** `langsmith_tenants`. Workspace-scoped keys often **403** — acceptance tests skip (`AGENTS.md`). |
| **Info / users / data planes** | **Read-only:** `langsmith_info`, `langsmith_user`, `langsmith_data_planes`. |
| **Insights (beta)** | **Implemented:** `langsmith_insights_config`. |

## Explicitly out of scope for Terraform (won’t do — rationale)

| Area | Rationale |
|------|-----------|
| Trace/run **ingestion**, **query**, **streaming**, **stats** | Hot path; query/analysis semantics, not declarative infra. |
| **Public share links** (beyond dataset share state) | Ephemeral sharing; security-sensitive. |
| **OAuth login** and interactive **SSO** steps | User auth flows, not infra definitions. |
| **MCP proxy**, **ACE**, **comments/likes** on repos | Product/UX surfaces. |
| **Feedback** on traces (`POST /api/v1/feedback`, eager, etc.) | Runtime scores; use ingest **tokens** for automation boundaries. |
| **Fleet** GitHub App / usage / webhook delivery | Connector and telemetry; not steady-state IaC. |

## Maintainer note

Avoid registering multiple Terraform resources or data sources for the same HTTP surface. When adding or removing types, update [`internal/provider/provider.go`](internal/provider/provider.go), run **`make generate`**, refresh this audit for major coverage changes, and keep [`README.md`](README.md) resource/data source inventories aligned where appropriate.

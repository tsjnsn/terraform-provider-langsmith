# LangSmith OpenAPI vs Terraform provider audit

**Scope:** `terraform-provider-langsmith` (resources and data sources under `internal/provider/`) compared to the public LangSmith OpenAPI description at `https://api.smith.langchain.com/openapi.json`.

**Method:** Path inventory from OpenAPI (May 2026) cross-checked against `grep` of HTTP paths in `internal/provider/*.go`. This is not an exhaustive field-level parity check; it highlights major API surface areas with no Terraform mapping.

## Already covered (representative `/api/v1` paths)

| Area | Notes |
|------|--------|
| Projects (`/api/v1/sessions`) | `langsmith_project` |
| Filter views (`/api/v1/sessions/{id}/views`) | `langsmith_filter_view` |
| Datasets & examples (basic CRUD) | `langsmith_dataset`, `langsmith_example` |
| Annotation queues | `langsmith_annotation_queue` |
| Run automation rules | `langsmith_run_rule` |
| Prompt repos, tags, commits | `langsmith_prompt`, `langsmith_prompt_tag`, data sources |
| Prompt webhooks | `langsmith_webhook` |
| Feedback configs & formulas | `langsmith_feedback_config`, `langsmith_feedback_formula` |
| Workspaces, members, secrets, tags | Workspace/member/secret/tag resources |
| Service accounts & org service keys | `langsmith_service_account`, `langsmith_service_key` |
| Bulk exports | `langsmith_bulk_export_destination`, `langsmith_bulk_export` |
| Model price map, usage limits | `langsmith_model_price_map`, `langsmith_usage_limit` |
| Playground settings | `langsmith_playground_settings` |
| Org TTL | `langsmith_ttl_settings` (`/api/v1/orgs/ttl-settings`) |
| Org roles, SSO, members | `langsmith_org_role`, `langsmith_sso_settings`, `langsmith_org_member` |
| Charts (workspace charts API) | `langsmith_chart`, `langsmith_chart_section` |
| Access policies & SCIM tokens | `langsmith_access_policy`, `langsmith_scim_token` (platform paths) |
| Alert rules | `langsmith_alert_rule` (`/v1/platform/alerts/...`) |

Platform paths `/v1/platform/...` are used where the public API routes admin features outside `/api/v1`.

## Documented intentional exclusions

| API | Reason |
|-----|--------|
| `POST /runs`, `PATCH /runs/{id}`, `POST /runs/multipart`, batch tracing | Ephemeral telemetry; LangChain recommends SDKs. Poor fit for Terraform state. |
| `GET /api/v1/runs/query`, run stats, threads | Operational/query APIs, not declarative infrastructure. |

## Gaps (suggested backlog items)

Each row is tracked as a separate GitHub issue with label `langsmith-api-gap` (and can be mirrored into the Linear project manually).

| ID | OpenAPI area (representative paths) | Gap summary |
|----|--------------------------------------|-------------|
| GAP-01 | `/v1/platform/evaluators`, `/v1/platform/evaluators/{evaluator_id}` | No evaluator definitions as Terraform resources. |
| GAP-02 | `/api/v1/org-charts`, `/api/v1/org-charts/*` | Org-level charts are separate from workspace `/api/v1/charts/*` (already implemented). |
| GAP-03 | `/v1/platform/gateway-policies`, `/v1/platform/gateway-policies/{id}` | Gateway / LLM routing policies not exposed. |
| GAP-04 | `/v1/platform/annotation-queues/{queue_id}/reviewers/*` | Queue membership for reviewers not managed (distinct from workspace members). |
| GAP-05 | `/workspaces/current/ttl-settings`, `/api/v1/ttl-settings` | Workspace-scoped TTL may differ from org singleton `langsmith_ttl_settings`. |
| GAP-06 | `/api/v1/api-key/*`, `/api/v1/orgs/current/personal-access-tokens/*` | PAT / API key lifecycle for users not covered (service keys exist separately). |
| GAP-07 | `/api/v1/datasets/clone`, `.../splits`, `.../versions`, `.../share`, `.../upload-experiment`, comparative experiments | Dataset lifecycle beyond create/read/update/delete. |
| GAP-08 | `/api/v1/examples/bulk`, `/api/v1/examples/upload/{dataset_id}`, validate endpoints | Bulk example operations not modeled (single-example resource only). |
| GAP-09 | `/api/v1/sessions/{session_id}/insights/*` | Project/session insights jobs and configs not exposed. |
| GAP-10 | `/api/v1/audit-logs` | No read-only data source for audit log export. |
| GAP-11 | `/api/v1/hub/environments/*` | Hub environments API has no provider mapping. |
| GAP-12 | `/api/v1/feedback` (per-run feedback), `/api/v1/feedback/{feedback_id}` | Distinct from feedback *configs* / *formulas*; run-attached scores not managed. |

## Linear project

Target Linear project (from request): `https://linear.app/team-tyty/project/tsjnsnterraform-langsmith-aa3b8885d64a/overview`

If the Linear integration is unavailable from this environment, use GitHub issues labeled `langsmith-api-gap` as the source of truth, or copy issue titles and descriptions into Linear issues one by one.

## References

- LangSmith OpenAPI: `https://api.smith.langchain.com/openapi.json`
- Interactive docs: `https://api.smith.langchain.com/redoc`
- Tracing API guide: `https://docs.langchain.com/langsmith/trace-with-api`

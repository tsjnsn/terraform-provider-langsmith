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

GitHub Issues are disabled on `tsjnsn/terraform-provider-langsmith`, so the items below are written as **ready-to-paste Linear issues** (one issue per gap). Create them in the project above and link each PR to a single issue.

---

### GAP-01 — Platform evaluators

**Title:** Terraform: add `langsmith_evaluator` (platform evaluators API)

**Description:**
OpenAPI defines `/v1/platform/evaluators` and `/v1/platform/evaluators/{evaluator_id}`. The Terraform provider has no resource or data source for evaluator definitions.

**Acceptance criteria:** Resource and/or data source matching API capabilities; docs in `docs/resources/`; tests mirroring other platform resources.

---

### GAP-02 — Org charts

**Title:** Terraform: support org-level charts (`/api/v1/org-charts`)

**Description:**
Workspace charts are implemented (`langsmith_chart` using `/api/v1/charts/*`). OpenAPI also lists `/api/v1/org-charts/*` with parallel section/chart flows. Add org-chart resources or document when to use workspace vs org charts.

**Acceptance criteria:** Clear API mapping, provider resources or explicit doc-only scope decision.

---

### GAP-03 — Gateway policies

**Title:** Terraform: add gateway policies (`/v1/platform/gateway-policies`)

**Description:**
Enterprise-style gateway / routing policy endpoints exist under `/v1/platform/gateway-policies`. No provider coverage today.

**Acceptance criteria:** CRUD aligned with OpenAPI; document required API key scopes.

---

### GAP-04 — Annotation queue reviewers

**Title:** Terraform: annotation queue reviewer assignments

**Description:**
`/v1/platform/annotation-queues/{queue_id}/reviewers` and `.../reviewers/{identity_id}` manage queue-level reviewers. Distinct from `langsmith_workspace_member`.

**Acceptance criteria:** Resource modeling reviewer list per queue; import support if IDs are stable.

---

### GAP-05 — Workspace TTL settings

**Title:** Terraform: workspace-scoped TTL (`/workspaces/current/ttl-settings`)

**Description:**
`langsmith_ttl_settings` targets org TTL (`/api/v1/orgs/ttl-settings`). OpenAPI also exposes `/workspaces/current/ttl-settings` and `/api/v1/ttl-settings` for workspace-level configuration.

**Acceptance criteria:** New resource or extend existing with explicit scope; avoid ambiguous defaults.

---

### GAP-06 — Personal access tokens / API keys

**Title:** Terraform: PAT and workspace API key lifecycle

**Description:**
OpenAPI includes `/api/v1/api-key/*`, `/api/v1/orgs/current/personal-access-tokens/*`, and related routes. Provider covers org `service-keys` but not user PAT flows.

**Acceptance criteria:** Decide Terraform suitability (secrets in state); if implemented, mark sensitive attributes and document rotation.

---

### GAP-07 — Advanced dataset operations

**Title:** Terraform: dataset clone, versions, splits, share, upload-experiment

**Description:**
Beyond CRUD on `/api/v1/datasets`, OpenAPI includes clone, splits, versions, share, comparative experiments, `upload-experiment`, exports (csv/jsonl), etc.

**Acceptance criteria:** Prioritize sub-features (e.g. splits vs upload-experiment); one resource family or documented out-of-scope with rationale.

---

### GAP-08 — Bulk examples

**Title:** Terraform: bulk example operations

**Description:**
`/api/v1/examples/bulk`, upload-by-dataset, and validate endpoints support large dataset management. Provider only exposes single-example `langsmith_example`.

**Acceptance criteria:** Optional `langsmith_dataset_examples` import pattern or `examples_json` batch resource; rate-limit considerations in docs.

---

### GAP-09 — Session insights

**Title:** Terraform: project/session insights configs and jobs

**Description:**
`/api/v1/sessions/{session_id}/insights/*` covers configs, jobs, clusters. Useful for orgs standardizing observability dashboards via IaC.

**Acceptance criteria:** Minimal viable resource set (e.g. config + job trigger) or datasource-only read.

---

### GAP-10 — Audit logs

**Title:** Terraform: `langsmith_audit_logs` data source (read-only)

**Description:**
`/api/v1/audit-logs` supports compliance export patterns. Read-only data source with filters and pagination is a natural fit.

**Acceptance criteria:** Pagination cursors exposed; document API retention limits.

---

### GAP-11 — Hub environments

**Title:** Terraform: hub environments API

**Description:**
`/api/v1/hub/environments` and `/{id}` appear in OpenAPI without provider mapping.

**Acceptance criteria:** Confirm product overlap with prompt repos; implement or document deferral.

---

### GAP-12 — Per-run feedback

**Title:** Terraform: run-attached feedback vs feedback configs

**Description:**
`/api/v1/feedback` and `/api/v1/feedback/{feedback_id}` attach scores to runs. Existing resources cover feedback *configs* and *formulas*, not individual feedback records.

**Acceptance criteria:** Explicit scope (likely narrow: idempotent feedback from CI) or document as intentionally excluded alongside tracing APIs.

## References

- LangSmith OpenAPI: `https://api.smith.langchain.com/openapi.json`
- Interactive docs: `https://api.smith.langchain.com/redoc`
- Tracing API guide: `https://docs.langchain.com/langsmith/trace-with-api`

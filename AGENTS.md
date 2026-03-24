# Repository Guidelines

## Mission
This repository is the single source of truth for organization-wide observability.
It currently contains two domains in one codebase:
1. `go-observability-kit`: reusable Go module consumed by Go/Gin services.
2. `obs-platform`: centralized LGTM platform (Grafana, Loki, Tempo, Prometheus, OTEL Collector).

This repo is intentionally designed to be split into two repos in the future, with minimal refactor.

## Product Goals
1. Standardize logs, metrics, traces, health checks, and alerting across all services.
2. Make incidents debuggable end-to-end using correlation IDs and trace context.
3. Enforce low-noise, actionable alerting with runbooks and ownership.
4. Provide a repeatable onboarding path so any new service is observable in <= 1 day.

## Non-Goals
1. Service business logic does not live here.
2. Per-team custom observability stacks are not supported in v1.
3. High-cardinality metric dimensions are not allowed.

## Monorepo Structure
- `module/` Go module `go-observability-kit`.
- `platform/` central observability stack configs and provisioning.
- `examples/` sample service + worker integration.
- `docs/` specs, playbooks, migration guides.
- `dashboards/` Grafana dashboard JSON and provisioning files.
- `alerts/` Prometheus/Alertmanager rule sets.
- `scripts/` validation, linting, and local bootstrap scripts.

## Architecture Standard
Telemetry flow must be:
1. Application emits telemetry using `go-observability-kit`.
2. OTEL SDK exports to OTEL Collector.
3. Collector routes metrics to Prometheus, logs to Loki, traces to Tempo.
4. Grafana reads from Prometheus/Loki/Tempo.
5. Alerts route via Alertmanager (or Grafana Alerting with equivalent policy).

Required labels across all signals:
- `project`
- `service`
- `env`
- `region`
- `version`
- `team`

## Module Requirements (`module/`)

### Public API (must remain stable)
- `bootstrap.Init(ctx, cfg) (*Runtime, error)`
- `bootstrap.Shutdown(ctx, rt) error`
- `ginx.Middleware(rt) gin.HandlerFunc`
- `workerx.AsynqMiddleware(rt) asynq.MiddlewareFunc`
- `dbx.WrapGORM(db, rt, opts...) *gorm.DB`
- `health.RegisterRoutes(router, checks...)`
- `metrics.RegisterRoute(router)`

### Required Service Endpoints
- `GET /healthz` liveness only.
- `GET /readyz` dependency readiness.
- `GET /metrics` Prometheus scrape endpoint.
- `GET /debug/pprof/*` optional, disabled by default in production.

### Required Environment Contract
- `SERVICE_NAME` required.
- `SERVICE_VERSION` required.
- `DEPLOYMENT_ENV` required.
- `OTEL_EXPORTER_OTLP_ENDPOINT` required.
- `OTEL_EXPORTER_OTLP_PROTOCOL` default `grpc`.
- `OTEL_TRACES_SAMPLER` default `parentbased_traceidratio`.
- `OTEL_TRACES_SAMPLER_ARG` env-specific defaults.
- `LOG_LEVEL` default `info`.
- `METRICS_ENABLED` default `true`.
- `PPROF_ENABLED` default `false`.

### Logging Contract
Logs must be structured JSON and include:
- `timestamp`
- `level`
- `msg`
- `service`
- `env`
- `trace_id`
- `span_id`
- `request_id`
- `route`
- `status_code`
- `latency_ms`

### Metrics Contract
Minimum metrics:
- `http_server_requests_total`
- `http_server_request_duration_seconds`
- `http_server_inflight_requests`
- `asynq_jobs_enqueued_total`
- `asynq_jobs_started_total`
- `asynq_jobs_succeeded_total`
- `asynq_jobs_failed_total`
- `asynq_jobs_retried_total`
- `asynq_job_duration_seconds`
- `asynq_queue_depth`
- `asynq_queue_oldest_age_seconds`
- `asynq_dead_letter_total`
- `db_client_queries_total`
- `db_client_query_duration_seconds`

Allowed metric labels:
- `service`, `env`, `method`, `route`, `status_class`, `queue`, `task_type`, `db_operation`, `result`

Forbidden metric labels:
- `user_id`, `request_id`, raw URL path values, email, phone, payload IDs, any unbounded identifier.

### Trace Contract
- All inbound HTTP requests must start/continue a trace.
- Worker jobs must create root spans when asynchronous boundaries are crossed.
- DB and outbound HTTP spans must be children of request/job span.
- Errors must set span status and attach exception events.

### Worker/Job Standard (mandatory)
Job lifecycle:
- `queued -> started -> succeeded | failed | retried | dead_lettered`

Required job fields in logs/spans:
- `job_execution_id`
- `asynq_job_id`
- `task_type`
- `queue`
- `attempt`
- `tenant_id` (if available)
- `trigger_source`
- `trace_id`

Required reliability policy:
- Idempotency key for side-effecting jobs.
- Exponential backoff with jitter.
- Max retries per task type.
- Dead-letter queue policy.
- Controlled replay tooling with audit trail.

### Module Engineering Rules
- Go version must match root `go.mod`.
- Use idiomatic Go and explicit error wrapping `%w`.
- All blocking work must accept `context.Context`.
- No panic for normal control flow.
- No telemetry path may block business path indefinitely.
- Telemetry failures must degrade gracefully.

## Platform Requirements (`platform/`)

### Components
- OTEL Collector (gateway mode).
- Prometheus.
- Loki.
- Tempo.
- Grafana.
- Alertmanager.

### Collector Pipeline
Receivers:
- `otlp` mandatory.

Processors:
- `memory_limiter` mandatory.
- `batch` mandatory.
- `resource`/`attributes` enrichment mandatory.
- `tail_sampling` recommended for production.

Exporters:
- Metrics -> Prometheus-compatible path.
- Logs -> Loki.
- Traces -> Tempo.

### Retention Defaults
- `dev`: metrics 7d, logs 7d, traces 3d.
- `stage`: metrics 15d, logs 15d, traces 7d.
- `prod`: metrics 30d+, logs 30d+, traces 14d+.

### Grafana Standards
- Data sources provisioned as code.
- Dashboards provisioned as code.
- Folder RBAC by team.
- Log-to-trace correlation configured via derived fields (`trace_id`).

### Alerting Standards
Every alert must include:
- `severity`
- `team`
- `summary`
- `description`
- `runbook_url`
- `dashboard_url`

Baseline alerts:
- HTTP 5xx rate spike.
- HTTP p95 latency breach.
- Service not ready/down.
- Queue oldest age breach.
- Retry storm.
- Dead-letter growth.
- DB dependency failure.

## SLO Standard
Minimum SLOs per production service:
1. Availability SLO.
2. Latency SLO.
3. Job freshness SLO for async-heavy services.

Burn-rate alerts must be configured for critical SLOs.

## Local Development

### Prerequisites
- Docker and Docker Compose.
- Go toolchain.
- Make (recommended).

### Typical Commands
- `make up` start local LGTM stack.
- `make down` stop stack.
- `make test` run module tests.
- `make lint` run lint checks.
- `make smoke` run end-to-end telemetry smoke tests.

### Dev Validation Checklist
- Generate one HTTP request and confirm metrics/log/trace visibility in Grafana.
- Run one worker job and confirm lifecycle telemetry.
- Verify `/healthz`, `/readyz`, `/metrics` contract.

## Deployment Model
- Centralized platform per organization with logical environment isolation.
- Services from all repos point OTLP endpoint to the environment collector.
- Module version is pinned per service via `go.mod`.

## CI/CD Requirements

### Module CI
- `go test -race ./...`
- Lint checks.
- Compatibility tests against supported Gin/GORM/Asynq versions.

### Platform CI
- Collector config validation.
- Prometheus rule lint.
- Dashboard provisioning validation.
- Alert rule quality checks.

### Consumer Service CI (policy)
- Fail build when required observability env vars are missing.
- Integration check for `/metrics` and middleware wiring.

## Security and Compliance
- Never log secrets, tokens, passwords, or raw PII.
- Provide configurable redaction list.
- Use TLS/mTLS and auth for collector ingress in non-local environments.
- Enforce least privilege for Grafana and data-source credentials.

## Documentation Requirements
Keep these docs updated for every behavior change:
- `docs/spec.md` architecture and contracts.
- `docs/requirements.md` product/engineering requirements.
- `docs/adr/` architectural decisions.
- `docs/runbooks/` alert runbooks.
- `docs/migration/` onboarding guides for service teams.
- `docs/releases/` module release notes and breaking changes.

## Decision Workflow (Mandatory)
All significant changes must be driven through docs first:
1. Update `docs/spec.md` for architecture or behavior contract changes.
2. Update `docs/requirements.md` for requirement/policy changes.
3. Add or update an ADR under `docs/adr/` for technical decisions/tradeoffs.
4. Then implement code/platform changes.
5. Include doc links in PR description.

Changes are incomplete if implementation lands without required spec/requirement/ADR updates.

## Deliverables Lock Workflow (Mandatory)
Execution must follow the locked plan in `docs/deliverables/`:
1. `D0-foundation-scaffold`
2. `D1-module-api-contract`
3. `D2-platform-baseline`
4. `D3-contracts-as-code`
5. `D4-example-integration`
6. `D5-cicd-enforcement`
7. `D6-documentation-completion`

Enforcement rules:
1. Work is strictly sequential by deliverable (D0 -> D6).
2. Do not start a deliverable until the previous one has explicit signoff.
3. Any scope change requires updating the affected deliverable README in `docs/deliverables/` before implementation.
4. For every deliverable handoff, provide: artifacts changed, checks run (exact commands), key evidence, pass/fail against expected outcomes, and explicit request for approval to proceed.

## Split-Ready Rules (future two-repo migration)
Design now so split is trivial:
1. `module/` must not import from `platform/`.
2. `platform/` must treat module as external consumer, not internal package.
3. Shared docs reference stable contracts, not internal paths.
4. Separate CI workflows by directory boundary.

## Definition of Done (for any feature)
1. Code changes merged with tests.
2. Telemetry contract preserved.
3. Dashboards/alerts updated when behavior changes.
4. Runbook updates included for new alerts.
5. Release notes and migration notes updated when API/config changes.

## Change Management
- Use Conventional Commits (`feat:`, `fix:`, `chore:`).
- Breaking changes require migration guidance.
- Prefer additive, backward-compatible changes in module APIs.

## Ownership
- Platform team owns `platform/` runtime, availability, and alert routing.
- Module maintainers own API stability and instrumentation quality.
- Service teams own correct module integration and service-level runbooks.

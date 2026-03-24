# Observability Requirements

## 1. Product Requirements

### 1.1 Core Outcomes
1. Every service exposes consistent observability endpoints and telemetry.
2. Incidents can be debugged from alert -> logs -> trace -> root cause quickly.
3. Jobs are observable with explicit lifecycle and backlog signals.
4. New service onboarding to observability completes in <= 1 working day.

### 1.2 In-Scope (v1)
1. Go/Gin HTTP services
2. Asynq workers and queues
3. GORM database telemetry
4. Central LGTM stack operations

### 1.3 Out-of-Scope (v1)
1. Non-Go runtime auto-instrumentation standards
2. Per-team dedicated observability stacks
3. Advanced ML anomaly detection

## 2. Functional Requirements

### 2.1 Module Functional Requirements
1. Provide stable public APIs defined in `docs/spec.md`.
2. Enforce required env config and fail fast for invalid required values.
3. Register `/healthz`, `/readyz`, `/metrics`.
4. Emit HTTP RED metrics.
5. Emit Asynq job lifecycle metrics.
6. Emit queue depth/backlog age metrics.
7. Emit DB query count/latency/error metrics.
8. Add request and trace correlation fields to logs.
9. Propagate trace context across HTTP and async boundaries.
10. Readiness checks must use context-aware `FnCtx` probes; legacy `Fn` usage is deprecated and removed after 2026-06-30.

### 2.2 Platform Functional Requirements
1. OTEL Collector must receive OTLP traffic from services.
2. Prometheus must scrape metrics endpoints and store queryable time series.
3. Loki must ingest and query logs with label filters.
4. Tempo must ingest and query traces with trace IDs.
5. Grafana must provision data sources and baseline dashboards from code.
6. Alertmanager must route alerts by severity and team ownership.

### 2.3 Job Functional Requirements
1. Support lifecycle state model `queued/started/succeeded/failed/retried/dead_lettered`.
2. Record required job context fields in logs/spans/metrics.
3. Enforce retry policy and dead-letter handling visibility.
4. Provide replay observability with audit metadata.

## 3. Non-Functional Requirements

### 3.1 Reliability
1. Telemetry failure must not break business request processing.
2. Exporters must use bounded memory/queues.
3. Collector outages must degrade gracefully with error counters.

### 3.2 Performance
1. Middleware overhead target: low single-digit milliseconds p95 under normal load.
2. Telemetry processing must avoid unbounded allocations in hot paths.
3. Blocking calls must honor context deadlines.

### 3.3 Security
1. No secrets/PII in logs and labels.
2. TLS/mTLS required for non-local ingestion paths.
3. Principle of least privilege for dashboards and datasources.

### 3.4 Operability
1. Platform and module configs managed as code.
2. All alerts must include runbook and dashboard links.
3. Runbooks required for all critical alerts.

## 4. Decision Requirements
1. Public module API changes require ADR.
2. Telemetry schema changes (labels/fields/metric names) require ADR.
3. Alert/SLO policy changes require ADR.
4. Breaking changes require migration document + release notes.

## 5. Quality Gates

### 5.1 Module Gates
1. `go test -race ./...` must pass.
2. Lint must pass.
3. Integration tests for endpoint and telemetry contracts must pass.
4. Public API compatibility checks must pass.

### 5.2 Platform Gates
1. Collector config validation must pass.
2. Prometheus rule lint must pass.
3. Grafana provisioning validation must pass.
4. Alert quality checks (labels/annotations) must pass.

### 5.3 Consumer Service Gates
1. Required env vars present.
2. `/metrics` exposed.
3. Forbidden high-cardinality labels absent.
4. Mandatory alerts exist with runbook links.

### 5.4 Repository Execution Gates (current baseline)
1. `make test` must pass (module race tests).
2. `make lint` must pass (platform/alerts/dashboards validations).
3. `make smoke` must pass (example endpoint + job lifecycle telemetry checks).
4. `scripts/check-boundaries.sh` must pass (split-ready boundary policy).
5. `scripts/check-docs-first.sh` must pass in CI for PR policy.
6. `scripts/test-race.sh` must pass in CI as race-test gate.
7. `scripts/check-readiness-fn-deprecation.sh` must pass (enforced after 2026-06-30).

## 6. Acceptance Criteria
1. Logs, traces, and metrics are correlatable with `trace_id` and `request_id`.
2. Baseline dashboards work with only `service` + `env` filters.
3. Queue backlog and retry storms are detectable within configured windows.
4. Burn-rate SLO alerting is active for production critical services.
5. A new service can integrate with zero contract exceptions.

## 7. Rollout Requirements
1. Pilot one real service first.
2. Validate incident workflow and alert noise for at least 2 weeks.
3. Roll out to tier-1 services next, then broader services.
4. Track adoption and compliance status per service.

## 8. Documentation Requirements
The following documents are mandatory and must stay current:
1. `docs/spec.md`
2. `docs/requirements.md`
3. `docs/adr/*.md`
4. `docs/runbooks/*.md`
5. `docs/releases/*.md`
6. `docs/migration/*.md`
7. `docs/deployment/*.md`
8. `docs/integration/*.md`
9. `docs/developer/*.md`

## 9. CI Enforcement Baseline (2026-03-24)
CI workflows must enforce the following:
1. Module CI: `module-ci.yml` runs `go test -race ./...` under `module/`.
2. Platform CI: `platform-ci.yml` runs `make lint`.
3. Policy CI: `policy-ci.yml` runs docs-first and split-ready boundary checks.
4. Example CI: `examples-smoke.yml` runs `make smoke`.

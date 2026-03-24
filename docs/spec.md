# Observability Architecture Spec

## 1. Purpose
This document defines the target architecture for:
1. `go-observability-kit` (service-side observability module)
2. `obs-platform` (central LGTM platform)

This architecture is mandatory for all integrated Go/Gin services.

## 2. High-Level Architecture

### 2.1 Components
1. Service runtime instrumented with `go-observability-kit`
2. OpenTelemetry Collector gateway
3. Prometheus (metrics)
4. Loki (logs)
5. Tempo (traces)
6. Grafana (dashboards/explore)
7. Alertmanager (routing)

### 2.2 Signal Flow
1. Services emit metrics/traces/log correlation metadata.
2. OTEL SDK exports OTLP to Collector.
3. Collector enriches and routes telemetry:
   1. Metrics -> Prometheus
   2. Traces -> Tempo
   3. Logs -> Loki
4. Grafana reads from all three backends.
5. Alert rules evaluate in Prometheus; alerts route through Alertmanager.

## 3. Module Architecture (`go-observability-kit`)

### 3.1 Package Boundaries
1. `bootstrap/` provider setup and shutdown
2. `config/` env parsing + validation
3. `ginx/` HTTP middleware (request ID, RED metrics, tracing)
4. `workerx/` Asynq middleware (job lifecycle telemetry)
5. `dbx/` GORM instrumentation
6. `health/` liveness/readiness handlers
7. `metrics/` prometheus route registration

Rule: keep API stable and implementation adapter-based.

### 3.2 Public API Contract
1. `bootstrap.Init(ctx, cfg) (*Runtime, error)`
2. `bootstrap.Shutdown(ctx, rt) error`
3. `ginx.Middleware(rt) gin.HandlerFunc`
4. `workerx.AsynqMiddleware(rt) asynq.MiddlewareFunc`
5. `dbx.WrapGORM(db, rt, opts...) *gorm.DB`
6. `health.RegisterRoutes(router, checks...)`
7. `metrics.RegisterRoute(router)`

### 3.3 Runtime Endpoints
1. `GET /healthz` (process liveness)
2. `GET /readyz` (dependency readiness)
3. `GET /metrics` (prometheus scrape)
4. `GET /debug/pprof/*` (optional, restricted)

### 3.4 Readiness Check Standard
1. Readiness checks must implement context-aware signature `FnCtx func(context.Context) error`.
2. Legacy `Fn func() error` compatibility is temporary and scheduled for removal on **2026-06-30**.
3. After the deprecation date, repository policy checks fail CI when new `Check{Fn: ...}` usage exists outside compatibility internals.

## 4. Platform Architecture (`obs-platform`)

### 4.1 Topology
1. One central stack per organization.
2. Logical isolation by labels (`project`, `service`, `env`, `region`, `version`, `team`).
3. Environment-specific collector endpoints (`dev`, `stage`, `uat`, `prod`).

### 4.2 Collector Pipelines
Receivers:
1. `otlp` required

Processors:
1. `memory_limiter`
2. `batch`
3. `resource`/`attributes` enrichment
4. optional `tail_sampling` in prod

Exporters:
1. metrics exporter path compatible with Prometheus
2. `loki`
3. `otlp` (Tempo)

### 4.3 Storage and Retention
1. `dev`: metrics 7d, logs 7d, traces 3d
2. `stage/uat`: metrics 15d, logs 15d, traces 7d
3. `prod`: metrics 30d+, logs 30d+, traces 14d+

## 5. Cross-Signal Contract

### 5.1 Required Labels
- `project`
- `service`
- `env`
- `region`
- `version`
- `team`

### 5.2 Logging Contract
Logs must include:
- `timestamp`, `level`, `msg`, `service`, `env`, `trace_id`, `span_id`, `request_id`, `route`, `status_code`, `latency_ms`

### 5.3 Metrics Contract
Required families:
1. HTTP RED metrics
2. Asynq job lifecycle metrics
3. Queue depth/backlog age metrics
4. DB query count/latency/error metrics

### 5.4 Tracing Contract
1. Inbound request starts/continues trace
2. Worker job starts/continues trace across async boundary
3. DB and outbound dependencies as child spans
4. Errors captured as span status + exception events

## 6. Job Observability Architecture

### 6.1 Lifecycle States
`queued -> started -> succeeded | failed | retried | dead_lettered`

### 6.2 Required Job Context
- `job_execution_id`
- `asynq_job_id`
- `task_type`
- `queue`
- `attempt`
- `trigger_source`
- `trace_id`
- optional `tenant_id`

### 6.3 Reliability Requirements
1. Idempotency key for side-effecting jobs
2. Exponential backoff with jitter
3. Max retries by task type
4. Dead-letter queue with replay audit trail

## 7. Alerting and SLO Architecture

### 7.1 Baseline Alerts
1. HTTP 5xx spike
2. HTTP p95 latency breach
3. readiness/service down
4. queue oldest age breach
5. retry storm
6. dead-letter growth
7. DB dependency failure

### 7.2 SLO Baseline
1. Availability SLO
2. Latency SLO
3. Job freshness SLO (async-heavy services)

Use burn-rate alerting for critical SLOs.

## 8. Security and Compliance
1. No secrets/PII in logs, metrics labels, or span attributes
2. Configurable redaction list
3. TLS/mTLS and auth for collector in non-local envs
4. RBAC on Grafana folders and datasources

## 9. Split-Ready Constraints
1. `module/` must not import `platform/`
2. `platform/` treats module as external contract
3. CI pipelines must be separable by directory
4. All shared contracts documented outside implementation details

## 10. Acceptance of This Architecture
Architecture is accepted when:
1. Pilot service shows correlated logs/metrics/traces in Grafana.
2. Job lifecycle is queryable end-to-end.
3. Baseline alerts and runbook links are active.
4. New service onboarding follows the contract without custom exceptions.

## 11. Baseline Implementation Snapshot (2026-03-24)
The following baseline is implemented in this repository:

1. Module API skeleton in `module/` with required public interfaces and env contract parser.
2. Platform baseline in `platform/`:
   1. OTEL Collector, Prometheus, Loki, Tempo, Grafana, Alertmanager, Redis.
   2. Collector pipeline with `otlp` receiver; `memory_limiter`, `batch`, `resource`, `attributes` processors; exporters to Prometheus/Loki/Tempo.
3. Alerts as code in `alerts/prometheus/baseline.rules.yml`.
4. Dashboards and provisioning as code in `dashboards/provisioning/` and `dashboards/json/`.
5. Example integration in `examples/`:
   1. Gin endpoints `/healthz`, `/readyz`, `/metrics`
   2. Asynq job route `POST /jobs/demo`
   3. Job lifecycle logs/metrics baseline.
6. Validation scripts in `scripts/`:
   1. `validate-platform.sh`
   2. `validate-alerts.sh`
   3. `validate-dashboards.sh`
   4. `platform-health.sh`
   5. `smoke-example.sh`

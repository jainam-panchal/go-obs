# Deployment Guide

## Purpose
This guide defines how to deploy:
1. The centralized observability platform (`platform/`).
2. Service integrations using `go-observability-kit` (`module/`).

It is written for pre-release development and can be used as the baseline for stage/prod rollout.

## Deployment Model
1. One LGTM stack per environment (`dev`, `stage`, `prod`).
2. Services send OTLP telemetry to the environment collector.
3. Dashboards, datasources, and alert rules are provisioned from this repository.

## Prerequisites
1. Docker and Docker Compose on the target host.
2. Go toolchain for module/test verification.
3. Environment variables for each service:
   1. `SERVICE_NAME`
   2. `SERVICE_VERSION`
   3. `DEPLOYMENT_ENV`
   4. `OTEL_EXPORTER_OTLP_ENDPOINT`
   5. Optional: `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG`

## Platform Deployment (Local/Dev)
1. Start stack:
```bash
make up
```
2. Validate platform:
```bash
./scripts/platform-health.sh
make lint
```
3. Stop stack:
```bash
make down
```

## Service Deployment Checklist
1. Service imports `module/` and initializes runtime with `bootstrap.Init`.
2. HTTP stack uses `ginx.Middleware`.
3. Endpoints are registered:
   1. `/healthz`
   2. `/readyz`
   3. `/metrics`
4. Workers use `workerx.AsynqMiddleware`.
5. GORM is wrapped with `dbx.WrapGORM`.
6. All required environment values are configured.

## Pre-Deploy Gates
Run before any environment promotion:
```bash
./scripts/test-race.sh
make lint
make test
make smoke
```

## Promotion Flow
1. Merge change to default branch.
2. Deploy or refresh platform configs for target environment.
3. Deploy updated service image/config pointing to target OTLP endpoint.
4. Run smoke checks:
   1. HTTP request appears in metrics/logs/traces.
   2. Worker job appears with lifecycle telemetry.
   3. Alerts and dashboards load without errors.

## Rollback Flow
1. Roll back service image/config to previous known-good release.
2. If platform config changed, roll back compose/config artifact revision.
3. Verify:
   1. `/healthz` and `/readyz` return healthy.
   2. No critical alert remains firing from rollback issue.
   3. Telemetry pipeline recovers.

## Post-Deploy Verification
1. Grafana query for `service=<name>, env=<env>` shows:
   1. `http_server_requests_total`
   2. `asynq_jobs_started_total`
   3. `db_client_queries_total`
2. Trace lookup by `trace_id` resolves HTTP and worker spans.
3. Alert rules evaluate with expected labels/annotations.

## Known Pre-Release Notes
1. Legacy readiness `Check{Fn: ...}` compatibility still exists for development, but migration to `FnCtx` is required before deprecation enforcement.
2. Use release notes under `docs/releases/` to track behavior changes before GA.

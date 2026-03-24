# Service Onboarding Guide (v1)

## Goal
Integrate a Go/Gin service with the observability contract in <= 1 working day.

## Prerequisites
1. Service can import `module/` API package.
2. Required env vars are available:
   - `SERVICE_NAME`, `SERVICE_VERSION`, `DEPLOYMENT_ENV`
   - `OTEL_EXPORTER_OTLP_ENDPOINT`
3. Local platform stack is available via `make up`.

## Integration Steps
1. Initialize runtime with `bootstrap.Init` at service startup.
2. Add `ginx.Middleware(rt)` to Gin middleware chain.
3. Register:
   - `health.RegisterRoutes(router, checks...)` for `/healthz` and `/readyz`
   - readiness checks must use `health.Check{Name: ..., FnCtx: func(context.Context) error { ... }}`
   - `metrics.RegisterRoute(router)` for `/metrics`
4. For workers, add `workerx.AsynqMiddleware(rt)`.
5. For GORM usage, wrap DB with `dbx.WrapGORM(db, rt, ...)`.

## Validation Steps
1. `make test`
2. `make lint`
3. Generate one request and verify `/healthz`, `/readyz`, `/metrics`.
4. Run `make smoke` for end-to-end baseline validation.

## Readiness Deprecation Deadline
1. Legacy readiness checks using `Check{Fn: ...}` are deprecated.
2. Migration deadline is **2026-06-30**.
3. After the deadline, CI enforces the policy via `scripts/check-readiness-fn-deprecation.sh`.

## Rollout Guidance
1. Start with one pilot service.
2. Verify alerts and dashboard usefulness for at least 2 weeks.
3. Expand integration to tier-1 services.

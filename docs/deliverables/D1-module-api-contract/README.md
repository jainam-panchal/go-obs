# D1 - Module API Contract Skeleton

## Scope
- Scaffold public API packages and signatures:
  - `bootstrap.Init(ctx, cfg) (*Runtime, error)`
  - `bootstrap.Shutdown(ctx, rt) error`
  - `ginx.Middleware(rt) gin.HandlerFunc`
  - `workerx.AsynqMiddleware(rt) asynq.MiddlewareFunc`
  - `dbx.WrapGORM(db, rt, opts...) *gorm.DB`
  - `health.RegisterRoutes(router, checks...)`
  - `metrics.RegisterRoute(router)`
- Implement env contract parsing (required vars + defaults).
- Define graceful telemetry failure behavior.

## Expected Outcome
- Module compiles with stable API skeleton.
- Env contract enforced and defaults applied.

## Gate
- `go test -race ./...` passes for module skeleton.
- API compile contract tests pass.

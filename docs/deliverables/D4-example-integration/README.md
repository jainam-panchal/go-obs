# D4 - Example Integration

## Scope
- Build minimal Gin service + Asynq worker in `examples/` using module APIs.
- Expose required endpoints and emit request/job telemetry.

## Expected Outcome
- `/healthz`, `/readyz`, `/metrics` are functional.
- HTTP and job lifecycle telemetry visible in local stack.

## Gate
- Smoke checks confirm logs, metrics, and traces end-to-end.

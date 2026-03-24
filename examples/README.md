# examples

Minimal Gin service + Asynq worker integration using `go-observability-kit` APIs.

## What it provides
- Required endpoints: `/healthz`, `/readyz`, `/metrics`
- Demo job enqueue route: `POST /jobs/demo`
- HTTP and job lifecycle structured logs with `trace_id`
- HTTP and Asynq lifecycle metrics on `/metrics`

## Run manually
1. `make up`
2. `cd examples && go run .`
3. `curl -X POST http://127.0.0.1:18080/jobs/demo`

## Smoke
- `make smoke`

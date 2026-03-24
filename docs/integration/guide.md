# Integration Guide

## Purpose
This guide shows how a Go/Gin service integrates with `go-observability-kit` end-to-end.

## Integration Flow
```mermaid
flowchart LR
  A[HTTP Request] --> B[ginx.Middleware]
  B --> C[Business Handler]
  C --> D[DB via dbx.WrapGORM]
  C --> E[Enqueue Asynq Task]
  E --> F[workerx.AsynqMiddleware]
  B --> G[/metrics]
  B --> H[/healthz /readyz]
  B --> I[OTLP Export]
  F --> I
  D --> I
  I --> J[OTEL Collector]
  J --> K[Prometheus]
  J --> L[Loki]
  J --> M[Tempo]
  K --> N[Grafana]
  L --> N
  M --> N
```

## 1. Bootstrap Runtime
```go
cfg, err := config.LoadFromEnv()
if err != nil { /* handle */ }

rt, err := bootstrap.Init(ctx, cfg)
if err != nil { /* handle */ }
defer bootstrap.Shutdown(context.Background(), rt)
```

## 2. Wire HTTP Middleware and Required Endpoints
```go
r := gin.New()
r.Use(ginx.Middleware(rt))

health.RegisterRoutes(r,
  health.Check{
    Name: "redis",
    FnCtx: func(ctx context.Context) error {
      return pingRedis(ctx)
    },
  },
)
metrics.RegisterRoute(r)
```

## 3. Wire Worker Middleware
```go
mux := asynq.NewServeMux()
mux.Use(workerx.AsynqMiddleware(rt))
mux.HandleFunc("task:type", func(ctx context.Context, t *asynq.Task) error {
  // business logic
  return nil
})
```

## 4. Wire GORM Instrumentation
```go
db, err := gorm.Open(...)
if err != nil { /* handle */ }

db = dbx.WrapGORM(db, rt)
```

## 5. Optional Queue Snapshot Hooks
Use these when you have queue inspector data:
```go
workerx.ObserveEnqueue(rt, queueName, taskType)
workerx.ObserveRetried(rt, queueName, taskType)
workerx.ObserveQueueSnapshot(rt, queueName, taskType, depth, oldestAgeSeconds, deadLetter)
```

## 6. Required Runtime Contract
1. Endpoints:
   1. `GET /healthz`
   2. `GET /readyz`
   3. `GET /metrics`
2. Required env:
   1. `SERVICE_NAME`
   2. `SERVICE_VERSION`
   3. `DEPLOYMENT_ENV`
   4. `OTEL_EXPORTER_OTLP_ENDPOINT`

## 7. Verification Flow
1. Send one HTTP request to a real route.
2. Enqueue one worker task.
3. Confirm in Grafana:
   1. HTTP metrics present.
   2. Worker lifecycle metrics present.
   3. Trace correlation by `trace_id`.
4. Validate alerts and dashboards:
```bash
make lint
```

## 8. Common Integration Mistakes
1. Missing `FnCtx` on readiness checks.
2. Registering raw dynamic URL path labels instead of route templates.
3. Sending payload-only IDs as metric labels (high cardinality).
4. Omitting queue snapshot instrumentation while alerting on queue metrics.

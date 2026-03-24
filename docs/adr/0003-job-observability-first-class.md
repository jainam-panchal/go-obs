# ADR 0003: Job Observability Is First-Class

## Status
Accepted

## Context
System behavior depends heavily on async jobs; request-only observability is insufficient.

## Decision
Treat job processing as first-class telemetry with mandatory lifecycle coverage.

Required state model:
1. `queued`
2. `started`
3. `succeeded`
4. `failed`
5. `retried`
6. `dead_lettered`

Required job context fields:
1. `job_execution_id`
2. `asynq_job_id`
3. `task_type`
4. `queue`
5. `attempt`
6. `trigger_source`
7. `trace_id`
8. optional `tenant_id`

## Consequences
Positive:
1. faster diagnosis of queue issues
2. measurable reliability for background processing
3. better operational accountability for retries/DLQ

Negative:
1. extra instrumentation and dashboard work
2. stricter schema governance required

## Follow-up
1. enforce job metrics/log fields in module contract tests
2. add queue backlog/retry/DLQ baseline alerts
3. create replay audit requirements

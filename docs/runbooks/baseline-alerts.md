# Baseline Alerts Runbook

## Purpose
This runbook covers response steps for baseline observability alerts defined in `alerts/prometheus/baseline.rules.yml`.

## Ownership
- Team: `platform`
- Alert labels include: `severity`, `team`, `summary`, `description`, `runbook_url`, `dashboard_url`

## General Triage Flow
1. Open Grafana and filter by `service` and `env`.
2. Correlate metrics with logs (`trace_id`) and traces.
3. Confirm impact window and blast radius.
4. Apply mitigation, then verify recovery and alert resolution.

## Alert Playbooks

### HTTP5xxRateSpike
1. Check `HTTP Overview` dashboard for request rate and 5xx trends.
2. Inspect recent deploy/version changes.
3. Correlate failing routes with logs and traces.
4. Roll back or traffic-shift if regression is confirmed.

### HTTPP95LatencyBreach
1. Check p95/p99 latency on `HTTP Overview` dashboard.
2. Identify slow routes and dependency spans.
3. Verify DB/query saturation and queue backlog effects.
4. Mitigate via scaling, throttling, or rollback.

### ServiceNotReady
1. Verify `/readyz` output and failing dependency checks.
2. Confirm dependency health (DB/queue/network).
3. Restart unhealthy instances and validate readiness.

### QueueOldestAgeBreach
1. Check `asynq_queue_oldest_age_seconds` and `asynq_queue_depth`.
2. Verify worker availability and retry activity.
3. Increase worker concurrency or reduce inflow temporarily.

### RetryStorm
1. Inspect `asynq_jobs_retried_total` by queue/task.
2. Identify failing task type and root-cause exception.
3. Stop bad producers or disable problematic task path.
4. Apply fix and monitor retry decay.
5. Alert semantics: counter rate (`rate(asynq_jobs_retried_total[5m])`) sustained for the `for` window.

### DeadLetterGrowth
1. Check dead-letter growth trend and affected task types.
2. Review failure signatures in worker logs.
3. Execute controlled replay after root cause is fixed.
4. Record replay actions in incident notes.
5. Alert semantics: gauge net-growth (`delta(asynq_dead_letter_total[30m]) > 0`) sustained for the `for` window.

### DBDependencyFailure
1. Inspect DB error-rate metrics and slow query signals.
2. Confirm DB reachability, pool saturation, and credentials.
3. Mitigate via failover/connection tuning/rollback.

## Validation After Mitigation
1. Alert condition remains below threshold for full `for` window.
2. No correlated critical alerts remain active.
3. Dashboards and logs/traces confirm stable behavior.

#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="/tmp/go-obs-example.log"
PID_FILE="/tmp/go-obs-example.pid"

cleanup() {
  if [ -f "$PID_FILE" ]; then
    pid="$(cat "$PID_FILE")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" 2>/dev/null || true
    fi
    rm -f "$PID_FILE"
  fi
}
trap cleanup EXIT

make up >/dev/null

cd examples
export SERVICE_NAME="example-service"
export SERVICE_VERSION="0.1.0"
export DEPLOYMENT_ENV="dev"
export OTEL_EXPORTER_OTLP_ENDPOINT="http://127.0.0.1:14317"
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
export REDIS_ADDR="127.0.0.1:16379"
export HTTP_ADDR=":18080"

: > "$LOG_FILE"
go run . > "$LOG_FILE" 2>&1 &
echo $! > "$PID_FILE"
cd ..

for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:18080/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS "http://127.0.0.1:18080/healthz" >/dev/null
curl -fsS "http://127.0.0.1:18080/readyz" >/dev/null
curl -fsS -X POST "http://127.0.0.1:18080/jobs/demo?tenant_id=t1" -H "X-Request-Id: smoke-1" -H "X-Trace-Id: trace-smoke-001" >/dev/null
sleep 4

metrics_output="$(curl -fsS http://127.0.0.1:18080/metrics)"

for metric in \
  http_server_requests_total \
  http_server_request_duration_seconds \
  http_server_inflight_requests \
  asynq_jobs_enqueued_total \
  asynq_jobs_started_total \
  asynq_jobs_succeeded_total \
  asynq_job_duration_seconds \
  asynq_queue_depth \
  asynq_queue_oldest_age_seconds \
  asynq_dead_letter_total; do
  echo "$metrics_output" | grep -q "$metric" || { echo "missing metric: $metric"; exit 1; }
done

grep -q '"msg":"job_queued"' "$LOG_FILE" || { echo "missing job_queued log"; exit 1; }
grep -q '"msg":"job_started"' "$LOG_FILE" || { echo "missing job_started log"; exit 1; }
grep -q '"msg":"job_succeeded"' "$LOG_FILE" || { echo "missing job_succeeded log"; exit 1; }
grep -q '"trace_id":"trace-smoke-001"' "$LOG_FILE" || { echo "missing trace_id in logs"; exit 1; }

echo "example smoke passed"

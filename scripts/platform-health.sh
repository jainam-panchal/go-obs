#!/usr/bin/env bash
set -euo pipefail

otel_health_port="${OTEL_HEALTH_PORT:-13133}"
prometheus_port="${PROMETHEUS_PORT:-19090}"
loki_port="${LOKI_PORT:-13100}"
tempo_port="${TEMPO_PORT:-13200}"
grafana_port="${GRAFANA_PORT:-13000}"
alertmanager_port="${ALERTMANAGER_PORT:-19093}"

checks=(
  "http://localhost:${otel_health_port}/|otel-collector"
  "http://localhost:${prometheus_port}/-/ready|prometheus"
  "http://localhost:${loki_port}/ready|loki"
  "http://localhost:${tempo_port}/ready|tempo"
  "http://localhost:${grafana_port}/api/health|grafana"
  "http://localhost:${alertmanager_port}/-/ready|alertmanager"
)

for check in "${checks[@]}"; do
  url="${check%%|*}"
  name="${check##*|}"
  ok=0
  for _ in $(seq 1 20); do
    if curl -fsS "$url" > /dev/null; then
      ok=1
      break
    fi
    sleep 1
  done
  if [ "$ok" -ne 1 ]; then
    echo "health check failed: $name ($url)"
    exit 1
  fi
  echo "healthy: $name"
done

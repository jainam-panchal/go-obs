#!/usr/bin/env bash
set -euo pipefail

compose_file="platform/docker-compose.yml"
collector_cfg="platform/otel-collector/config.yaml"

[ -f "$compose_file" ] || { echo "missing $compose_file"; exit 1; }
[ -f "$collector_cfg" ] || { echo "missing $collector_cfg"; exit 1; }

docker compose -f "$compose_file" config > /dev/null

required_patterns=(
  "receivers:"
  "otlp:"
  "processors:"
  "memory_limiter:"
  "batch:"
  "resource:"
  "attributes:"
  "pipelines:"
  "metrics:"
  "logs:"
  "traces:"
  "prometheus:"
  "loki:"
  "otlp/tempo:"
)

for p in "${required_patterns[@]}"; do
  if ! grep -q "$p" "$collector_cfg"; then
    echo "collector config missing required pattern: $p"
    exit 1
  fi
done

echo "platform validation passed"

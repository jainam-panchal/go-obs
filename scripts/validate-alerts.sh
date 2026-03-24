#!/usr/bin/env bash
set -euo pipefail

alerts_file="alerts/prometheus/baseline.rules.yml"
[ -f "$alerts_file" ] || { echo "missing $alerts_file"; exit 1; }

required=(severity team summary description runbook_url dashboard_url)
for key in "${required[@]}"; do
  if ! grep -q "${key}:" "$alerts_file"; then
    echo "missing required alert label: $key"
    exit 1
  fi
done

echo "alerts validation passed"

#!/usr/bin/env bash
set -euo pipefail

alerts_file="alerts/prometheus/baseline.rules.yml"
[ -f "$alerts_file" ] || { echo "missing $alerts_file"; exit 1; }

for label in severity team; do
  if ! grep -q "^[[:space:]]*${label}:" "$alerts_file"; then
    echo "missing required alert label: $label"
    exit 1
  fi
done

for ann in summary description runbook_url dashboard_url; do
  if ! grep -q "^[[:space:]]*${ann}:" "$alerts_file"; then
    echo "missing required alert annotation: $ann"
    exit 1
  fi
done

echo "alerts validation passed"

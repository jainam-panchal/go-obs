#!/usr/bin/env bash
set -euo pipefail

rules_test_file="alerts/tests/baseline.rules.test.yml"
[ -f "$rules_test_file" ] || { echo "missing $rules_test_file"; exit 1; }

if command -v promtool >/dev/null 2>&1; then
  promtool test rules "$rules_test_file"
  echo "alert rule fixtures passed"
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "promtool not found and docker not available to run alert rule fixtures"
  exit 1
fi

docker run --rm \
  --entrypoint promtool \
  -v "$(pwd):/workspace" \
  -w /workspace \
  prom/prometheus:v2.54.1 \
  test rules "$rules_test_file"

echo "alert rule fixtures passed"

#!/usr/bin/env bash
set -euo pipefail

deadline="${READINESS_FN_DEPRECATION_DEADLINE:-2026-06-30}"
today="${READINESS_CHECK_TODAY_OVERRIDE:-$(date -u +%F)}"

if [[ "$today" < "$deadline" ]]; then
  echo "readiness Fn deprecation gate inactive until $deadline (today: $today)"
  exit 0
fi

if ! command -v rg >/dev/null 2>&1; then
  echo "rg is required for readiness deprecation check"
  exit 1
fi

violations="$(rg -n --glob '*.go' --glob '!*module/health/routes.go' --glob '!*module/health/routes_test.go' 'Check\\{[^}]*Fn:' module examples || true)"
if [[ -n "$violations" ]]; then
  echo "legacy readiness Fn usage detected after deprecation deadline $deadline:"
  echo "$violations"
  exit 1
fi

echo "readiness Fn deprecation check passed"

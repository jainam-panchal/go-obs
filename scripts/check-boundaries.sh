#!/usr/bin/env bash
set -euo pipefail

if [ ! -d module ]; then
  echo "boundary check skipped: module directory missing"
  exit 0
fi

# Split-ready rule: module must not import platform code.
if (cd module && go list -deps ./... | grep -E '(^|/)platform($|/)') >/dev/null 2>&1; then
  echo "boundary check failed: module depends on platform"
  exit 1
fi

# Split-ready rule: no direct platform path references from module source.
if rg -n '(^|[^[:alnum:]_])platform/' module --glob '*.go' >/dev/null 2>&1; then
  echo "boundary check failed: module source references platform path"
  exit 1
fi

echo "boundary check passed"

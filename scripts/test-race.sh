#!/usr/bin/env bash
set -euo pipefail

if [[ -f "go.mod" ]]; then
  go test -race ./...
  exit 0
fi

if [[ -f "module/go.mod" ]]; then
  (cd module && go test -race ./...)
  exit 0
fi

echo "no go.mod found at root or module/"
exit 1

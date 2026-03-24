#!/usr/bin/env bash
set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-}"

if [ -n "$base_ref" ] && [ -n "$head_ref" ]; then
  changed_files="$(git diff --name-only "$base_ref" "$head_ref")"
else
  tracked="$(git diff --name-only HEAD)"
  untracked="$(git ls-files --others --exclude-standard)"
  changed_files="$(printf "%s\n%s\n" "$tracked" "$untracked" | sed '/^$/d' | sort -u)"
fi

if [ -z "$changed_files" ]; then
  echo "docs-first check: no changed files"
  exit 0
fi

requires_docs=0
while IFS= read -r f; do
  case "$f" in
    module/*|platform/*|alerts/*|dashboards/*)
      requires_docs=1
      break
      ;;
  esac
done <<< "$changed_files"

if [ "$requires_docs" -eq 0 ]; then
  echo "docs-first check: no significant product/runtime changes"
  exit 0
fi

has_docs_update=0
while IFS= read -r f; do
  case "$f" in
    docs/spec.md|docs/requirements.md|docs/adr/*.md)
      has_docs_update=1
      break
      ;;
  esac
done <<< "$changed_files"

if [ "$has_docs_update" -eq 1 ]; then
  echo "docs-first check passed"
  exit 0
fi

echo "docs-first check failed: significant changes detected without docs updates"
echo "Required: update docs/spec.md and/or docs/requirements.md and/or docs/adr/*.md"
exit 1

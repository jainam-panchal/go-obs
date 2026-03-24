# ADR 0001: Unified Repo Now, Split-Ready Design

## Status
Accepted

## Context
We need to deliver both a reusable module and central platform quickly, while preserving a clean path to split into two repos later.

## Decision
Use one repository now with strict boundaries:
1. `module/` for `go-observability-kit`
2. `platform/` for LGTM infrastructure
3. no `module -> platform` imports
4. independent CI pipelines by directory

## Consequences
Positive:
1. faster initial delivery
2. easier coordination while architecture stabilizes
3. split path remains low risk

Negative:
1. requires governance discipline to avoid coupling
2. may temporarily increase blast radius of repo-level changes

## Follow-up
1. enforce boundaries in CI
2. maintain contract docs outside implementation paths
3. revisit split timing after stable v1 adoption

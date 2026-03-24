# ADR 0005: Baseline Implementation and CI Enforcement

## Status
Accepted (2026-03-24)

## Context
The repository moved from documentation-only planning to executable baseline implementation (D0-D5). We needed:
1. A runnable local platform baseline.
2. A compilable module contract skeleton.
3. Alert/dashboard contracts as code.
4. Automated policy and quality gates in CI.

## Decision
1. Implement D0-D5 baseline directly in repo with split-ready boundaries.
2. Use top-level `alerts/` and `dashboards/` as source-of-truth, mounted into platform services.
3. Enforce policy in CI with dedicated workflows:
   - module tests
   - platform validations
   - docs-first policy
   - split-ready boundary checks
   - examples smoke validation

## Consequences
### Positive
1. Repo now has executable baseline for module/platform/examples.
2. Contracts are testable locally and in CI.
3. Policy checks reduce regressions and undocumented behavior changes.

### Tradeoffs
1. CI runtime increases due to smoke workflow.
2. Docs-first check requires disciplined docs updates in PRs.

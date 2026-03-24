# ADR 0004: Contract-Driven Development with CI Gates

## Status
Accepted

## Context
Without strict gates, observability contracts drift and multi-service consistency degrades.

## Decision
All changes must follow contract-first workflow:
1. update `docs/spec.md` and/or `docs/requirements.md` when behavior changes
2. add/update ADR for architectural or policy decisions
3. pass module/platform/consumer CI gates

Mandatory CI checks:
1. module tests + race + lint
2. collector/rules/dashboard validation
3. env contract and `/metrics` exposure checks for consumers
4. high-cardinality label policy checks

## Consequences
Positive:
1. predictable observability behavior across services
2. reduced regressions and faster onboarding

Negative:
1. slightly slower change throughput
2. requires ongoing maintenance of governance files

## Follow-up
1. build CI templates for downstream consumer repos
2. publish migration guides for contract changes

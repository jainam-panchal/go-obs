# D2 - Platform Baseline

## Scope
- Add local LGTM + OTEL Collector baseline in `platform/`.
- Include mandatory collector receiver/processors/export routes.

## Expected Outcome
- Stack boots via `make up`.
- Collector pipeline includes required baseline blocks.

## Gate
- Platform config validation passes.
- Stack health checks pass.

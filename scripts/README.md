# scripts

Validation, linting, and local bootstrap scripts.

## Current scripts
- `validate-platform.sh`: validates compose and collector baseline requirements.
- `platform-health.sh`: checks local stack health endpoints with retries.
- `validate-alerts.sh`: checks required alert labels/fields and runs alert rule fixtures.
- `validate-dashboards.sh`: checks dashboard/datasource provisioning files.
- `smoke-example.sh`: runs D4 example integration smoke checks.
- `test-alert-rules.sh`: executes `promtool test rules` against alert fixtures.
- `test-race.sh`: runs `go test -race ./...` at root or `module/` depending on repository layout.
- `check-readiness-fn-deprecation.sh`: enforces `FnCtx` readiness migration deadline policy.

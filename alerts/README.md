# alerts

Prometheus/Alertmanager rule sets.

## Baseline alerts
- HTTP 5xx rate spike
- HTTP p95 latency breach
- Service readiness down
- Queue oldest age breach
- Retry storm
- Dead-letter growth
- DB dependency failure

Rules are defined in `alerts/prometheus/baseline.rules.yml` and loaded by Prometheus from code.

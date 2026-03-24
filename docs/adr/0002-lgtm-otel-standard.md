# ADR 0002: Adopt Open-Source LGTM + OTEL as Standard

## Status
Accepted

## Context
We need a cost-effective, vendor-neutral observability stack across multiple services and environments.

## Decision
Adopt:
1. OpenTelemetry SDK + Collector for telemetry pipeline
2. Prometheus for metrics
3. Loki for logs
4. Tempo for traces
5. Grafana for visualization
6. Alertmanager for alert routing

## Consequences
Positive:
1. standardized cross-service telemetry contract
2. no hard vendor lock-in
3. lower cost profile at small/medium scale

Negative:
1. more operational ownership than SaaS-only approach
2. requires careful retention/capacity planning

## Follow-up
1. implement baseline stack in `platform/`
2. define retention/sampling by environment
3. enforce dashboard and alert provisioning as code

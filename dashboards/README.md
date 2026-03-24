# dashboards

Grafana provisioning and dashboard JSON as code.

## Provisioning
- Datasources: `dashboards/provisioning/datasources/datasources.yml`
- Dashboard providers: `dashboards/provisioning/dashboards/dashboards.yml`
- Baseline dashboards: `dashboards/json/*.json`

This directory is mounted directly into Grafana by `platform/docker-compose.yml`.

#!/usr/bin/env bash
set -euo pipefail

for file in \
  dashboards/provisioning/datasources/datasources.yml \
  dashboards/provisioning/dashboards/dashboards.yml \
  dashboards/json/http-overview.json; do
  [ -f "$file" ] || { echo "missing $file"; exit 1; }
done

grep -q 'datasources:' dashboards/provisioning/datasources/datasources.yml || { echo "datasources provisioning invalid"; exit 1; }
grep -q 'providers:' dashboards/provisioning/dashboards/dashboards.yml || { echo "dashboard provider invalid"; exit 1; }
grep -q '"uid": "http-overview"' dashboards/json/http-overview.json || { echo "baseline dashboard uid missing"; exit 1; }

echo "dashboards validation passed"

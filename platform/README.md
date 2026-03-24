# platform

Central observability platform configs (LGTM + OTEL Collector).

## Local stack
- Compose file: `platform/docker-compose.yml`
- Bring up: `make up`
- Bring down: `make down`
- Validate config: `./scripts/validate-platform.sh`
- Health check: `./scripts/platform-health.sh`

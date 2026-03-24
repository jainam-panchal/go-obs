.PHONY: up down test lint smoke

up:
	@docker compose -p go-obs-platform -f platform/docker-compose.yml rm -fsv >/dev/null 2>&1 || true
	@docker compose -p go-obs-platform -f platform/docker-compose.yml up -d --remove-orphans

down:
	@docker compose -p go-obs-platform -f platform/docker-compose.yml down --remove-orphans

test:
	@cd module && go test -race ./...

lint:
	@./scripts/validate-platform.sh
	@./scripts/validate-alerts.sh
	@./scripts/validate-dashboards.sh

smoke:
	@echo "[D0] Smoke tests are not wired yet. Planned in D4 (example integration)."

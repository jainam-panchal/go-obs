.PHONY: up down test lint smoke

up:
	@docker compose -p go-obs-platform -f platform/docker-compose.yml up -d --remove-orphans

down:
	@docker compose -p go-obs-platform -f platform/docker-compose.yml down --remove-orphans

test:
	@cd module && go test -race ./...

lint:
	@./scripts/validate-platform.sh

smoke:
	@echo "[D0] Smoke tests are not wired yet. Planned in D4 (example integration)."

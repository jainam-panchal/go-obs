.PHONY: up down test lint smoke

up:
	@echo "[D0] Stack startup is not wired yet. Planned in D2 (platform baseline)."

down:
	@echo "[D0] Stack shutdown is not wired yet. Planned in D2 (platform baseline)."

test:
	@cd module && go test -race ./...

lint:
	@echo "[D0] Lint checks are not wired yet. Planned in D5 (CI/CD enforcement)."

smoke:
	@echo "[D0] Smoke tests are not wired yet. Planned in D4 (example integration)."

# ══════════════════════════════════════════════════════════════════════════
# Happy Feet — workspace tasks
#
#   make dev        start everything (infra + API + storefront)
#   make stop       stop everything
#   make status     show what's up and where
#
# Individual pieces are available too — see the sections below.
# ══════════════════════════════════════════════════════════════════════════

BACKEND  := backend
FRONTEND := frontend
COMPOSE  := docker compose -f $(BACKEND)/compose.local.yml -p happyfeet
RUN_DIR  := .run

# GOENV is applied via `env` where the command is wrapped in nohup — `nohup
# VAR=x go ...` fails, because nohup treats the assignment as the program name.
GOENV := GOFLAGS=-mod=mod
GO    := $(GOENV) go

.DEFAULT_GOAL := help
.PHONY: help dev stop status infra-up infra-down infra-reset migrate migrate-down \
        seed psql api web api-fg web-fg kill-api kill-web install build test lint \
        typecheck check clean logs-api logs-web mail

# ── Everything ────────────────────────────────────────────────────────────
dev: infra-up migrate api web ## Start infra, API and storefront
	@echo ""
	@echo "  Storefront   http://localhost:3000"
	@echo "  API          http://localhost:8080/api/v1"
	@echo "  API docs     http://localhost:8080/swagger"
	@echo "  Mail capture http://localhost:8025"
	@echo ""
	@echo "  Logs:  make logs-api | make logs-web        Stop:  make stop"

stop: kill-api kill-web infra-down ## Stop API, storefront and infra

# `go run` execs a compiled binary as a *child*; killing the recorded pid leaves
# that child holding the port, and the next start then fails with "address in
# use" — into a log nobody reads. So kill whatever actually owns the port, and
# use the pidfile only as a fallback.
kill-api: ## Stop the API (by listening port, then by pidfile)
	@-pids=`lsof -ti tcp:8080 -sTCP:LISTEN 2>/dev/null`; \
	  [ -n "$$pids" ] && kill $$pids 2>/dev/null && echo "✓ API stopped" || true
	@-[ -f $(RUN_DIR)/api.pid ] && kill `cat $(RUN_DIR)/api.pid` 2>/dev/null || true
	@rm -f $(RUN_DIR)/api.pid

kill-web: ## Stop the storefront
	@-pids=`lsof -ti tcp:3000 -sTCP:LISTEN 2>/dev/null`; \
	  [ -n "$$pids" ] && kill $$pids 2>/dev/null && echo "✓ Storefront stopped" || true
	@-[ -f $(RUN_DIR)/web.pid ] && kill `cat $(RUN_DIR)/web.pid` 2>/dev/null || true
	@rm -f $(RUN_DIR)/web.pid

status: ## Show service health
	@printf "postgres   "; docker inspect -f '{{.State.Status}}' happyfeet-postgres-1 2>/dev/null || echo "down"
	@printf "redis      "; docker inspect -f '{{.State.Status}}' happyfeet-redis-1 2>/dev/null || echo "down"
	@printf "mailpit    "; docker inspect -f '{{.State.Status}}' happyfeet-mailpit-1 2>/dev/null || echo "down"
	@printf "api        "; curl -fsS localhost:8080/readyz 2>/dev/null || echo "down"; echo
	@printf "storefront "; curl -fsS -o /dev/null -w '%{http_code}\n' localhost:3000 2>/dev/null || echo "down"

# ── Infrastructure ────────────────────────────────────────────────────────
infra-up: ## Start postgres, redis and the local mail capture
	@# --wait blocks on the healthchecks. Without it `up -d` returns as soon as
	@# the containers start and the migration step races Postgres's first accept.
	@$(COMPOSE) up -d --wait
	@echo "✓ postgres :5433  redis :6380  mailpit :1025 (UI :8025)"

infra-down: ## Stop the containers (data volumes are kept)
	@$(COMPOSE) down
	@echo "✓ infra stopped"

infra-reset: ## Stop and DELETE the database volume
	@$(COMPOSE) down -v
	@echo "✓ infra stopped, volumes removed"

mail: ## Open the captured-mail inbox
	@open http://localhost:8025

# ── Database ──────────────────────────────────────────────────────────────
migrate: ## Apply pending migrations
	@cd $(BACKEND) && $(GO) run ./cmd/migrate -direction up

migrate-down: ## Roll back one migration
	@cd $(BACKEND) && $(GO) run ./cmd/migrate -direction down -steps 1

seed: ## Load the demo catalogue
	@docker exec -i happyfeet-postgres-1 psql -U happyfeet -d happyfeet < $(BACKEND)/migrations/seed_products.sql
	@echo "✓ catalogue seeded"

psql: ## Open a database shell
	@docker exec -it happyfeet-postgres-1 psql -U happyfeet -d happyfeet

# ── Services ──────────────────────────────────────────────────────────────
# Detach fully: nohup plus `< /dev/null` releases the terminal's stdin, so make
# (and any CI or agent wrapper around it) returns instead of waiting on a pipe
# the long-running child still holds open.
api: kill-api ## Start the Go API in the background (restarts if already running)
	@mkdir -p $(RUN_DIR)
	@cd $(BACKEND) && nohup env $(GOENV) go run ./cmd/api > ../$(RUN_DIR)/api.log 2>&1 < /dev/null & \
		echo $$! > $(RUN_DIR)/api.pid
	@$(call wait_for,localhost:8080/readyz,20,API,$(RUN_DIR)/api.log)
	@echo "✓ API        http://localhost:8080"

web: kill-web ## Start the storefront in the background (restarts if already running)
	@mkdir -p $(RUN_DIR)
	@cd $(FRONTEND) && nohup npm run dev > ../$(RUN_DIR)/web.log 2>&1 < /dev/null & \
		echo $$! > $(RUN_DIR)/web.pid
	@$(call wait_for,localhost:3000,40,Storefront,$(RUN_DIR)/web.log)
	@echo "✓ Storefront http://localhost:3000"

# wait_for <url> <attempts> <name> <logfile> — poll until the service answers,
# then fail loudly with the tail of its log rather than a bare non-zero exit.
define wait_for
	for i in $$(seq 1 $(2)); do \
		curl -fsS -o /dev/null $(1) 2>/dev/null && exit 0; \
		sleep 1; \
	done; \
	echo "✗ $(3) did not come up — last lines of $(4):"; tail -25 $(4); exit 1
endef

api-fg: ## Run the API in the foreground
	@cd $(BACKEND) && $(GO) run ./cmd/api

web-fg: ## Run the storefront in the foreground
	@cd $(FRONTEND) && npm run dev

logs-api: ; @tail -f $(RUN_DIR)/api.log
logs-web: ; @tail -f $(RUN_DIR)/web.log

# ── Quality ───────────────────────────────────────────────────────────────
install: ## Install frontend dependencies
	@cd $(FRONTEND) && npm install

build: ## Production build of both halves
	@cd $(BACKEND) && $(GO) build -o /dev/null ./... && echo "✓ API builds"
	@cd $(FRONTEND) && npm run build

test: ## Run the Go test suite
	@cd $(BACKEND) && $(GO) test ./... -count=1

lint: ## Vet the Go code
	@cd $(BACKEND) && $(GO) vet ./...

typecheck: ## Typecheck the storefront
	@cd $(FRONTEND) && npx tsc --noEmit && echo "✓ no type errors"

check: lint test typecheck ## Everything a PR should pass

clean:
	@rm -rf $(RUN_DIR) $(FRONTEND)/.next
	@cd $(BACKEND) && $(GO) clean -cache

help:
	@echo "Happy Feet — make <target>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

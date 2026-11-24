# Short commands for local development. Run `make help` for the list.
# Backend targets load backend/.env; docker targets use the root .env.

SHELL := /bin/bash
COMPOSE     := docker compose
COMPOSE_DEV := docker compose -f compose.yml -f compose.dev.yml
LOAD_ENV    := set -a && source backend/.env && set +a
GO          := GOTOOLCHAIN=local go

.PHONY: help
help: ## Show available commands
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# --- Full stack in Docker -------------------------------------------------------
.PHONY: up down logs ps seed-docker
up: ## Build and start the whole stack behind Nginx (migrations run automatically)
	$(COMPOSE) up -d --build

down: ## Stop the stack (data volume is kept)
	$(COMPOSE) down

logs: ## Follow logs of all services
	$(COMPOSE) logs -f --tail=100

ps: ## Show service status
	$(COMPOSE) ps

seed-docker: ## Load demo data into the Docker database (manual step)
	$(COMPOSE) run --rm seed

# --- Host development -----------------------------------------------------------
.PHONY: outbox admin-create
outbox: ## Show the latest local test codes (API on the host)
	@$(LOAD_ENV) && tail -n 5 "$${CONTACT_LOCAL_DIR:-$${TMPDIR:-/tmp}/qurylys-outbox}/outbox.jsonl"

admin-create: ## Create an administrator: make admin-create CONTACT=... NAME="..." (password is asked)
	@$(LOAD_ENV) && read -rsp 'Password: ' P && echo && cd backend && printf '%s' "$$P" | $(GO) run ./cmd/admin create -contact "$(CONTACT)" -name "$(NAME)"

.PHONY: db-up db-down migrate-up migrate-down migrate-status seed api web
db-up: ## Start only PostgreSQL, published on 127.0.0.1:$POSTGRES_DEV_PORT
	$(COMPOSE_DEV) up -d --wait postgres

db-down: ## Stop PostgreSQL
	$(COMPOSE_DEV) stop postgres

migrate-up: ## Apply migrations to DATABASE_URL from backend/.env
	$(LOAD_ENV) && cd backend && $(GO) run ./cmd/migrate up

migrate-down: ## Roll back the last migration
	$(LOAD_ENV) && cd backend && $(GO) run ./cmd/migrate down

migrate-status: ## Show migration status
	$(LOAD_ENV) && cd backend && $(GO) run ./cmd/migrate status

seed: ## Load demo data into DATABASE_URL (idempotent)
	$(LOAD_ENV) && cd backend && $(GO) run ./cmd/seed

api: ## Run the API on the host (http://localhost:8081)
	$(LOAD_ENV) && cd backend && $(GO) run ./cmd/api

web: ## Run the Next.js dev server (http://localhost:3100)
	cd frontend && npm run dev

# --- Checks ---------------------------------------------------------------------
.PHONY: test-db test test-unit check-backend check-frontend check
test-db: ## (Re)create the empty test database qurylys_test
	$(COMPOSE_DEV) exec -T postgres sh -c 'dropdb -U "$$POSTGRES_USER" --if-exists qurylys_test && createdb -U "$$POSTGRES_USER" qurylys_test'

test-unit: ## Go unit tests (no database)
	cd backend && $(GO) test ./...

test: test-db ## Go unit + integration tests against TEST_DATABASE_URL
	$(LOAD_ENV) && cd backend && $(GO) test -count=1 ./...

check-backend: ## gofmt, vet, build
	cd backend && test -z "$$(gofmt -l .)" && $(GO) vet ./... && $(GO) build ./...

check-frontend: ## Lint, typecheck and production build of the frontend
	cd frontend && npm run lint && npm run typecheck && npm run build

check: check-backend check-frontend test ## Everything above

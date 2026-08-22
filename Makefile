COMPOSE ?= docker compose
BACKEND_GOCACHE ?= $(CURDIR)/.gocache

.DEFAULT_GOAL := help

.PHONY: help build up down restart ps logs backend-logs db-logs db-shell test backend-test web-build extension-build clean

help:
	@printf '%s\n' \
		'Targets:' \
		'  make build            Build Docker images' \
		'  make up               Start backend, postgres, and mailpit' \
		'  make down             Stop Docker services' \
		'  make restart          Restart Docker services' \
		'  make ps               Show Docker service status' \
		'  make logs             Tail all Docker logs' \
		'  make backend-logs     Tail backend logs' \
		'  make db-logs          Tail Postgres logs' \
		'  make db-shell         Open psql in the Postgres container' \
		'  make test             Run backend tests' \
		'  make web-build        Build web app' \
		'  make extension-build  Build Chrome extension' \
		'  make clean            Stop services and remove local build caches'

build:
	$(COMPOSE) build

up:
	$(COMPOSE) up -d backend

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) up -d --build backend

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f

backend-logs:
	$(COMPOSE) logs -f backend

db-logs:
	$(COMPOSE) logs -f postgres

db-shell:
	$(COMPOSE) exec postgres psql -U clever -d clever_consumer

test: backend-test

backend-test:
	cd backend && GOCACHE=$(BACKEND_GOCACHE) go test ./...

web-build:
	cd web/clever-consumer && pnpm build

extension-build:
	cd extension/chrome && pnpm build

clean:
	$(COMPOSE) down --remove-orphans
	rm -rf .gocache .gomodcache .pnpm-store

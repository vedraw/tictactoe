SHELL := /bin/zsh
COMPOSE_ENV := .env.local

.PHONY: local-env up down ps nakama-health frontend-dev frontend-lint frontend-build

local-env:
	@if [ ! -f "$(COMPOSE_ENV)" ]; then cp .env.local.example $(COMPOSE_ENV); fi

.PHONY: frontend-install logs

up: local-env
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml up -d

down:
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml down

ps:
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml ps

nakama-health:
	curl --fail http://127.0.0.1:7350/

frontend-dev:
	cd frontend && npm run dev

frontend-install:
	cd frontend && npm install

frontend-lint:
	cd frontend && npm run lint

frontend-build:
	cd frontend && npm run build

logs:
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml logs -f nakama

SHELL := /bin/zsh
COMPOSE_ENV := .env.local

.PHONY: local-env up up-build down ps nakama-health smoke-http-rpc simulate-match simulate-match-full backend-test backend-vet frontend-dev frontend-lint frontend-build

local-env:
	@if [ ! -f "$(COMPOSE_ENV)" ]; then cp .env.local.example $(COMPOSE_ENV); fi

.PHONY: frontend-install logs

# Start Postgres + Nakama (reuse images; use up-build after backend Go changes).
up: local-env
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml up -d

# Rebuild Nakama image so backend.so is recompiled from current tree.
up-build: local-env
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml up -d --build

down:
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml down

ps:
	docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml ps

# Waits for Nakama HTTP + Go runtime (plugin RPC). Plain GET / can return an empty TCP reply while the server is still booting.
# Automated HTTP checks for all four RPCs (validation, success paths, rate limits). ~40s (includes sleeps).
smoke-http-rpc:
	python3 infra/smoke_http_rpc.py

# Live Nakama: two sockets (requires `ws` + `npm install` in frontend/). ~55s default, ~90s with timed forfeit.
simulate-match:
	cd frontend && npm run simulate

simulate-match-full:
	cd frontend && RUN_TIMED_FORFEIT=1 npm run simulate

nakama-health:
	@for i in $$(seq 1 30); do \
	  if curl -sfS --max-time 5 "http://127.0.0.1:7350/v2/rpc/list_leaderboard?http_key=defaulthttpkey&unwrap" \
	    -H 'Content-Type: application/json' -d '{}' >/dev/null; then \
	    echo "nakama ready (list_leaderboard RPC)"; \
	    exit 0; \
	  fi; \
	  echo "waiting for nakama ($$i/30)..."; \
	  sleep 2; \
	done; \
	echo "nakama did not respond in time. Check logs:"; \
	echo "  docker compose --env-file $(COMPOSE_ENV) -f infra/docker-compose.local.yml logs --tail=100 nakama"; \
	exit 1

backend-test:
	cd backend && GOTOOLCHAIN=local go test ./... -count=1

backend-vet:
	cd backend && GOTOOLCHAIN=local go vet ./...

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

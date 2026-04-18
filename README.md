# tictactoe

Server-authoritative tic-tac-toe on **Nakama** (Go runtime plugin) with a **React** client.

## Repo layout

- `backend/` — Nakama Go plugin (`backend.so`), local and prod Nakama YAML snippets.
- `frontend/` — Vite + React client.
- `infra/` — Docker Compose for local dev and production-style single-host deploy.
- `docs/` — architecture, protocol, and manual test notes ([`docs/testing.md`](docs/testing.md)).

## Local development

1. Install Docker.
2. From the repo root: `docker compose -f infra/docker-compose.local.yml up -d --build`
3. Frontend: see [`frontend/README.md`](frontend/README.md).
4. Smoke checks: `make nakama-health`, `make smoke-http-rpc`, and [`docs/testing.md`](docs/testing.md).

## Production deploy (single EC2 + Docker)

Target shape: **one x86_64 Linux host** with Docker, **Postgres + Nakama** in Compose, **only port 7350** published (HTTP + WebSocket). Do **not** expose Postgres **5432** or Nakama console **7351** to the internet.

### 1. Host prep

- Open EC2 security group inbound **TCP 7350** (clients) and **TCP 22** from your IP (admin), or use SSM without SSH.
- Install Docker Engine and the Compose plugin on the instance.

### 2. Code and secrets on the server

```bash
git clone <your-repo-url> tictactoe
cd tictactoe
cp .env.production.example .env.production
# edit .env.production — set POSTGRES_PASSWORD and NAKAMA_SERVER_KEY (and ideally NAKAMA_HTTP_KEY)
```

### 3. Start stack

```bash
docker compose -f infra/docker-compose.prod.yml --env-file .env.production up -d --build
```

### 4. Verify

From your laptop (replace host and HTTP key if you changed `NAKAMA_HTTP_KEY`):

```bash
curl -sS "http://YOUR_PUBLIC_IP:7350/v2/rpc/list_leaderboard?http_key=YOUR_HTTP_KEY&unwrap"
```

See also [`docs/testing.md`](docs/testing.md) for broader checks once the server is up.

### 5. Rollback / stop

```bash
docker compose -f infra/docker-compose.prod.yml --env-file .env.production down
```

Data lives in the Docker volume `postgres_data_prod` until you remove it with `docker volume rm` (destructive).

### Frontend (Vercel) later

- Browsers on **https** need **https/wss** to Nakama or they hit mixed-content rules; plan TLS (reverse proxy + cert, or tunnel) before pointing production Vercel at this host.
- Set `VITE_NAKAMA_HOST`, `VITE_NAKAMA_PORT`, `VITE_NAKAMA_SSL`, and `VITE_NAKAMA_SERVER_KEY` to match this deployment; configure Nakama **CORS / allowed origins** for your Vercel URLs when you add TLS.

### Optional ops todos

- Tighten SSH to your IP only; keep **5432** and **7351** off `0.0.0.0/0`.
- **Elastic IP** if you want a stable public address (small ongoing public IPv4 cost; not required for the assignment).

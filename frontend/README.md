# Tic-tac-toe frontend (Vite + React)

## Prerequisites

1. **Backend running locally** — Postgres + Nakama with the Go plugin, from the repo root:

   ```bash
   make up-build
   make nakama-health
   ```

2. **Install deps** (once):

   ```bash
   make frontend-install
   ```

## Dev server

From the repo root:

```bash
make frontend-dev
```

Defaults in `src/App.jsx` talk to Nakama at `127.0.0.1:7350` with `defaultkey`, matching local compose. Override with a `frontend/.env.local` file (see `.env.example`).

## Quality checks

```bash
make frontend-lint
make frontend-build
```

See `docs/testing.md` for multiplayer smoke checks after the stack is up.

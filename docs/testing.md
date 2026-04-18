# Testing Guide

## Backend layout

The Nakama Go plugin is split for clarity and testability:

- `backend/cmd/plugin/` — `package main` entrypoint only: `InitModule`, RPC registration, and thin wrappers that delegate to `internal/tictactoe`.
- `backend/internal/tictactoe/` — authoritative match logic, RPC handlers (behind small interfaces for unit tests), persistence, rate limiting, and envelopes.

The Docker image builds `backend.so` with:

```bash
cd cmd/plugin && go build --trimpath --buildmode=plugin -o /out/backend.so .
```

(from module root copied into the builder stage; see `backend/Dockerfile`).

## Backend unit tests

From the module root:

```bash
cd backend
GOTOOLCHAIN=local go test ./... -count=1
```

Packages:

- `tictactoe-backend/internal/tictactoe` — unit tests for envelopes and mode parsing, RPC rate limiting, board helpers, pure move/leave/timed-turn logic, match lifecycle, persistence, and RPC handlers using fakes for `MatchCreate` / `MatchList` / `LeaderboardRecordsList`.
- `tictactoe-backend/cmd/plugin` — no tests; compile-only entry for the plugin binary.

What is intentionally **not** covered by unit tests:

- Full `runtime.NakamaModule` behavior (only the narrow persistence and RPC surfaces are faked).
- Cross-process or multi-node rate limiter semantics (limiter is in-memory per Nakama process).

## Local stack (Postgres + Nakama)

From the **repository root** (after `make local-env` or copying `.env.local.example` to `.env.local`):

```bash
make up-build
```

Rebuilds the Nakama image so `backend.so` matches your tree. For a quicker start when nothing changed, use `make up` (no image rebuild).

Equivalent raw compose:

```bash
docker compose -f infra/docker-compose.local.yml --env-file .env.local up -d --build
```

### Production-style (EC2 / single host)

See the repository root [`README.md`](../README.md) for `infra/docker-compose.prod.yml`, `.env.production.example`, and firewall notes. After the stack is up, smoke RPC from your laptop (set `http_key` to match `NAKAMA_HTTP_KEY` in `.env.production`):

```bash
curl -sS "http://YOUR_PUBLIC_HOST:7350/v2/rpc/list_leaderboard?http_key=defaulthttpkey&unwrap" \
  -H 'Content-Type: application/json' \
  -d '{}'
```

The compose file builds from the repository root with `dockerfile: backend/Dockerfile` and loads `backend.so` into Nakama’s `runtime.path`.

Version alignment notes:

- Nakama `3.22.0` should use `github.com/heroiclabs/nakama-common` `v1.32.0` (see Heroic Labs Go runtime compatibility).
- Local Go development should use `GOTOOLCHAIN=local` so your machine does not auto-download a newer toolchain than the Nakama release expects.
- On Apple Silicon, Docker may need `DOCKER_BUILDKIT=0` for reliable image pulls/builds; `infra/docker-compose.local.yml` pins `linux/amd64` for the Nakama/pluginbuilder images.

### If `curl` prints “Empty reply from server” (curl 52)

Nakama may still be running migrations or loading the Go plugin right after `docker compose up`. Wait a few seconds and retry, or use `make nakama-health` (it polls for up to ~60s). If it never succeeds, inspect the container: `make logs` or `docker compose ... logs nakama` (look for runtime/plugin panics or database errors).

HTTP smoke (server-to-server RPC, no user session):

```bash
curl -sS "http://127.0.0.1:7350/v2/rpc/list_leaderboard?http_key=defaulthttpkey&unwrap" \
  -H 'Content-Type: application/json' \
  -d '{}'
```

Avoid relying on `curl --fail http://127.0.0.1:7350/` alone; some startup windows can close the connection before an HTTP response is sent.

To run a full **HTTP-only** scenario suite (all RPCs, validation errors, and per-route rate limits with proper timing), use:

```bash
make smoke-http-rpc
```

(`infra/smoke_http_rpc.py` — takes about 40 seconds because it waits out rate-limit windows between throttle tests.)

### Live match simulation (socket, not unit tests)

End-to-end **authoritative match** checks (two device-authenticated players, real `joinMatch` / `sendMatchState` / `leaveMatch`) live in:

```bash
make simulate-match
```

This runs `frontend/scripts/match-sim.mjs` via `npm run simulate`. It needs:

- Nakama up (`make up-build` or equivalent).
- `npm install` in `frontend/` (adds the `ws` dev dependency used as a **Node WebSocket** implementation).

Scenarios covered by `npm run simulate` (default):

- Two-player activation (`waiting` → `active`)
- Move errors: `NOT_YOUR_TURN`, `INVALID_PAYLOAD`, `INVALID_POSITION`, `CELL_OCCUPIED`
- Top-row win and full-board draw
- Sole leaver abandon (match disappears from `list_matches` for the `waiting|active` query)
- Disconnect win (active → other leaves)
- `find_match` pairs two users onto the same `matchId`
- Leaderboard reflects a win after a finished game (`list_leaderboard`)
- Bounded `OPCODE_STATE` traffic while idle after `finished` (docs item 6 style check)
- Two concurrent matches stay isolated on the board
- Unknown match opcode does not mutate state
- `RATE_LIMIT_RPC` on the **11th** `create_match` in one window for an **authenticated** user
- `RATE_LIMIT_MOVE` (two parallel moves same tick)

Full run including **timed forfeit** (~32s wait + server tick):

```bash
make simulate-match-full
# or:  cd frontend && RUN_TIMED_FORFEIT=1 npm run simulate
```

**Note:** `list_matches` uses Nakama label search (`+label.mode:<mode> +label.status:/(waiting|active)/`). If the query form is wrong for your server build, open rooms can incorrectly show as empty even when a player is waiting—rebuild the plugin (`make up-build`) after backend changes. Right after `create_match` there can still be a short indexing delay before a room appears.

Nakama 3.22 may still print startup lines for its embedded JavaScript runtime *provider*; that is expected server wiring, not custom JS, and it does not load project modules unless `index.js` is present.

Deferred Phase 3 follow-ups: [`phase3-deferred-checklist.md`](phase3-deferred-checklist.md).

## Frontend manual checks

Prereq: Nakama up (e.g. `make up-build`), UI via `make frontend-dev` or `npm run build` + `npm run preview`.

- **Connect:** lobby phase strip shows **Lobby ready** when realtime is online; **Quick Match**, **Create Room**, and **Join** (listed rooms) enabled; **Refresh Rooms** / **Refresh Leaderboard** work with session (HTTP RPC).
- **Realtime offline** (network drop or Nakama stopped): lobby strip and in-match **banner** show offline; **Reconnect** available; Quick Match / Create / Join / cells disabled until socket is live; disabled controls expose reason via **`title`**; list refresh can still run without a socket.
- **Matchmaking:** phase strip during **Quick Match** or **Create Room**; mode **select** disabled for that window.
- **RPC error** (e.g. `RATE_LIMIT_RPC`): message in lobby phase strip (`role="alert"`).
- **Match banner** (`role="status"`): **waiting**, **active** (your turn vs opponent), **finished** (winner or draw), **abandoned**; meta row: match id, your mark, mode; invalid moves → error line under the board.
- **Timed, waiting:** preamble only — no countdown until match is **active**.
- **Timed, active:** turn clock from server **deadline**; **≤5s** low-time styling; **0s** at-zero styling until next state; caption states forfeiture is **server-driven**.
- **Narrow (~375px):** no horizontal scroll on lobby / match / leaderboard; ~44px tap targets on primary actions, **Tabs**, auth **Connect**, cells; **Tab** → visible **focus-visible** outline; mode **select** not clipped; long ids wrap in meta.
- **Safe area:** with `viewport-fit=cover`, shell padding respects `env(safe-area-inset-*)` on notched devices.

## Local smoke checks (behavior)

After `go test ./...` succeeds, validate core multiplayer behavior manually:

1. Two players join same match and observe `waiting -> active`.
2. Submit invalid move (wrong turn or occupied cell) and confirm move rejected.
3. Complete game and confirm `finished` with winner or draw.
4. Leave a waiting room as sole player and confirm `abandoned`.
5. Trigger invalid move or malformed payload and confirm FE receives `ok: false` with `error.code` and `error.message`.
6. After a match reaches `finished`, keep the match open briefly and confirm no repeated idle `OPCODE_STATE` spam is emitted.
7. Verify `matchLabelUpdate` is emitted on real status transitions only (e.g., `waiting -> active`, `active -> finished`) and not for repeated same-status ticks/joins.
8. Rapidly call `create_match`, `find_match`, and `list_matches` from the same user and confirm throttle envelope with `error.code = RATE_LIMIT_RPC` after limits are exceeded.
9. Send 2 move attempts from the same user in one tick and confirm second attempt is rejected with `error.code = RATE_LIMIT_MOVE`.

## Risk checklist (anti-abuse)

Use this checklist during testing/review to track known anti-abuse limitations:

- [ ] In-memory limiter scope is acceptable for current environment (single instance/local).
- [ ] `ctx.userId` is consistently present in expected RPC paths; fallback keying is not causing false throttles.
- [ ] Fixed-window burst behavior at boundary is acceptable for current UX.
- [ ] RPC limits do not create false positives during normal room refresh/matchmaking use.
- [ ] Move throttle (`1` attempt per user per tick) is acceptable for client retry behavior.
- [ ] Limiter map growth is monitored (no unbounded stale key accumulation in long runs).
- [ ] Throttle error payloads include actionable details for FE handling.
- [ ] Frontend shows throttle messages clearly (`RATE_LIMIT_RPC`, `RATE_LIMIT_MOVE`).
- [ ] Restart behavior (counters reset) is acceptable for assignment scope.

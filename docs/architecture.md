# Backend Runtime Architecture

## Authoritative Boundary

The Nakama Go plugin (`backend/cmd/plugin` + `backend/internal/tictactoe`, built as `backend.so`) is server-authoritative:

- Clients send intent (`OPCODE_MOVE`) only.
- Backend validates move intent and mutates canonical match state.
- Backend broadcasts authoritative state (`OPCODE_STATE`) to all participants.

## Logic split

Move processing is intentionally split into two layers in Go (mostly under `internal/tictactoe`):

- Deterministic game/state transitions:
  - board validation and turn ownership checks
  - move application, win/draw detection, next-turn resolution
  - status transitions (`waiting`, `active`, `finished`, `abandoned`)
- Runtime side-effect orchestration:
  - match and RPC registration in `InitModule` (`cmd/plugin`)
  - error/system/state broadcasts
  - label updates for matchmaking discovery
  - leaderboard/stat persistence and anti-abuse limiting

This boundary keeps gameplay behavior deterministic and easier to test without a full Nakama module stub. RPC paths use small interfaces (`MatchCreate` / `MatchList` / `LeaderboardRecordsList`) for unit tests; match persistence uses a minimal `nkPersistence` surface plus `NoopPersistence` when tests pass a nil `NakamaModule` (production always supplies a real module).

## Deployment surface

- Production and local configs ship **only** the Go plugin (`backend.so`) under Nakama’s `runtime.path`.
- **Do not** add `index.js` (or set `runtime.js_entrypoint`) unless you intentionally reintroduce JS runtime code; otherwise Nakama’s JS loader skips project modules.
- Nakama 3.22 still starts its embedded JS/Lua runtime *machinery*; that is separate from shipping custom `.js` logic in this repository.

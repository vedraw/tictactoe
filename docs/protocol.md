# Multiplayer Protocol Contract

Backend implementation note: protocol contracts are unchanged for clients, but game logic runs in the Nakama Go plugin (`backend/cmd/plugin` + `backend/internal/tictactoe`, compiled to `backend.so`). There is no project JavaScript under the Nakama `runtime.path` (no `index.js`, no `js_entrypoint`). Nakama 3.22 may still log initialization of its embedded JavaScript runtime *provider*; that is not this repo’s code and loads no modules unless `index.js` exists.

## Match State Schema (Server Authoritative)

All realtime state updates (`OPCODE_STATE = 2`) must follow this canonical object shape:

- `mode` (`"classic" | "timed"`): game mode selected at match creation.
- `board` (`string[9]`): board cells, each `""`, `"X"`, or `"O"`.
- `players` (`Array<{ userId: string, username: string, presence: object }>`): currently connected participants.
- `playerSymbols` (`Record<string, "X" | "O">`): symbol ownership by `userId`.
- `turnUserId` (`string`): user id whose turn is active, or empty when not active.
- `winner` (`string`): winning user id, empty string on draw/not finished.
- `winnerName` (`string`): winning username, empty string on draw/not finished.
- `status` (`"waiting" | "active" | "finished" | "abandoned"`): lifecycle status.
- `moveCount` (`number`): validated moves applied to board.
- `turnDeadlineSec` (`number`): unix epoch second deadline for timed turns, `0` for classic or inactive.
- `turnDurationSec` (`number`): per-turn duration, `30` for timed mode and `0` for classic.
- `endedAtTick` (`number`): match tick when terminal state was reached; `0` before terminal state.

## Allowed Status Transitions

The backend enforces the following transition graph:

- `waiting -> active`
  - Trigger: second distinct player joins a waiting match.
- `waiting -> abandoned`
  - Trigger: all players leave before match activation.
- `active -> finished`
  - Trigger: win, draw, timeout forfeit, disconnect win, or server terminate.

Disallowed transitions (for example `waiting -> finished`, `finished -> active`) are rejected by transition guard logic.

## Match Label Contract

Match labels stay aligned with lifecycle and must be queryable by:

- `mode`: `"classic"` or `"timed"`
- `status`: `"waiting" | "active" | "finished" | "abandoned"`

This supports RPC filtering for room discovery and matchmaking.

## Unified Error Contract

All error responses are standardized to:

- `{ ok: false, error: { code: string, message: string, details?: object } }`

All successful RPC responses are standardized to:

- `{ ok: true, data: object }`

`list_leaderboard` success `data` includes:

- `leaderboard`: array of `{ rank, username, score, wins, losses, draws, streak }` (counts parsed from Nakama record metadata).
- `cursor`: Nakama’s **next** pagination cursor string (empty when there is no next page). This maps to the runtime `LeaderboardRecordsList` next-cursor return value.

This envelope is used for:

- realtime match errors sent via `OPCODE_ERROR`
- RPC responses returned by backend runtime functions

### Stable Error Codes

- `INVALID_PAYLOAD`
- `INVALID_SENDER`
- `MATCH_NOT_ACTIVE`
- `NOT_YOUR_TURN`
- `INVALID_POSITION`
- `CELL_OCCUPIED`
- `PLAYER_NOT_IN_MATCH`
- `MATCH_FULL`
- `MATCH_FINISHED`
- `MATCH_INACTIVE`
- `INVALID_MODE`
- `RATE_LIMIT_RPC`
- `RATE_LIMIT_MOVE`

The web client maps these codes to short player-facing strings in [`frontend/src/errorCopy.js`](../frontend/src/errorCopy.js) (extend that registry when adding codes).

## Anti-Abuse Limits

Default lightweight limits:

- RPC window: `10` seconds per user per route.
- `create_match`: max `10` calls per window.
- `find_match`: max `20` calls per window.
- `list_matches`: max `30` calls per window.
- Match move attempts: max `1` move attempt per user per tick.

When exceeded, backend returns standard envelope with:

- RPC throttle: `RATE_LIMIT_RPC`
- Match move throttle: `RATE_LIMIT_MOVE`

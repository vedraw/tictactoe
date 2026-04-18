# Agent Operating Guide

This repository contains a server-authoritative multiplayer Tic-Tac-Toe implementation.

## System Boundaries

- `frontend/`: React web client, UI flow and realtime socket interaction.
- `backend/cmd/plugin` + `backend/internal/tictactoe`: Nakama Go plugin runtime (authoritative match + RPC APIs).
- `infra/`: Docker Compose — [`infra/docker-compose.local.yml`](infra/docker-compose.local.yml) for dev, [`infra/docker-compose.prod.yml`](infra/docker-compose.prod.yml) for single-host production.
- `docs/`: architecture, protocol, and operational decisions.

## Non-Negotiable Rules

- Keep game rules authoritative in Nakama only.
- Do not trust client board state or turn ownership.
- Preserve RPC names and opcodes unless protocol docs are updated together.
- Any gameplay behavior change must include docs updates and test notes.

## Implementation Constraints

- Keep backend runtime Go code compatible with the pinned Nakama + `nakama-common` versions (see `backend/go.mod`).
- Prefer pure helper functions for game rules to improve testability.
- Keep state shape backward compatible for connected clients.
- Match labels must remain queryable by `mode` and `status`.

## Quality Gates Before Commit

1. `make backend-test` (and `make backend-vet` when changing Go code)
2. `make smoke-http-rpc` (with local Nakama up) when changing RPC / rate limits
3. `make simulate-match` (or `make simulate-match-full` to include timed forfeit) when changing match logic
4. `make frontend-lint`
5. `make frontend-build`
6. Smoke checklist from `docs/testing.md` updated (if behavior changed)

## Review Expectations

- Explain architectural intent in PR description, not just code diff.
- Call out security/cheat-prevention impact for gameplay changes.
- Include rollback approach for backend runtime changes.

## AI-First Workflow

- Use `.cursor/skills/context7-execute-local/SKILL.md` for library/framework/tool-specific work so implementation is grounded in current docs before code changes.

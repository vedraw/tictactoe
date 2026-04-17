# Agent Operating Guide

This repository contains a server-authoritative multiplayer Tic-Tac-Toe implementation.

## System Boundaries

- `frontend/`: React web client, UI flow and realtime socket interaction.
- `backend/modules/`: Nakama runtime code (authoritative match + RPC APIs).
- `infra/`: local environment and containerized services.
- `docs/`: architecture, protocol, and operational decisions.

## Non-Negotiable Rules

- Keep game rules authoritative in Nakama only.
- Do not trust client board state or turn ownership.
- Preserve RPC names and opcodes unless protocol docs are updated together.
- Any gameplay behavior change must include docs updates and test notes.

## Implementation Constraints

- Keep backend runtime JavaScript compatible with Nakama JavaScript runtime.
- Prefer pure helper functions for game rules to improve testability.
- Keep state shape backward compatible for connected clients.
- Match labels must remain queryable by `mode` and `status`.

## Quality Gates Before Commit

1. `make frontend-lint`
2. `make frontend-build`
3. Smoke checklist from `docs/testing.md` updated (if behavior changed)

## Review Expectations

- Explain architectural intent in PR description, not just code diff.
- Call out security/cheat-prevention impact for gameplay changes.
- Include rollback approach for backend runtime changes.

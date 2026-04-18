---
name: tech-task-execution
description: Execute dependency or framework-specific implementation tasks using current documentation from Context7 and the repository's existing architecture. Use when a change depends on third-party APIs, SDKs, CLI flags, config syntax, or versioned behavior, and produce a repo-aware execution plan before editing.
---

# Tech Task Execution

Use this skill for implementation tasks where correctness depends on current external docs and local codebase constraints.

## When this skill applies

Apply this skill when the task involves:

- Library/framework APIs (React, Nakama runtime APIs, Vite, ESLint, Docker, etc.)
- Dependency upgrades or migration work
- CLI/config syntax for tooling and infrastructure
- Any behavior that may have changed across versions

If the task is pure internal logic with no third-party dependency surface, this skill is optional.

## Required workflow

### 0) Hard gate: Plan mode + approval before execution

- MUST switch to Plan mode before doing any implementation work for this skill.
- In Plan mode, produce the execution plan and explicitly ask for user approval.
- Do not edit files, run implementation commands, or apply patches until the user approves.
- If Plan mode cannot be entered, stop and ask the user how to proceed.

### 1) Identify the tech dependency surface

- List the dependencies/tools involved.
- Determine likely versions from repo files (`package.json`, lockfiles, configs).
- Note ambiguity if versions are unclear.

### 2) Fetch latest relevant docs from Context7

- Retrieve current, task-specific docs for each dependency/tool.
- Focus on exact syntax/contracts required for this task (API signatures, config keys, CLI flags, migration notes).
- Prefer official patterns over blog-style alternatives.

### 3) Build a repo-aware execution plan before editing

Produce a concise plan with:

- `Doc findings`: facts from Context7 that drive decisions
- `Impact map`: files/modules likely affected in this repo
- `Execution steps`: smallest safe implementation sequence
- `Verification`: commands/checks to run after changes
- `Risk notes`: compatibility or migration risk and mitigation

### 4) Implement using smallest safe diff

- Follow the plan and keep changes scoped.
- Reuse existing architecture patterns in the repo.
- Avoid broad rewrites unless explicitly requested.

### 5) Verify and sync docs

Run relevant checks, preferring:

1. `make frontend-lint`
2. `make frontend-build`
3. Targeted tests for changed logic
4. Smoke checks from `docs/testing.md` for behavior changes

Then update docs if behavior/contracts/setup changed:

- `README.md`
- `docs/architecture.md`
- `docs/protocol.md`
- `docs/testing.md`

### 6) Report in a fixed output format

- `Context7 sources consulted`
- `Plan followed`
- `Changes made`
- `Verification results`
- `Open risks / next actions`

## Guardrails

- Always start in Plan mode for this skill before implementation.
- Never execute the plan without explicit user consent.
- If implementation starts without prior Plan mode + consent, stop immediately and report the violation.
- Do not skip Context7 for framework/library/tool-specific tasks.
- Do not silently introduce breaking changes or major version jumps.
- Ask for confirmation before destructive migrations.
- Keep repo-facing prose technically neutral (no unrelated sponsor or employer identifiers).

## Repo-specific constraints

- Keep gameplay rules server-authoritative in Nakama runtime.
- Preserve RPC names/opcodes unless protocol docs are updated in the same change.

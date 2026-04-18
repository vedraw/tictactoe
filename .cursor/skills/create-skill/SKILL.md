---
name: create-skill
description: Create or refine project skills for this repository with clear triggers, concise instructions, and repo-specific guardrails. Use when adding a new skill, updating an existing skill, or turning a repeated workflow into a reusable `.cursor/skills/*/SKILL.md`.
---

# Create Skill

Use this skill to create practical, maintainable project skills for this repository.

## Goal

Produce high-signal skills that are easy to trigger, short enough to stay useful, and aligned with repo constraints.

## When to use this skill

Use when:

- The user asks to add or improve a skill in `.cursor/skills/`
- A repeated workflow should become reusable instructions
- Existing skill names/descriptions are unclear and under-trigger

Do not use when the task is a one-off and does not need reusable workflow guidance.

## Required output location

- Project skills only: `.cursor/skills/<skill-name>/SKILL.md`
- Do not use global/internal folders for this repo task.

## Skill authoring workflow

### 1) Capture intent

Define:

- What problem the skill solves
- Exact trigger contexts (phrases/tasks where it should be invoked)
- Expected output behavior or format
- Required guardrails (safety, compatibility, docs/tests expectations)

### 2) Design the skill metadata first

- `name`: lowercase, hyphen-separated, specific, short
- `description`: include both WHAT it does and WHEN to use it
- Make description explicit enough to avoid under-triggering

### 3) Draft concise instructions

Use clear imperative steps. Prefer:

- A short “when this applies” section
- A required workflow checklist
- Repo-specific guardrails
- Expected response/report format

Avoid long generic theory, environment-specific assumptions, or external eval framework references.

### 4) Align with repository constraints

When relevant, enforce:

- Server-authoritative gameplay boundaries
- Protocol/doc sync for RPC/opcode or behavior changes
- Local verification steps (`make frontend-lint`, `make frontend-build`, tests/smoke as applicable)

### 5) Validate before finalizing

Checklist:

- Skill is discoverable from `description`
- Instructions are actionable and not bloated
- No references to unavailable external tooling
- Path references are correct for this repo
- Content is compatible with current project workflow

## Recommended structure template

```markdown
---
name: <skill-name>
description: <what it does + when to use it>
---

# <Title>

## When this skill applies
- ...

## Required workflow
1) ...
2) ...

## Guardrails
- ...

## Output format
- ...
```

## Maintenance rules

- If a skill becomes too broad, split into focused skills.
- If a skill is no longer used, remove or archive it.
- Keep names stable unless the intent changes materially.


---
name: writing-plans
source: builtin
description: Write file-level, bite-sized implementation plans before coding. Use when you have a spec or multi-step requirements and need an actionable plan before touching implementation.
license: MIT
compatibility: Requires read_file, grep, glob; write only on Default/Team when the user asks to save the plan file
metadata:
  author: danmo-work
  version: "1.0"
  category: coding
  adapted_from: "https://github.com/obra/superpowers/tree/main/skills/writing-plans"
  upstream_license: MIT
---

# Writing Plans

> Adapted from obra/superpowers `writing-plans` (© Jesse Vincent / contributors, MIT).
> Rewritten for Danmo Work tools and agents; not a verbatim copy.

Write plans that a skilled engineer with little repo context can execute: exact files, concrete steps, tests, and commands. Prefer DRY, YAGNI, TDD, frequent commits.

## Workflow

1. **Explore** — map relevant files with `glob` / `grep` / `read_file`. Follow existing patterns.
2. **Scope check** — if the work spans independent subsystems, propose separate plans (each delivers testable value).
3. **File map** — list create/modify/test paths and each file’s responsibility before tasks.
4. **Tasks** — bite-sized units with their own test cycle; each ends in an independently verifiable deliverable.
5. **Self-review** — spec coverage, no placeholders, consistent names/types across tasks.
6. **Handoff**
   - **Planner (read-only):** always deliver the complete plan in the assistant reply (session event stream). Do not call `write` or claim a file was saved.
   - **Default / Team:** deliver in chat by default; only `write` a plan file when the user explicitly asks to save it. Then ask whether to implement.

## Plan header (required)

```markdown
# [Feature] Implementation Plan

**Goal:** one sentence
**Architecture:** 2–3 sentences
**Tech stack:** key pieces
**Constraints:** version floors, naming, platforms (verbatim from spec)
```

## Task template

```markdown
### Task N: [Name]

**Files:**
- Create: `path`
- Modify: `path` (why)
- Test: `path`

**Steps:**
- [ ] Write failing test (show code or exact assertion)
- [ ] Run test — expect FAIL for reason X (exact command)
- [ ] Minimal implementation (show code or precise edit)
- [ ] Run test — expect PASS (exact command)
- [ ] Commit message suggestion
```

## No placeholders

Never leave: TBD/TODO, “add appropriate validation”, “write tests for the above” without actual tests, “similar to Task N”, or steps without exact paths/commands.

## Anti-patterns

- Vague “update the service layer” without files
- Giant tasks that mix unrelated deliverables
- Plans that assume unspoken domain knowledge
- Skipping self-review for type/name drift across tasks

## Stop condition

Deliver the complete plan in chat (and path only if Default/Team saved a file). Offer to implement next with Single-Agent or Multi-Agent mode; do not start coding inside this skill unless the user already approved execution.

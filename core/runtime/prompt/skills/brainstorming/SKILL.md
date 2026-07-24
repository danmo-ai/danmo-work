---
name: brainstorming
description: Clarify intent, requirements, and design before building. Use before creative work — new features, components, behavior changes, or non-trivial workplace deliverables — when requirements are ambiguous.
license: MIT
compatibility: Requires read_file, grep, glob; web_search optional
metadata:
  author: danqing-teams
  version: "1.0"
  category: work
  adapted_from: "https://github.com/obra/superpowers/tree/main/skills/brainstorming"
  upstream_license: MIT
---

# Brainstorming

> Adapted from obra/superpowers `brainstorming` (© Jesse Vincent / contributors, MIT).
> Rewritten for Danmo Work tools and agents; not a verbatim copy.

Turn fuzzy ideas into an approved design before implementation. Do not write production code or scaffolds until the user approves the design (a short design is fine for small work).

## Workflow

1. **Explore context** — relevant files, docs, recent patterns via `read_file` / `grep` / `glob`.
2. **Scope** — if the request spans independent subsystems, help decompose; brainstorm one sub-project at a time.
3. **Clarify** — call `ask_user` **one question at a time** (prefer `options` / multiple choice). Cover purpose, constraints, success criteria. Never ask in a plain message.
4. **Approaches** — propose 2–3 options with trade-offs; lead with a recommendation.
5. **Design** — present architecture/components/data flow/errors/testing scaled to complexity; get approval (section by section if large).
6. **Spec** — if the user wants a durable artifact, write a concise design doc with `write` (path they prefer, or a sensible project docs path).
7. **Self-review** — no TBD, no contradictions, unambiguous requirements, right-sized scope.
8. **Next** — hand off to `writing-plans` for implementation planning when the work is software; otherwise proceed to the appropriate Work skill.

## Anti-patterns

- “Too simple for a design” — still state the approach and get a yes
- Asking five questions in one message (or asking in plain text instead of `ask_user`)
- Jumping to code, file scaffolds, or `writing-plans` before approval
- Unrelated refactor proposals that do not serve the goal
- Vague specs with placeholders

## Stop condition

User-approved design (and optional saved spec). Explicitly offer the next step (usually writing-plans). Do not implement inside this skill.

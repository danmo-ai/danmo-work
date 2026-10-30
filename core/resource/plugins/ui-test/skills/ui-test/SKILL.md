---
name: ui-test
source: builtin
description: >-
  Author and run Web UI test specs as Markdown. Execute via browser_*
  (preferred) or computer (headed Linux desktop browser). Produce PASS/FAIL
  reports under ui-tests/.
license: MIT
compatibility: >-
  Requires browser_* tools; computer optional when runtime.computer.enabled.
  Linux desktop sandbox with Chromium/Chrome/Edge. Vision model needed for
  computer screenshots and browser_screenshot evidence.
metadata:
  author: danmo
  version: "0.1.0"
  category: research
---

# UI Test (Web, agent-driven)

Design Markdown specs, execute them on the **Linux desktop sandbox** with
`browser_*` / `computer`, and write a run report. This is **not** Playwright
codegen and **not** general-purpose browsing (use the `browser` expert for that).

## When to use vs other experts

| Need | Expert / skill |
|------|----------------|
| Systematic Web UI cases + PASS/FAIL | **This skill** (`ui-test`) |
| One-off multi-step web ops (no suite) | `browser` expert |
| Arbitrary desktop app GUI | `computer` expert |
| Unit/integration code tests | `implementer` + TDD |

## Layout in the project

| Path | Purpose |
|------|---------|
| `ui-tests/specs/*.md` | Specs (source of truth) |
| `ui-tests/runs/<YYYYMMDD-HHMMSS>/report.md` | Run report |
| `ui-tests/runs/<…>/*.png` (optional) | Evidence screenshots |

If the project already has a test folder convention, follow it; otherwise use the
paths above. Seed new specs from `assets/templates/spec.md`.

## Workflow

1. **Clarify target** — base URL (or proxy/raw URL), scope, auth constraints.
2. **Author / update specs** — happy path + edge + negative when relevant.
   Read `references/case-design.md` for structure and verbs.
3. **Plan** — list scenarios; use `todowrite` when ≥3 scenarios.
4. **Execute** — follow `references/execution.md`:
   - Default engine: `browser_*` (read_skill `browser` for ref/act details).
   - Escalate to `computer` only when headed/visual/canvas or browser unavailable
     (read_skill `computer-use` for see→act→verify).
5. **Oracle** — each `verify` / `verify not` must cite snapshot text, URL, or
   screenshot evidence. Search `kb-ui-test` for landscape/oracle notes if unsure.
6. **Report** — write `ui-tests/runs/<stamp>/report.md` with per-scenario
   PASS/FAIL, then return the expert output format.
7. **Cleanup** — `browser_close` unless the goal asks to keep the session.

## Hard rules

- Web only. Refuse Windows/native desktop product testing as out of scope.
- No `file://`. Convert to `/raw/` or `/proxy/` URLs (see `browser` skill).
- No `exec_shell` for GUI driving when `browser_*` or `computer` is available.
- Do not invent snapshot refs; only use refs from the latest navigate/act/snapshot.
- Do not loop the same failing action unchanged more than once.
- Sensitive actions need `ask_user` unless the goal authorizes them.

## Stop condition

Specs and report are on disk (when authoring/running was requested), every run
scenario has PASS or FAIL with evidence, and the structured expert report is ready.

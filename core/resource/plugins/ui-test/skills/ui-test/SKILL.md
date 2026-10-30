---
name: ui-test
source: builtin
description: >-
  From code changes in the user's Danmo Work project, auto-author Markdown UI
  specs under ui-tests/, then execute them with browser_* (preferred) and
  computer (headed Linux desktop) and report PASS/FAIL. Not for testing Danmo
  Work itself; not Playwright codegen.
license: MIT
compatibility: >-
  Requires browser_* tools; computer optional when runtime.computer.enabled.
  Linux desktop sandbox with Chromium/Chrome/Edge. Vision model needed for
  computer screenshots and browser_screenshot evidence. exec_shell only for
  read-only git / reachability — never for GUI driving.
metadata:
  author: danmo
  version: "0.2.0"
  category: research
---

# UI Test (Web, change-driven + agent-executed)

For **the user's project** opened in Danmo Work: inspect **code changes**,
**auto-write** Markdown UI specs, then **automate** those scenarios with
`browser_*` and (when needed) `computer` on the Linux desktop sandbox. Produce
PASS/FAIL evidence under `ui-tests/`.

This is **not** Playwright/Cypress codegen, **not** a suite for the Danmo Work
app itself, and **not** general browsing (use the `browser` expert for ad-hoc
ops without a report).

## When to use vs other experts

| Need | Expert / skill |
|------|----------------|
| Diff/PR → UI specs → PASS/FAIL run | **This skill** (`ui-test`) |
| One-off multi-step web ops (no suite) | `browser` expert |
| Arbitrary desktop app GUI (not a Web UI test deliverable) | `computer` expert |
| Unit/integration code tests | `implementer` + TDD |

## Layout in the project

| Path | Purpose |
|------|---------|
| `ui-tests/specs/*.md` | Specs (source of truth, next to product code) |
| `ui-tests/runs/<YYYYMMDD-HHMMSS>/report.md` | Run report |
| `ui-tests/runs/<…>/*.png` (optional) | Evidence screenshots |

If the project already has a test folder convention, follow it; otherwise use the
paths above. Seed new specs from `assets/templates/spec.md`.

## Workflow (default)

1. **Change-driven authoring** — follow `references/from-changes.md`:
   discover git/working-tree (or goal-named) diffs → map UI impact → write/update
   Markdown specs under `ui-tests/specs/`.
2. **Clarify target** — base URL for the **user's app** (`/proxy/…` or
   `/raw/…`). Ask if the app is not reachable.
3. **Plan** — list scenarios; use `todowrite` when ≥3 scenarios.
4. **Execute** — follow `references/execution.md`:
   - Default engine: **`browser_*`** (read_skill `browser` for ref/act details).
   - Escalate to **`computer`** for headed/visual/canvas or when browser is
     unavailable (read_skill `computer-use` for see→act→verify).
5. **Oracle** — each `verify` / `verify not` must cite snapshot text, URL, or
   screenshot evidence. Search `kb-ui-test` for landscape/oracle notes if unsure.
6. **Report** — write `ui-tests/runs/<stamp>/report.md` with per-scenario
   PASS/FAIL, then return the expert output format.
7. **Cleanup** — `browser_close` unless the goal asks to keep the session.

If the goal already supplies complete specs and only asks to run them, skip to
step 2–7. If it only asks to author specs, stop after specs are on disk (note
that in SUMMARY).

## Hard rules

- Web only. Refuse Windows/native desktop product testing as out of scope.
- Specs and runs belong in the **user project**, not the Danmo Work source tree
  (unless that tree *is* the user's opened project).
- No `file://`. Convert to `/raw/` or `/proxy/` URLs (see `browser` skill).
- No `exec_shell` for GUI driving when `browser_*` or `computer` is available.
  `exec_shell` is limited to read-only git, reachability probes, or starting a
  project dev script the goal already relies on.
- Do not invent snapshot refs; only use refs from the latest navigate/act/snapshot.
- Do not loop the same failing action unchanged more than once.
- Sensitive actions need `ask_user` unless the goal authorizes them.

## Stop condition

Change-derived specs (when authoring was in scope) and the run report (when
execution was in scope) are on disk; every executed scenario has PASS or FAIL
with evidence; the structured expert report is ready.

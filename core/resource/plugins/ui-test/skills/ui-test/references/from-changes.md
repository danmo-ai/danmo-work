# From code changes → UI specs (auto-author)

Primary product path: **inspect what changed in the user's project → write or
update Markdown specs that cover the UI impact → then execute** (see
`execution.md`). Do not invent Playwright projects.

## When this path applies

| Goal signal | Action |
|-------------|--------|
| "根据这次改动写/跑 UI 测试", "from this diff", "regression for my PR" | **Change-driven** (this doc) |
| Explicit URL + hand-written scenarios only | Skip discovery; author/run as given |
| Pure API / non-UI diff | Note in BLOCKERS / SUMMARY — out of scope for UI Test |

## Discover the change set

Prefer evidence over guessing. In order:

1. **Goal-supplied paths / PR / commit range** — use those first.
2. **`exec_shell` git (read-only)** — e.g. `git status --short`,  
   `git diff --stat`, `git diff` / `git diff main...HEAD` (or the base the goal
   names). Do **not** commit, push, reset, or rewrite history.
3. **`glob` / `grep` / `read_file`** — open the touched UI surfaces:
   routes, pages, components, forms, i18n strings, CSS that affects layout.

If the working tree is clean and the goal names no range, `ask_user` for the
base ref or the feature URL — do not invent a change set.

## Map changes → UI behaviors

Build a short **impact list** (keep in `todowrite` when ≥3 items):

| Change kind (examples) | Spec focus |
|------------------------|------------|
| New/edited route or page | Happy path navigate + primary CTA |
| Form / validation / submit | Happy + invalid/empty + error banner `verify not`/`verify` |
| Button / menu / dialog copy or wiring | Click intent + resulting URL or visible text |
| Auth / permission gate | Allowed path + forbidden path (`verify not`) |
| List / table / filter | Seed data assumption + filter/sort verify |
| Layout / visual-only / canvas | Prefer `Engine: computer` (+ screenshot oracle) |
| Pure refactor, no user-visible delta | Skip or one smoke navigate — say so in SUMMARY |

Ignore generated lockfiles, pure types, and backend-only files unless the goal
ties them to a visible UI contract.

## Auto-author / update specs

1. Seed from `assets/templates/spec.md`.
2. One **area** per file: `ui-tests/specs/<area>-<feature>.md` (kebab-case).
3. Fill metadata:
   - `Change-source:` commit range, PR, or `working-tree` + key paths
   - `Target:` running app URL (`/proxy/…`) or project `/raw/…` page
   - `Engine: browser` by default; `computer` when impact list says visual/canvas
4. For each impact item, add **≥1 scenario** (happy). Add edge/negative when
   forms, auth, or destructive actions appear in the diff.
5. Steps must use verbs from `case-design.md` and assert **checkable** oracles
   derived from the new UI (labels, routes, empty states) — not "looks fine".
6. **Update** existing specs when the same area already has a file; do not
   duplicate scenarios — edit in place with `edit` / `apply_patch`.

## Handoff to execution

After specs are on disk:

1. Confirm the app is reachable (proxy URL or ask user to start the dev server;
   `exec_shell` only to probe/`curl -I` or start a **project** script the goal
   already uses — never to drive the GUI).
2. Follow `execution.md`: run every new/updated scenario with **`browser_*`**
   first; escalate to **`computer`** per engine rules.
3. Write `ui-tests/runs/<stamp>/report.md` and stop with the expert output format.

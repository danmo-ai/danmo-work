# Execution (Linux desktop sandbox)

Run **after** specs exist (hand-written or auto-authored from
`from-changes.md`). Tools run inside the Danmo Linux desktop VM. Prefer
structured **browser use** (`browser_*`); use **computer use** (`computer`)
when the case needs a real headed window or vision on non-semantic UI.

## Engine selection

| Situation | Engine |
|-----------|--------|
| Normal DOM / SPA forms, links, dialogs | **`browser_*`** (default) |
| Need headed Chromium on the desktop, layout/visual check, canvas/WebGL, or `browser_*` unavailable | **`computer`** |
| Unit tests / API-only | Stop — not this skill |

Never drive the GUI with `exec_shell` when `browser_*` or `computer` is bound.

## browser_* path (a11y-first)

1. `read_skill(path="browser")` if you need act/ref details.
2. `browser_navigate(url=…)` → title, URL, snapshot with refs `e1`, `e2`, …
3. Map each intent step (`click` / `type` / …) to a **ref from the latest**
   snapshot. Call `browser_act`. Use the **returned** snapshot for the next step.
4. `verify` / `verify not`: judge against the latest snapshot text and/or URL.
   Do not pass without citing that evidence in the report.
5. `browser_screenshot` for human evidence when layout matters or a11y refs are
   insufficient — not as the sole oracle when text/URL would suffice.
6. End with `browser_close` unless the goal keeps the session.

Local targets: convert `file://` and bare project paths to
`http://127.0.0.1:7801/api/v1/projects/<projectId>/raw/<path>` or
`http://127.0.0.1:7801/api/v1/proxy/http/<host>-<port>/<path>`.

## computer path (headed desktop browser)

Use only when `Engine: computer` or browser path is blocked / insufficient.

1. `read_skill(path="computer-use")`.
2. Ensure a browser window exists: launch Chromium/Chrome/Edge via desktop
   patterns the environment supports, then `list_windows` / `focus_window`.
3. Loop **see → act → verify**: `screenshot` → click/type/key → `screenshot`.
4. Address bar: prefer `ctrl+l`, type URL, `Return`, then `wait` and re-screenshot.
5. Assertions: state what is visible in the latest screenshot (`read_image` if
   needed). Record FAIL if the vision model cannot see images.
6. If `runtime.computer.enabled` is false or desktop tools are missing, record
   BLOCKER — do not invent clicks.

## Per-scenario protocol

1. Reset to the scenario entry URL (do not assume prior scenario left a clean page).
2. Execute steps in order; on first hard failure, mark **FAIL**, capture evidence,
   continue to the next scenario unless the goal says stop-on-fail.
3. Do not retry the same failed act with identical parameters more than once;
   change strategy (new snapshot, different ref, or escalate engine) once.

## Report file

Write `ui-tests/runs/<YYYYMMDD-HHMMSS>/report.md`:

```markdown
# UI Test Run <stamp>

Target: …
Engine: browser | computer | mixed
Specs: …

## Results

| ID | Scenario | Verdict | Evidence |
|----|----------|---------|----------|
| S1 | … | PASS | URL=…; snapshot contains "…" |
| S2 | … | FAIL | expected "…"; got "…" |

## Notes
…
```

Overall exit narrative: all PASS → success; any FAIL → failed run; blockers
listed separately.

---
name: browser
source: builtin
description: >-
  Operate a sticky browser tab with navigate / snapshot / act / screenshot /
  close. Use for multi-step web UI interaction; prefer web_fetch for one-shot
  readable page extraction.
license: MIT
compatibility: >-
  Local Chrome/Edge/Chromium (auto-launched headless) or runtime.browser.cdp_url
  attach; vision model optional for browser_screenshot.
metadata:
  author: danmo
  version: "0.1.0"
  category: research
---

# Browser (interactive)

You own multi-step browser interaction for the parent agent. The tab is sticky
for the Danmo session until `browser_close`, idle reclaim (~15 min), or session
delete.

## When to use vs web_fetch

| Need | Tool / expert |
|------|----------------|
| Extract readable article / docs text once | Parent: `web_fetch` (or `researcher`) |
| Click, fill forms, multi-page SPA state | This skill + `browser_*` |
| REST/JSON API | Parent: `http_request` |

## Engine modes

- **Launch (default):** no `cdp_url` → local Chrome/Edge/Chromium starts headless.
- **Attach:** Settings CDP URL (e.g. `http://127.0.0.1:9222`) → attach to that browser; closing tabs does not kill the remote process.

## Local files and dev servers

Do not open `file://`. Project files and local dev servers are already served by this app (default `http://127.0.0.1:7801`). Use those URLs.

| Target | Open this |
|--------|-----------|
| Project file | `http://127.0.0.1:7801/api/v1/projects/<projectId>/raw/<path>` |
| Local dev server | `http://127.0.0.1:7801/api/v1/proxy/http/<host>-<port>/<path>` |

- `<projectId>` is the `proj-…` directory in the working path when the workspace lives under the data dir.
- `<path>` is relative to the project files root (the working directory), e.g. `index.html`.
- HTML responses include `<base href>`, so relative assets (`vendor/`, `textures/`) load from the same `/raw/` directory.
- A dev server on `localhost:3000` is `/api/v1/proxy/http/localhost-3000/`. HTTPS uses `/api/v1/proxy/https/…`.
- If the goal gives a `file://` path, convert it to the `/raw/` URL above. Do not start another static server.
- Loopback (`127.0.0.1`, `localhost`, `::1`) is allowed. Other private addresses (10/8, 192.168/16, link-local, cloud metadata) stay blocked.

## Workflow

1. `browser_navigate(url=…)` — returns title, URL, and interactive snapshot with refs `e1`, `e2`, …
2. Choose a ref from that snapshot. Call `browser_act(action=…, ref=…)`.
3. Use the **returned** snapshot for the next act (do not reuse stale refs).
4. `browser_screenshot` only when a11y/refs are insufficient and the model accepts images.
5. End with `browser_close` unless the goal says to keep the session.

### Actions

| action | Required | Notes |
|--------|----------|-------|
| `click` | `ref` | |
| `type` | `ref`, `text` | Clears then types |
| `press` | `key` | Enter, Tab, Escape, Backspace, ArrowUp, … |
| `scroll` | optional `direction`/`amount` | `up`/`down`, default 600px |
| `select` | `ref`, `value` | |
| `hover` | `ref` | |

## Rules

- Never invent refs; only use refs from the latest snapshot/navigate/act result.
- Do not loop the same failing act more than once without a new strategy.
- Sensitive actions (login credentials, payments, destructive posts): `ask_user` unless the goal already authorizes the exact action.
- Prefer concise evidence in the final report over dumping full snapshots.

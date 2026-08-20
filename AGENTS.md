# AGENTS.md — Danmo Work

## Quick reference

| Item | Location |
|------|----------|
| API entry | `server/main.go` |
| HTTP routes | `server/api/v1/` |
| Bootstrap | `core/bootstrap/bootstrap.go` |
| Domain model | `core/domain/` |
| Port interfaces | `core/port/` |
| Runtime | `core/runtime/` |
| Services | `core/service/` |
| Adapters | `core/adapter/` |
| Store | `core/store/` |
| CLI entry | `cli/main.go` |
| TUI entry | `tui/main.go` |
| Frontend | `frontend/` → build to `out/frontend/dist/` |
| Builtin home pack | `core/resource/home/` (core agents/skills; `go:embed` → `SyncBuiltinToFS`) |
| Builtin plugins | `core/resource/plugins/` (capability packs: github / danmo-make / novel / browser; `SyncBuiltinPlugins`) |
| Reasoning dialects | `core/adapter/llm/REASONING_DIALECTS.md` + `domain.ReasoningDialectInfos` |
| Optional OCI env | `environments/agent-base/` → CI Release asset (not in app packs); `make build-env-tar` locally |
| Bundled ripgrep | `scripts/fetch_ripgrep.sh` → `out/rg/<target>/rg` + `~/.danmo-work/bin/rg`; desktop packs stage it into the bundle (`resources/rg/`); `make fetch-ripgrep` locally |
| First-launch hooks | `scripts/first_launch/{darwin,linux,windows}/` — staged per platform; CodeGraph CLI via **market connector deps scripts** |
| Dev scripts | `scripts/start_backend.sh`, `scripts/start_web.sh`, `scripts/start_desktop.sh`, `scripts/stop.sh` |
| Paths | `scripts/out_paths.sh` |

## Commands

```bash
make dev-web              # backend :7801 + Vite :5801
make dev-desktop          # backend + Tauri webview
make backend              # backend only (for debugger)
make dev-cli              # CLI
make dev-tui              # TUI
make stop                 # stop all dev processes
make test                 # go test ./...
make test-integration     # integration tests
make build-all            # out/frontend/dist + out/server/* (3 binaries)
make build-go             # all 3 Go binaries
make build-server         # server only
make build-cli            # cli only
make build-tui            # tui only
make build-env-tar        # optional OCI env tar → out/env/*.tar (Release asset; not in app packs)
make pack-linux-desktop
make pack-macos-desktop
make pack-windows-desktop
make clean                # rm -rf out/
```

## Dev ports

Backend **7801**, frontend **5801** (`78xx` / `58xx`, suffix `01` = Teams). See `scripts/out_paths.sh`.

## Build layout (`out/`)

```
out/frontend/dist/   # Vite production
out/server/          # Go binaries (danmo-work, danmo-work-cli, danmo-work-tui)
out/desktop/bundle/  # Tauri installers
out/desktop/cargo/   # Cargo intermediate
out/env/             # optional OCI image tar (dev/CI; user downloads to ~/.danmo-work/env/)
out/run/             # dev PIDs, logs, wrappers (DQ_DEV markers)
```

## Environment

One root, everything derived: `WORK_HOME` relocates the whole data home;
config, all databases, and `data/` follow. The per-path variables below are
fine-grained overrides and normally unnecessary. `history.db` has no env var —
it always sits next to the control DB (or set `data.history_database` in YAML).

| Variable | Default | Purpose |
|----------|---------|---------|
| `WORK_HOME` | `~/.danmo-work` | Root data home; all paths below derive from it |
| `WORK_CONFIG` | `$WORK_HOME/config.yaml` | YAML config path |
| `WORK_DB_PATH` | `$WORK_HOME/work.db` | SQLite control-plane database |
| `WORK_STORE_DB_PATH` | `$WORK_HOME/store.db` | SQLite agent table-store (data plane) |
| `WORK_DATA_DIR` | `$WORK_HOME/data` | Projects (turn history lives in `history.db`; JSONL is export-only) |
| `DQ_BACKEND_PORT` | `7801` | Injected by dev scripts |
| `DQ_FRONTEND_PORT` | `5801` | Injected by dev scripts |
| `DQ_APP_NAME` | `danmo-work` | App name for build scripts |

### User data (`~/.danmo-work/`)

Server, CLI, TUI, and desktop all use the same home by default:

```
~/.danmo-work/
  config.yaml
  work.db        # control plane (sessions, turns metadata, compaction checkpoints, memories, …)
  history.db     # history plane (turn_log_entries + stream_events + file_changes; retention-pruned, incremental auto-vacuum)
  store.db       # agent table-store data plane
  knowledge/     # knowledge-base Markdown source of truth
  data/          # projects (legacy turn JSONL / checkpoint JSON / file_changes.jsonl kept as inert backups after DB import)
  skills/        # optional user custom skills (scanned each turn)
  bin/           # desktop sidecar binary
  bin/coreutils/ # Windows: Microsoft Coreutils (default-installed by NSIS) + applet hardlinks
  backend.log    # desktop sidecar log
```

Custom skills (Agentskills layout) are also read from `~/.agents/skills/`,
`<project>/.danmo-work/skills/`, and `<project>/.agents/skills/` on each new turn
(memory only, not SQLite).

History retention: orphaned history (deleted sessions) is pruned at startup
and daily; optional age-based pruning of stale sessions via
`runtime.retention.history_max_age_days` (0 = off, default).

Agent durable memories live in SQLite (`memories` table) via
`memory_update` / `memory_read` (scopes: user / project / agent). UI: right
workspace **Memory** tab. Config: `runtime.memory.read_top_k`.

Skill marketplace: configure `market.sources` (git catalog, curated
`kind: techleads` → Tech Leads Club npm CDN catalog, and optional
`kind: clawhub` → ClawHub registry). Market installs land in the SQLite
skill library (Skills → Market tab).

Agent table store (`table_*` tools) persists schema-free business rows in
`store.db` (isolated from `work.db`).

Knowledge bases store Markdown under `knowledge/<kbId>/` with chapter FTS
in `work.db`.

On first launch, existing data may be migrated from
`~/Library/Application Support/com.danmo.work/` or `./data/work.db`.

## Desktop (Tauri)

Thin shell — backend must run separately. Then `cd desktop && npm run tauri dev`.

Builds Go backend as a Tauri sidecar binary (`scripts/build_sidecar.sh`), injected into `.app` bundle with re-sign on macOS.

## CI

`.github/workflows/release.yml` builds on tag `v*` or `workflow_dispatch`:

- macOS desktop → `out/desktop/bundle/**/*.dmg, *.app`
- Linux desktop → `out/desktop/bundle/**/*.AppImage, *.deb`
- Windows desktop → `out/desktop/bundle/**/*.exe`

Homebrew cask: `Casks/danmo-work.rb` (bumped on `v*` release; URL =
GitHub Releases `.dmg`). Short tap: `brew tap danmo-ai/tap` via repo
`danmo-ai/homebrew-tap` + secret `HOMEBREW_TAP_TOKEN` (workflow
**Publish Homebrew Tap**). Fine-grained PAT for `danmo-ai` must have
**Expiration ≤ 366 days** (org policy). Fallback: tap this repo with
full URL. Optional release object mirror via `UPDATE_MIRROR_*`
(set `UPDATE_MIRROR_BASE_URL` only when a real CDN/bucket exists;
`danmo.work` is the marketing site on GitHub Pages, not a binary host).

```bash
brew tap danmo-ai/tap
brew install --cask danmo-work
```

Checks out `danmo-ai/dq-ui` alongside the repo.

## Architecture

```
server/   cli/   tui/    frontend/ (Vue 3 + Vite)
    \       \     /       /
     \       \   /       /
      ---- core/bootstrap ----
              |
  core/service ─── core/runtime ─── core/adapter
       |              |                 |
  core/port ←─────────┘    core/adapter/llm
       |                  (Anthropic / Mock)
  core/store/sqlite
  core/store/turnlog
```

### Layer descriptions

| Layer | Directory | Role |
|-------|-----------|------|
| Entry points | `server/`, `cli/`, `tui/` | HTTP API (Gin), CLI, TUI |
| Bootstrap | `core/bootstrap/` | DI wiring, global config assembly |
| Services | `core/service/` | SessionManager, ProjectManager, AgentManager, SkillManager, LLMConfigManager, etc. |
| Runtime | `core/runtime/` | SessionRunner, TurnRunner, PromptBuilder, Compaction, Permission, Tool exec |
| Domain | `core/domain/` | Agent, Session, Project, Skill, Knowledge, MCPServer, LLMConfig, Turn, StreamEvent, etc. |
| Ports | `core/port/` | Engine, LLMProvider, Repository, Stream interfaces |
| Adapters | `core/adapter/` | LLM providers (Anthropic, mock), config loader |
| Store | `core/store/` | SQLite persistence, turn log |

### Request flow

```
HTTP Request → server/api/v1 handler → port interface
    → service impl (core/service/)
    → port.Repository interface
    → core/store/sqlite (SQLite)
```

## Notes

- Static UI served from `./out/frontend/dist` at `/app/` when built
- Process stop uses `DQ_DEV` / `DQ_DEV_ROOT` markers (`scripts/stop.sh`)
- Requires sibling `dq-ui` repo for frontend (`file:../../dq-ui/packages/*`)

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
make pack-linux-server
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
out/dist/            # pack-linux-server tar.gz
out/run/             # dev PIDs, logs, wrappers (DQ_DEV markers)
```

## Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `WORK_CONFIG` | `~/.danmo-work/config.yaml` | YAML config path |
| `WORK_DB_PATH` | `~/.danmo-work/work.db` | SQLite control-plane database |
| `WORK_STORE_DB_PATH` | `~/.danmo-work/store.db` | SQLite agent table-store (data plane) |
| `WORK_DATA_DIR` | `~/.danmo-work/data` | Projects / turn logs |
| `DQ_BACKEND_PORT` | `7801` | Injected by dev scripts |
| `DQ_FRONTEND_PORT` | `5801` | Injected by dev scripts |
| `DQ_APP_NAME` | `danmo-work` | App name for build scripts |

### User data (`~/.danmo-work/`)

Server, CLI, TUI, and desktop all use the same home by default:

```
~/.danmo-work/
  config.yaml
  work.db        # control plane (sessions, memories, …)
  store.db       # agent table-store data plane
  data/          # projects, turn logs
  skills/        # optional user custom skills (scanned each turn)
  bin/           # desktop sidecar binary
  bin/coreutils/ # Windows: Microsoft Coreutils (default-installed by NSIS) + applet hardlinks
  backend.log    # desktop sidecar log
```

Custom skills (Agentskills layout) are also read from `~/.agents/skills/`,
`<project>/.danmo-work/skills/`, and `<project>/.agents/skills/` on each new turn
(memory only, not SQLite).

Agent durable memories live in SQLite (`memories` table) via
`memory_update` / `memory_read` (scopes: user / project / agent). UI: right
workspace **Memory** tab. Config: `runtime.memory.read_top_k`.

Agent table store (`table_*` tools) persists schema-free business rows in
`store.db` (isolated from `work.db`). See `docs/agent-table-store-plan.md`.

On first launch, existing data may be migrated from
`~/Library/Application Support/com.danmo.work/` or `./data/work.db`.

## Desktop (Tauri)

Thin shell — backend must run separately. Then `cd desktop && npm run tauri dev`.

Builds Go backend as a Tauri sidecar binary (`scripts/build_sidecar.sh`), injected into `.app` bundle with re-sign on macOS.

## CI

`.github/workflows/release.yml` builds on tag `v*` or `workflow_dispatch`:

- macOS desktop → `out/desktop/bundle/**/*.dmg, *.app`
- Linux server → `out/dist/danmo-work-linux-*.tar.gz`
- Windows desktop → `out/desktop/bundle/**/*.exe`

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

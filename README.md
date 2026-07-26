# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**Open-source AI Work Agent** — coding-agent-grade loop for long-horizon work. Self-hosted, multi-agent, MIT.

Not just another powerful coding agent. Not a workflow graph you babysit. Danmo Work is a **human–AI co-thinking workspace**: the same Agent Loop that handles files, shell, and multi-agent coding also ships **docs, slides, sheets, connectors, and IM** — every Tool Call logged so you can **resume, replay, and edit the thinking trail**.

> Code, research reports, decks, sheets, demos, and automations — from desktop, web, CLI/TUI, or WeChat / Feishu / WeCom / QQ — on one trail.

| | |
|--|--|
| **Positioning** | General-purpose **Work Agent** — coding is a first-class lane, not the ceiling |
| **Control** | **Pure LLM-driven** — no hand-maintained graph / role router / product “mode” |
| **Abstraction** | **Everything is a Tool** — `delegate_agent`, `ask_user`, memory, table store, MCP, files… |
| **State** | **Log is state** — Turn Log → recover, replay, edit a result and continue |
| **Surfaces** | Web · Desktop · CLI · TUI · IM channels · Document Stage |

MIT · Anthropic & OpenAI-compatible providers · Local-first data under `~/.danmo-work/`

---

## Why Danmo Work (in 30 seconds)

Most open-source agents stop at **writing code**: terminal pair-programmers, IDE plugins, sandboxed software engineers. Strong loops — narrow job.

Danmo Work keeps a **coding-agent-grade** execution core, then asks a wider question: **how do humans and models co-think through real work** — code, research, docs, slides, ops, connectors — over a long horizon, with a trail you can trust?

| You get | Instead of |
|---------|------------|
| One thinking chain + hard-isolated sub-agents | Parallel sessions / opaque handoffs |
| Document Stage (doc / slides / sheet / preview) | Chat that dumps Markdown into a void |
| Inspectable Memory + schema-free Table Store | Black-box product memory or another vector DB |
| MCP Connectors + cron/webhook Automations | One-off scripts glued outside the loop |
| WeChat · Feishu · WeCom · QQ on the same loop | “Deploy a public webhook” IM hacks |
| Resume / replay / edit Tool Results | Restart the chat and hope |

**Mainstream:** developer or product orchestrates; LLM executes.  
**Danmo Work:** LLM orchestrates on one chain; you supply capability units; humans join as peers via `ask_user`.

---

## Try it

| Platform | Channel | How |
|----------|---------|-----|
| **macOS** (Apple Silicon) | **Homebrew** (recommended if you use brew) | see below |
| **macOS** (Apple Silicon) | `.dmg` | [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest) |
| **Windows** | Setup `.exe` | [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest) |
| **Linux server** | `.tar.gz` | [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest) |

### macOS — Homebrew

```bash
brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git
brew install --cask danmo-work
```

Upgrade later: `brew update && brew upgrade --cask danmo-work`.

Not Apple-notarized yet — on first launch, right-click the app → **Open** (or allow under System Settings → Privacy & Security).

### macOS — DMG

Download `Danmo.Work_*_arm64.dmg` from [Releases](https://github.com/danmo-ai/danmo-work/releases/latest), drag into Applications, then right-click → Open the first time.

### From source

Needs sibling [`dq-ui`](https://github.com/danmo-ai/dq-ui):

```bash
make dev-web   # → http://localhost:5801/app/
```

Add an LLM API key in the UI (or `~/.danmo-work/config.yaml`). Full flow: [Quick start](#quick-start).

---

## Who it’s for

- **Builders & operators** who need an agent that finishes *work products*, not only PRs
- **Developers** who want a coding-agent-grade CLI/TUI — and the same loop for everything after the diff
- **Teams behind a firewall** who want Feishu / WeCom / QQ without a public callback URL
- **Power users** who want Memory, Table Store, and Turn Log to be **visible and editable**
- **Anyone tired of Graph/Role frameworks** that fight the model’s planning instead of feeding it Tools

---

## Work Agent, not coding-only — comparison

Typical OSS coding agents excel at **code-centric loops**. Danmo Work runs a loop in that class — then adds **work runtime + co-thinking UX** on the same trail.

| Dimension | Typical OSS coding agents | Agent frameworks (LangGraph / CrewAI / AutoGen) | **Danmo Work** |
|-----------|---------------------------|--------------------------------------------------|----------------|
| Primary job | Code, PR, terminal | App/workflow orchestration | **Long-horizon work (incl. coding) + artifacts** |
| Agent loop | Strong, code-focused | Developer-written graph / roles | **Coding-agent-grade + pure LLM Tool Call planning** |
| Sub-agents | Extra session or skill | Handoff / crew roles | `delegate_agent` on **same chain**, hard isolation |
| Human in the loop | Approvals / chat | Preset nodes | `ask_user` Tool — model chooses when |
| Artifacts | Repo diffs | App-defined | Diffs **+ Document Stage**: doc · slides · sheet · preview |
| Memory | Product-private or none | Buffers / external vector DBs | Explicit `memory_*` + scoped SQLite + UI tab |
| Business data | Files / DIY DB | LangGraph Store etc. | Built-in **Table Store** (`store.db`, schema-free) |
| Connectors | MCP / plugins (varies) | DIY | MCP catalog + secrets + permissions + automations |
| IM / chat ingress | Rare / DIY | Rare | **WeChat · Feishu · WeCom · QQ** (outbound WS) |
| Durability | Session / container | Optional checkpointer | **Turn Log = state** (resume · replay · edit) |
| License / host | Mostly OSS | OSS libs | **MIT, self-hosted**, Web/Desktop/CLI/TUI |

Use the CLI/TUI as your daily coding agent when the job is code. Stay in the same Work Agent when the job is **the work around the code** — docs, decks, data, connectors, and IM.

---

## Product value

1. **Finish the job, not the chat** — Stage-native docs, Markdown slides, sheets, and HTML preview stay in the project filesystem.
2. **Trust through transparency** — every Tool Call is persisted; recover mid-turn; edit a Tool Result and continue.
3. **Scale capability without scaling complexity** — new power = new Tool / Skill / MCP server, not a new graph language.
4. **Meet people where they already chat** — same Agent Loop from phone WeChat or Feishu cards; tools still run on your machine.
5. **Local-first ownership** — config, DB, turn logs, secrets under `~/.danmo-work/`; bring your own model keys.

---

## See it

Architecture · highlights · capacity — bilingual animated tour (HTML first; GIF/MP4 for sharing):

![Product tour (EN)](docs/demo/product-tour-en.gif)

Interactive (ZH/EN toggle): [`docs/demo/product-tour.html`](docs/demo/product-tour.html) · [MP4](docs/demo/product-tour-en.mp4)

Three-pane workspace: project sidebar · agent execution Stream · right panel (Plan / Files / **Memory** / Changes / Terminal). Center **Document Stage** switches toolbar by file kind.

### Document Stage — docs, slides, sheets, preview

Open any project file from Files → center Stage. Editable kinds use format-specific editors + AI; generic HTML / images / URLs use **Preview** (URL bar + Design mode). AI polish runs as a normal **session turn**.

| Kind | Source of truth | Editor / view |
|------|-----------------|---------------|
| **Doc** | GFM `.md` | TipTap (MD ↔ HTML in the edit session) |
| **Slides** | Markdown with `---` pages | Edit markdown + present playable HTML |
| **Sheet** | `.csv` / `.danmo-sheet.json` | Grid editor |
| **Preview** | Generic `.html`, images, URLs | iframe / image; element pick → Composer |

Toolbar builds an `[office-edit]` prompt → `POST /sessions/:id/turns`. Dirty content auto-saves before AI; scope can be selection / full document / this slide / whole sheet. After the turn, Stage reloads and restores scroll (and slides page index).

### Point at the page — don’t describe it

In Stage **Preview**, click a DOM element, annotate, confirm into Composer. The model gets exact HTML/CSS context — **select → annotate → edit**.

![Browser element annotate](docs/screenshots/ui-browser-annotate.png)

| Research & report | Interactive demo | Mini-game |
|-------------------|------------------|-----------|
| ![Market report](docs/screenshots/ui-market-report.png) | ![Cooking demo](docs/screenshots/ui-cooking-demo.png) | ![Snake game](docs/screenshots/ui-snake-game.png) |

- **Research & report** — web fetch, structured writing, live HTML preview  
- **Interactive demo** — step-by-step demo with playback controls  
- **Mini-game** — generate a playable page, then iterate via element annotate  

### Channels (WeChat · Feishu · WeCom · QQ)

Same Agent Loop from IM — tools on your machine; Turn Log in Teams. Sessions keyed by `(channel, account, peer)` so multiple channels on one project **don’t mix chats**.

| Channel | How it connects | Highlights |
|---------|-----------------|------------|
| **WeChat** | Phone WeChat (iLink long-poll) | Account default project + `/project`; text-menu approvals; image/file in |
| **Feishu** | Outbound WebSocket (no public URL) | Cards/forms, approvals, progress, media, `/project` |
| **WeCom** | Outbound WebSocket | Admin Smart Robot → Settings; stream placeholder then final answer |
| **QQ** | Outbound Gateway WebSocket | Keyboard approvals, C2C stream, group deny-tools, `/project` |

| Desktop (WeChat-tagged session) | Phone (WeChat chat) |
|---------------------------------|---------------------|
| ![WeChat session in Teams](docs/screenshots/wx1.png) | ![DQ-Teams AI in WeChat](docs/screenshots/wx2.png) |

### Experts, skills, connectors & data plane

Edit prompts, Agentskills (`SKILL.md`), sandbox / delegation in the UI — **capability units**, not a workflow graph. Summon skills from Composer via `@` / button.

| Expert prompt editor | Skill library | Runtime & sandbox |
|----------------------|---------------|-------------------|
| ![Explorer system prompt](docs/screenshots/ui-expert-prompts.png) | ![playable-slides skill](docs/screenshots/ui-skill-editor.png) | ![Runtime settings](docs/screenshots/ui-runtime-settings.png) |

- **Experts** — local + market agents; overview / prompt / skills / tools / knowledge  
- **Skills** — built-in & custom Agentskills; instructions, files, tool bindings  
- **Connectors (MCP)** — catalog (Composio / OpenConnector / GitHub / Notion / Feishu…); `tools/list` → `mcp_<server>_<tool>` on the loop; encrypted secrets; `external` risk → permission gate  
- **Automations** — cron / webhook start real session turns with Turn Log replay  
- **Memory** — `memory_update` / `memory_read` (scopes: user · project · agent); Memory tab  
- **Table Store** — schema-free `table_*` tools on isolated `store.db` for digests, counters, cursors (not Memory, not files)  
- **Runtime** — turn limits, tool output hard cap (`runtime.tools.max_output_chars`, default 50k), max delegation depth, memory TopK, OS sandbox & network policy  

---

## Design philosophy

### Everything is a Tool

| Traditional concept | Unified abstraction |
|---------------------|---------------------|
| Sub-agent delegation | `delegate_agent` |
| User interaction | `ask_user` |
| Skills | `read_skill` / skill bindings |
| Knowledge | `search_kb` |
| Durable memory | `memory_update` / `memory_read` |
| Business rows | `table_upsert` / `table_query` / … |
| Files | `read_file` / `write` / `edit` / … |
| External APIs | `http_request` / MCP / `web_fetch` · `web_search` |

One abstraction (Tool), one loop (Agent Loop), one execution store (Turn Log). New capability = new Tool.

### Pure LLM-driven control

No developer-maintained graph, role router, or mode switch — the model plans Tool Calls on one loop:

```
User input
    ↓
[LLM] → plans Tool Call DAG
    ↓
Execute tools (Agent Loop)
    ↓
Need clarification? → ask_user
    ↓
Need to remember? → memory_*  |  Need rows? → table_*
    ↓
Need delegation? → delegate_agent
      → fresh Turn (system + goal; parent transcript not inherited)
      → own tools / skills / knowledge → same Agent Loop
      → Report only → parent continues
    ↓
Done → deliver result
```

Coding vs “work” emerges from configuration and `ask_user` defaults — not an explicit `mode` flag.

### Log is state

- Every Tool Call (input, output, latency, rationale) persisted  
- Failures recoverable — retry from any step  
- Full replay for debug and audit  
- Humans can edit any Tool Result; the agent continues  

### Memory vs Table Store vs Knowledge

| Store | Role |
|-------|------|
| **Memory** | Lasting prefs / conventions the model *chooses* to keep |
| **Table Store** | Queryable business rows (digests, cursors) in `store.db` |
| **Knowledge** | Human-curated docs bound to an agent (`search_kb`) |
| **Compaction** | Session-local summary when context truncates — not durable memory |

---

## Concept model

```
Project/
  └── Session (long-horizon, days/weeks)
        ├── Turn-1  ← one [input → agent reply]
        │     ├── Step: LLM call (function calling)
        │     ├── Step: Tool exec → inject result
        │     └── ...
        ├── Turn-2  ← follow-up days later
        ├── ~ Checkpoint (compaction) ~
        └── Turn-N
```

| Concept | Definition |
|---------|------------|
| **Project** | Task collection bound to a filesystem directory |
| **Session / Task** | Multi-turn interaction around one goal |
| **Turn** | One [input → agent reply], containing N LLM Steps |
| **Step** | One LLM request/response inside a Turn |
| **Delegated agent** | `delegate_agent` Tool; isolated child; Report back |
| **ask_user** | Asking the user is a Tool; loop pauses for the reply |
| **Memory / Table Store** | Cross-session facts vs schema-free business rows |

---

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
       |                  (Anthropic / OpenAI-compat / Mock)
  core/store/sqlite + turnlog + store.db
```

| Layer | Directory | Role |
|-------|-----------|------|
| Entry points | `server/` `cli/` `tui/` | HTTP (Gin), CLI, TUI |
| Frontend | `frontend/` | Vue 3 + Vite + Document Stage |
| Bootstrap | `core/bootstrap/` | DI, config |
| Services | `core/service/` | Session, Project, Agent, Skill, MCP, channels, … |
| Runtime | `core/runtime/` | Turn loop, prompt, compaction, permission, tools |
| Domain / Ports | `core/domain/` `core/port/` | Entities & interfaces |
| Adapters | `core/adapter/` | LLM + IM (Feishu / QQ / Weixin / WeCom) |
| Store | `core/store/` | SQLite (`work.db`) + Turn Log + Table Store (`store.db`) |

Deep dive: [`docs/core-design.md`](docs/core-design.md).

---

## Prerequisites

- Go 1.26+
- Node.js 20+ (frontend / desktop)
- Sibling [`dq-ui`](https://github.com/danmo-ai/dq-ui) (`file:../../dq-ui/packages/*`)

```text
Workspace/
  Danmo-Work/
  dq-ui/
```

## Quick start

```bash
make dev-web          # backend :7801 + Vite :5801 → http://localhost:5801/app/
make dev-desktop      # backend + Tauri webview
make backend          # backend only (debugger-friendly)

make dev-cli          # CLI (no server)
make dev-tui          # TUI (no server)
make stop             # stop all DQ_DEV processes
```

```bash
mkdir -p ~/.danmo-work
cp config.example.yaml ~/.danmo-work/config.yaml
# Add LLM provider API keys in the UI or config
```

## Build & pack

```bash
make build-all              # frontend dist + Go server/cli/tui
make build-go               # all three Go binaries
make pack-macos-desktop     # .dmg / .app
make pack-linux-server      # tar.gz
make pack-windows-desktop   # .exe
make clean                  # rm -rf out/
```

```text
out/
  frontend/dist/     # Vite production (served at /app/)
  server/            # danmo-work, danmo-work-cli, danmo-work-tui
  desktop/bundle/    # Tauri installers
  dist/              # Linux server release tarball
  run/               # Dev PIDs, logs, wrappers
```

## Test

```bash
make test               # layer check + go test ./...
make test-integration   # integration tests
```

### Harbor agent compare (Terminal-Bench 2.0)

Official **terminal-bench@2.0** (**89** tasks). Tasks are **not in git** — sync locally, then Harbor + Podman. Pass = Mean reward ≥ 1.0.

How-to: [`evals/dq_harbor/README.md`](evals/dq_harbor/README.md) · Scores: [`evals/dq_harbor/COMPARE_RESULTS.md`](evals/dq_harbor/COMPARE_RESULTS.md).

```bash
podman machine start                                    # macOS if needed
make eval-harbor-base
GH_TOKEN=$(gh auth token) make eval-harbor-sync-tb2
make eval-harbor-bin
export WORK_MODEL=deepseek/deepseek-v4-flash WORK_API_KEY=... WORK_BASE_URL=https://api.deepseek.com
make eval-harbor-smoke
./evals/dq_harbor/compare_agents.sh                     # vs OpenCode / OpenHands
```

## Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `WORK_CONFIG` | `~/.danmo-work/config.yaml` | YAML config |
| `WORK_DB_PATH` | `~/.danmo-work/work.db` | Engine SQLite |
| `WORK_STORE_DB_PATH` | `~/.danmo-work/store.db` | Agent Table Store |
| `WORK_DATA_DIR` | `~/.danmo-work/data` | Projects / turn logs |
| `DQ_BACKEND_PORT` | `7801` | Dev backend |
| `DQ_FRONTEND_PORT` | `5801` | Dev frontend |
| `VITE_API_BASE_URL` | `""` | Frontend API base (empty = same origin) |

### Custom skill directories

Each new turn scans Agentskills (`skill-name/SKILL.md`) in memory — **not SQLite** — into `<available_skills>`:

| Path | Scope |
|------|-------|
| `~/.agents/skills/` | User |
| `~/.danmo-work/skills/` | User |
| `<projectRoot>/.agents/skills/` | Project |
| `<projectRoot>/.danmo-work/skills/` | Project |

Later paths win on ID collision. Disk changes apply next turn.

## Desktop (Tauri)

```bash
make dev-desktop
# or with an already-running backend:
SKIP_BACKEND=1 make dev-desktop
```

## CI / release

`.github/workflows/release.yml` on `v*` tags or `workflow_dispatch`:

| Job | Artifact |
|-----|----------|
| macOS desktop | `out/desktop/bundle/*.dmg`, `*.app` |
| Linux server | `out/dist/danmo-work-linux-*.tar.gz` |
| Windows desktop | `out/desktop/bundle/*.exe` |

**macOS channels:** GitHub Releases `.dmg`, or Homebrew cask [`Casks/danmo-work.rb`](Casks/danmo-work.rb) (bumped on each `v*` release; tap this repo with the long name above).

## Docs

| Doc | Description |
|-----|-------------|
| [docs/core-design.md](docs/core-design.md) | Core design: agent architecture, channels, Stage |
| [docs/agent-table-store-plan.md](docs/agent-table-store-plan.md) | Schema-free Table Store (`store.db`) |
| [docs/channel-qq-feishu-plan.md](docs/channel-qq-feishu-plan.md) | QQ / Feishu / Weixin channels (Phase A–C) |
| [docs/launch-posts.md](docs/launch-posts.md) | Community launch drafts |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Harbor Terminal-Bench 2.0 |
| [AGENTS.md](AGENTS.md) | Contributor / agent quick reference |
| [config.example.yaml](config.example.yaml) | Full config reference |

## License

[MIT](LICENSE)

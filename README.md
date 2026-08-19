# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**Open-source AI work agent** — multi-agent, long-horizon work on your own machine. Coding, research, reports, slides, sheets, and automations all run on a single agent loop you can watch, correct, and resume. Self-hosted, MIT.

- **The model orchestrates.** No workflow graphs to configure. Subagents (`delegate_agent`), questions (`ask_user`), and MCP connectors are all just tools the agent calls as it plans.
- **Turn Log is state.** Every tool call is appended to JSONL. Recover from a crash, replay a turn, or edit a tool result and let the loop continue.
- **Everywhere.** Web · Desktop (Tauri) · CLI · TUI · WeChat / Feishu / WeCom / QQ — one engine, tools always running on your machine.

[![Screenshot tour — multi-agent, Document Stage, browser preview, skills, sandbox, IM](docs/screenshots/carousel.gif)](docs/screenshots/carousel.html?lang=en)

[Interactive carousel (captions)](docs/screenshots/carousel.html?lang=en) · [Architecture tour](docs/demo/product-tour.html?lang=en) · [Office co-edit tour](docs/demo/office-coedit-tour.html?lang=en&tour=1)

> Code, reports, decks, sheets, demos, automations — on one trail.

---

## Highlights

| | |
|--|--|
| **Multi-agent teams** | A lead agent summons built-in specialists — Document, Implementer, Researcher, Reviewer, GitHub, Novel Writing… — each in its own isolated context |
| **Document Stage + AI Diff** | Docs, slides, and sheets on one canvas: propose → review diff → keep / revert / accept hunks. Markdown stays the source of truth |
| **Humans in the loop** | `ask_user` is just another tool — with options and forms. Approval gates stop risky commands before they run |
| **Plans are a tool too** | The plan is `todowrite`: the agent updates status as it works, marks items done only after verifying, and rewrites the plan when it drifts — not a one-time artifact that gathers dust |
| **Durable memory & Table Store** | `memory_update` / `memory_read` across user / project / agent scopes; `table_*` schema-free business rows; Markdown knowledge bases with chapter-level search |
| **Skills & connectors** | Skill market (official catalog, Tech Leads Club, ClawHub), disk-scanned custom skills, MCP connectors bound per agent |
| **Real sandbox** | OS-level shell sandbox (Seatbelt / Landlock / bwrap / WSL2), network deny / domain allowlists, four permission modes |
| **Automations** | Cron schedules and webhooks start agent turns in the background |
| **Your model, your data** | Anthropic-native + OpenAI-compatible providers (OpenAI, DeepSeek, GLM, Qwen, Kimi, Gemini, Grok, local Ollama…). Data lives in `~/.danmo-work/` |

---

## Design

Three ideas run through the whole system (full architecture: [docs/core-design.md](docs/core-design.md)):

1. **Everything is a tool.** Files, shell, web, memory, tables, knowledge, subagents (`delegate_agent`), even humans (`ask_user`) share one interface. The model decides what to call and when.
2. **The model orchestrates.** No developer-written control flow: the lead agent plans, delegates, and checks in with you on its own. Code and long-running work are the same loop at different depths — there is no mode switch.
3. **Logs are state.** Each turn is an append-only JSONL trail of tool calls. Sessions can span days or weeks; recovery is built in, not bolted on.

### Experts & teams

The lead agent runs the session; specialists do focused work on demand, each in an isolated sub-turn (roster and usage: [docs/experts.md](docs/experts.md)):

| Agent | Role |
|-------|------|
| **Team** (lead) | Multi-agent by default; collaboration toggle |
| **Document** | Reports, slides, sheets (Markdown source for Document Stage) |
| **Comms** | Polishes messages, emails, notifications |
| **Implementer** | Code changes from specs (TDD / debugging skills) |
| **Explorer** | Read-only codebase exploration |
| **Researcher** | Deep research and retrieval |
| **Reviewer** | Code and artifact review |
| **Data** | CSV / JSON analysis and reporting |
| **GitHub** | Issues, PRs, Actions, releases |
| **Danmo Make** | Local image / video / audio generation (separate app) |
| **Novel Writing** | Long-form fiction: outline → chapter contract → draft → review → commit |
| **CodeGraph** (market) | Code intelligence — definitions, references, impact |

Summon a specialist with `@` in the Composer, or just ask in plain language ("delegate the Document expert…"). The lead only sees the subagent's report, so the main context stays lean and KV-cache friendly. You can also build your own subagents in Teams, binding skills, tools, knowledge bases, and connectors.

Beyond experts, the same library hosts **skills** (one-click workflows — document-writing, playable-slides, TDD, deep-research…) and **connectors** (MCP integrations). Install from the market, bind them per agent, or drop your own skills into `~/.danmo-work/skills/`.

### Document Stage

Three panes — projects · agent stream · right panel (Plan / Files / Memory / Changes / Terminal) — around a central **Document Stage** whose toolbar follows the file kind (doc / slides / sheet / code / diff / preview). In Preview you can click a DOM element, annotate it, and send the exact HTML/CSS context to the Composer.

Office files are co-edited as a normal agent turn with a review step:

1. **Intent** — select text, a slide, or cells, type an instruction → `[office-edit]`
2. **Propose** — the agent edits; a snapshot is taken before the turn
3. **Review** — view the diff; keep, revert, or accept individual hunks
4. **Commit** — keep persists, revert restores the snapshot; the trail stays in the Turn Log

### Safety

- **Soft gate** — permission modes (`discuss` / `plan` / `interactive` / `auto`); risky commands ask before running
- **Hard sandbox** — OS-level isolation for shell, network `deny` or domain allowlists
- `auto_approve` never silently approves dangerous commands; approvals render inside IM chats too

### Channels, remote & automations

- **IM on the same loop** — Feishu, QQ, WeChat, and WeCom connect outbound to your own accounts; progress cards, questions, and approvals appear in-channel. Tools still run on your machine.
- **Remote Hub** — pair this PC with [danmo-hub](https://github.com/danmo-ai/danmo-hub) so remote clients can drive it, even behind NAT.
- **Automations** — cron and webhook triggers run sessions while you're away.

---

## Install

| Platform | Package |
|----------|---------|
| **macOS** (Apple Silicon) | Homebrew or `.dmg` |
| **Windows** | Setup `.exe` |
| **Linux** (x86_64) | AppImage / `.deb` |

All binaries: [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest).

### macOS

```bash
brew tap danmo-ai/tap
brew install --cask danmo-work
# upgrade: brew update && brew upgrade --cask danmo-work
```

Fallback: `brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git`  
Or download `Danmo.Work_*_arm64.dmg` from Releases. Not Apple-notarized yet — first launch: right-click → **Open**.

### Windows / Linux

- **Windows:** `Danmo.Work_*_x64-setup.exe` — until Authenticode is enabled, SmartScreen may warn → **More info → Run anyway**.
- **Linux:** `chmod +x Danmo.Work_*_amd64.AppImage && ./…` or `sudo apt install ./Danmo.Work_*_amd64.deb` (needs WebKitGTK).

### First run

Open the app, add an LLM API key in the UI (or `~/.danmo-work/config.yaml`), and pick a model. Projects, sessions, memories, and the rest are created for you under `~/.danmo-work/`.

### From source

Needs sibling [`dq-ui`](https://github.com/danmo-ai/dq-ui). Then:

```bash
make dev-web   # backend :7801 + Vite :5801 → http://localhost:5801/app/
```

---

## Development

**Prerequisites:** Go 1.26+, Node.js 20+, sibling [`dq-ui`](https://github.com/danmo-ai/dq-ui).

```bash
make dev-web          # backend :7801 + Vite :5801 → http://localhost:5801/app/
make stop             # stop all DQ_DEV processes
mkdir -p ~/.danmo-work && cp config.example.yaml ~/.danmo-work/config.yaml
```

Build, pack, test, env vars, and CI: [`AGENTS.md`](AGENTS.md). Architecture: [`docs/core-design.md`](docs/core-design.md).

---

## Docs

| Doc | Description |
|-----|-------------|
| [docs/core-design.md](docs/core-design.md) | Agent architecture, tool system, channels, Document Stage |
| [docs/experts.md](docs/experts.md) | Expert usage and the built-in roster |
| [docs/remote/README.md](docs/remote/README.md) | Remote Hub pairing and tunnel protocol |
| [docs/screenshots/README.md](docs/screenshots/README.md) | Screenshot carousel (README hero) |
| [docs/demo/README.md](docs/demo/README.md) | Architecture tour HTML / GIF / MP4 |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Terminal-Bench 2.0 eval harness |
| [AGENTS.md](AGENTS.md) | Contributor quick reference |
| [config.example.yaml](config.example.yaml) | Full config reference |

## License

[MIT](LICENSE)

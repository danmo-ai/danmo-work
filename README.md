# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**Open-source AI work agent** — multi-agent, long-horizon work on your own machine. Coding, research, reports, slides, sheets, and automations all run on a single agent loop you can watch, correct, and resume. Self-hosted, MIT.

- **The model orchestrates.** No workflow graphs to configure. Subagents (`delegate_agent`), questions (`ask_user`), and MCP connectors are all just tools the agent calls as it plans.
- **Turn Log is state.** Every tool call lands in SQLite (`history.db`). Recover from a crash, replay a turn, edit a tool result, or resume days later — JSONL is export-only.
- **Everywhere.** Web · Desktop (Tauri) · CLI · TUI · Feishu / QQ / WeChat / WeCom — one engine, tools always running on your machine.

[![Screenshot tour — multi-agent Plan, Trajectory, Document Stage, preview annotate, experts, skills, sandbox, IM](docs/screenshots/carousel.gif)](docs/screenshots/carousel.html?lang=en)

[Interactive carousel (captions)](docs/screenshots/carousel.html?lang=en) · [Architecture tour](docs/demo/product-tour.html?lang=en) · [Office co-edit tour](docs/demo/office-coedit-tour.html?lang=en&tour=1)

> Code, reports, decks, sheets, demos, automations — on one trail.

---

## Highlights

| | |
|--|--|
| **Multi-agent teams** | **Team** lead delegates built-in specialists — Document, Implementer, Researcher, Reviewer, GitHub, Novel Writing… — each in an isolated sub-turn; main context stays lean |
| **Document Stage + AI Diff** | Docs, slides, sheets, code, diff, and preview on one canvas — pre-turn snapshot → review hunks → keep / revert / accept; Markdown stays the source of truth |
| **Humans in the loop** | `ask_user` with options and forms; approval gates before risky shell or external MCP — same flow in desktop and IM chats |
| **Plans that stay alive** | `todowrite` is the plan: status updates as work proceeds, items marked done only after verification, rewritten when scope drifts |
| **Memory, tables & knowledge** | `memory_*` across user / project / agent scopes; schema-free `table_*` rows in `store.db`; Markdown knowledge bases with chapter-level FTS |
| **Skills & connectors** | Skill market (official catalog, Tech Leads Club, ClawHub), ambient disk skills (`~/.agents/skills/`, project folders), MCP connectors bound per agent |
| **Real sandbox** | OS-level isolation (Seatbelt / Landlock / bwrap / WSL2) or optional Podman/Docker env; network deny / domain allowlist; four permission modes |
| **Long-horizon sessions** | Turn log + stream events in `history.db`; compaction checkpoints; `RecoverRunning` after crashes; sessions can span days or weeks |
| **Automations & IM** | Cron schedules and webhooks; Feishu / QQ / WeChat / WeCom outbound — progress cards, questions, and approvals in-channel |
| **Your model, your data** | Anthropic-native + OpenAI-compatible (OpenAI, DeepSeek, GLM, Qwen, Kimi, Gemini, Grok, local Ollama…). Everything under `~/.danmo-work/` |

---

## Design

Three ideas run through the whole system (full architecture: [docs/core-design.md](docs/core-design.md)):

1. **Everything is a tool.** Files, shell, web, memory, tables, knowledge, subagents (`delegate_agent`), even humans (`ask_user`) share one interface. The model decides what to call and when.
2. **The model orchestrates.** No developer-written control flow: the lead agent plans, delegates, and checks in with you on its own. Code and long-running work are the same loop at different depths — there is no mode switch.
3. **Logs are state.** Each turn is an append-only trail in SQLite (`history.db`). Compaction checkpoints, crash recovery, and replay are built in — not bolted on.

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

Beyond experts, the same library hosts **skills** (workflows — document-writing, playable-slides, TDD, deep-research…) and **connectors** (MCP integrations). Install from the market, bind per agent, or drop skills into `~/.danmo-work/skills/` or `~/.agents/skills/`.

### Document Stage

Three panes — projects · agent stream · right panel (Plan / Files / Memory / Table Store / Git / Terminal / Trajectory) — around a central **Document Stage** routed by extension: `.md` / `.csv` use built-in editors; `.udoc.json` / `.uslides.json` / `.usheet.json` are editable Univer IR; `.docx` / `.pptx` / `.xlsx` are view-only until converted to IR. In **preview** mode you can click a DOM element, annotate it, and send exact HTML/CSS context to the Composer.

Office files are co-edited as a normal agent turn with a review step:

1. **Intent** — select text, a slide, or cells, type an instruction → `[office-edit]`
2. **Propose** — the agent edits; a snapshot is taken before the turn
3. **Review** — view the diff; keep, revert, or accept individual hunks
4. **Commit** — keep persists, revert restores the snapshot; the trail stays in the Turn Log

### Safety

- **Soft gate** — permission modes (`discuss` / `plan` / `interactive` / `auto`); risky commands and external MCP ask before running
- **Hard sandbox** — OS-level isolation or optional Podman/Docker container env; network `deny`, open, or domain allowlist
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

Open the app, add an LLM API key in the UI (or `~/.danmo-work/config.yaml`), and pick a model. Projects, sessions, and memories land under `~/.danmo-work/` (`work.db` control plane, `history.db` turn log, `store.db` table store).

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

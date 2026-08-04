# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**The open-source AI work agent** — coding-agent-grade loop for long-horizon jobs. Self-hosted, multi-agent, MIT.

Same Agent Loop for code, shell, and Document Stage co-edit (docs / slides / sheets with AI Diff). **Turn Log is state** — resume, replay, or edit a Tool Result and continue. Web · Desktop · CLI/TUI · WeChat / Feishu / WeCom / QQ.

> Code, reports, decks, sheets, demos, automations — on one trail.

![Product tour (EN)](docs/demo/product-tour-en.gif)

[Interactive tour](docs/demo/product-tour.html) · [MP4](docs/demo/product-tour-en.mp4) · [Office co-edit tour](docs/demo/office-coedit-tour.html)?lang=en&tour=1

---

## Why Danmo

| | |
|--|--|
| **Not just coding** | Reports, decks, sheets, and automations on the same trail — coding is a lane, not the ceiling |
| **Document Stage + AI Diff** | Propose → review → Keep / Revert / hunks. Text stays source of truth |
| **Turn Log = state** | Every Tool Call is persisted — recover and continue, don’t restart the chat |
| **Lighter main chain** | Expert packs stay isolated until `delegate_agent`; KV-cache-friendly prefixes → fewer tokens, less history cut |
| **IM on the same loop** | WeChat · Feishu · WeCom · QQ — outbound connect, no public-webhook hacks |
| **MIT, self-hosted + MCP** | Data under `~/.danmo-work/`; bring Anthropic / OpenAI-compatible models; extend with MCP |

Mainstream: developer orchestrates, LLM executes.  
**Danmo Work:** the LLM orchestrates on one chain; you supply Tools; humans join via `ask_user`.

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

### From source

Needs sibling [`dq-ui`](https://github.com/danmo-ai/dq-ui). Then:

```bash
make dev-web   # → http://localhost:5801/app/
```

Add an LLM API key in the UI or `~/.danmo-work/config.yaml`. See [Development](#development).

---

## See it

Three panes: project sidebar · agent Stream · right panel (Plan / Files / **Memory** / Changes / Terminal). Center **Document Stage** switches toolbar by file kind.

**Human ↔ AI Office co-edit** (not silent overwrite, not CRDT multiplayer):

1. **Intent** — selection / slide / cell range + instruction → `[office-edit]`
2. **Propose** — same Agent Loop; pre-turn snapshot
3. **Review** — View Diff · Keep · Revert · accept hunks
4. **Commit** — Keep keeps SoT; Revert restores snapshot; trail in Turn Log

**Surfaces:** Web · Desktop (Tauri) · CLI · TUI · Document Stage · IM channels.  
**Channels:** Feishu, QQ, Weixin, WeCom — same Agent Loop; tools still run on your machine. Details: [docs/core-design.md](docs/core-design.md) §12.

In Preview, click a DOM element, annotate, send to Composer — the model gets exact HTML/CSS context.

---

## Experts

System prompts and tool schemas ride every model call. Builtin expert packs (skill + bound-only MCP when needed) are **not** ambient on the main chain — the lead agent summons them with `delegate_agent`, so specialist context stays isolated and the stable prefix stays KV-cache friendly.

| Expert | Role |
|--------|------|
| **CodeGraph** | Local code intelligence via [CodeGraph-Rust](https://github.com/sunerpy/codegraph-rust). Install from **Market** (expert pulls skill + connector; connector **deps script** fetches ~10 MB CLI). First `delegate_agent` inits the index; degrades to `read_file` / `grep` until ready. |
| **GitHub** | GitHub platform work — issues, pull requests, Actions, releases, and related hosting tasks |
| **Danmo Make** | Local image / video / audio generation (separate app) |

Also: Skills (`@` in Composer), MCP connectors, Memory (`memory_*`), Table Store (`table_*`), cron/webhook Automations. Configure in the UI — capability units, not a workflow graph.

---

## Development

**Prerequisites:** Go 1.26+, Node.js 20+, sibling [`dq-ui`](https://github.com/danmo-ai/dq-ui).

```bash
make dev-web          # backend :7801 + Vite :5801 → http://localhost:5801/app/
make stop             # stop all DQ_DEV processes
mkdir -p ~/.danmo-work && cp config.example.yaml ~/.danmo-work/config.yaml
```

Build, pack, test, env vars, and CI: see [`AGENTS.md`](AGENTS.md). Architecture: [`docs/core-design.md`](docs/core-design.md).

---

## Docs

| Doc | Description |
|-----|-------------|
| [docs/core-design.md](docs/core-design.md) | Agent architecture, channels, Document Stage |
| [docs/demo/README.md](docs/demo/README.md) | Product tour HTML / GIF / MP4 |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Harbor Terminal-Bench 2.0 |
| [AGENTS.md](AGENTS.md) | Contributor quick reference |
| [config.example.yaml](config.example.yaml) | Full config reference |

## License

[MIT](LICENSE)

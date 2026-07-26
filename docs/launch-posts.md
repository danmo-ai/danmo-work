# Launch posts (copy-paste)

Short drafts for cold-start distribution.
Repo: https://github.com/danmo-ai/danmo-work  
Latest release: https://github.com/danmo-ai/danmo-work/releases/latest

---

## Product tour (shareable, bilingual)

**Not video-only** — open the HTML for ZH/EN toggle; attach MP4/GIF when posting.

- Interactive: [`docs/demo/product-tour.html`](../demo/product-tour.html)?lang=zh  
- ZH GIF / MP4: [`product-tour-zh.gif`](../demo/product-tour-zh.gif) · [`product-tour-zh.mp4`](../demo/product-tour-zh.mp4)  
- EN GIF / MP4: [`product-tour-en.gif`](../demo/product-tour-en.gif) · [`product-tour-en.mp4`](../demo/product-tour-en.mp4)  
- Capacity-only (older): [`work-capacity-demo.html`](../demo/work-capacity-demo.html)

Covers: architecture · Document Stage / multi-agent / Turn Log / Memory·Table / MCP / IM · capacity scenarios.

Post tip: attach `product-tour-zh.mp4` (or GIF) above the repo link.

---

## GitHub About (paste into repo description)

**EN (≤350 chars):**
```text
Open-source AI Work Agent — coding-agent-grade loop for long-horizon work: code, docs, slides, sheets, MCP & IM on one trail. Self-hosted, MIT.
```

**Topics:** keep `coding-agent` + `work-agent`; add `self-hosted`, `mcp`, `desktop-app`, `feishu`, `wechat`, `ai-workspace` as fits.

---

## 即刻 / 朋友圈 / 微信群（短）

开源了 **Danmo Work**——AI Work Agent：一流 Coding Agent 量级的执行底盘，面向长程工作，不只是又一个写代码的 CLI。

同一思维链多 Agent；一切皆工具（含 `ask_user`）；Turn Log 可恢复可回放。Document Stage 直接改文档/幻灯片/表格；微信·飞书·企微·QQ 同一套 Loop，工具在本机跑。

MIT，可自托管。macOS：`brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git && brew install --cask danmo-work`，或 https://github.com/danmo-ai/danmo-work/releases/latest

---

## V2EX / 掘金 / 少数派（中）

**标题：** 开源 AI Work Agent：一流 Coding 底盘，更大的是长程工作

主流开源 Agent 多半停在终端写代码。Danmo Work 保留同级执行 Loop，面向**长程工作产物**——代码、调研、文档、幻灯片、连接器、IM。

- **纯 LLM 驱动**：无 Graph / Mode；`delegate_agent` 同一思维链硬隔离
- **Document Stage**：文档 / Markdown 幻灯片 / 表格 / 预览点选批注
- **Memory + Table Store**：可检视记忆 + schema-free 业务流水（独立 `store.db`）
- **MCP 连接器 + 自动化**：目录、密钥、权限；cron/webhook 真开 Turn
- **微信 · 飞书 · 企微 · QQ**：出站接入，无需公网回调

macOS / Windows / Linux，MIT。

macOS（Apple Silicon）：
```bash
brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git
brew install --cask danmo-work
```
或 DMG：https://github.com/danmo-ai/danmo-work/releases/latest

https://github.com/danmo-ai/danmo-work

欢迎试用、提 Issue；也求轻拍 Star。

---

## Reddit / X (EN, short)

**Danmo Work** — open-source AI *Work* Agent. Coding-agent-grade loop, wider job: code + docs/slides/sheets + MCP + IM on one trail.

Pure LLM-driven. Sub-agents & humans are Tools. Turn Log → resume/replay. Self-hosted, MIT.

macOS: `brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git && brew install --cask danmo-work`

https://github.com/danmo-ai/danmo-work

---

## Show HN (draft)

**Show HN: Danmo Work – open-source AI Work Agent for long-horizon tasks**

Most OSS agents optimize for writing code. Danmo Work keeps a coding-agent-grade loop, then runs the rest of the work on the same trail: the model plans Tool Calls; `delegate_agent` and `ask_user` are tools; every call is logged for resume/replay. Document Stage edits docs/slides/sheets in-project. MCP connectors, schema-free table store, and IM channels (WeChat/Feishu/WeCom/QQ) share the same Agent Loop.

macOS (Apple Silicon): `brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git && brew install --cask danmo-work`

Repo: https://github.com/danmo-ai/danmo-work

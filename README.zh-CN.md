# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**开源 AI Work Agent** —— 具备一流 Coding Agent 的执行底盘，面向长程工作。可自托管，多 Agent 协作，MIT。

不只是又一个强大的 Coding Agent，也不是要你维护的工作流图。Danmo Work 是**人机共思工作台**：同一条 Agent Loop 既能做文件 / Shell / 多 Agent 编码，也能交付**文档、幻灯片、表格、连接器与 IM**——每一步 Tool Call 全量落盘，**可恢复、可回放、可改结果再继续**。

> 代码、调研报告、幻灯片、表格、演示页、自动化流水——桌面 / Web / CLI·TUI，或微信 / 飞书 / 企微 / QQ——同一条思维链。

| | |
|--|--|
| **产品定位** | 通用型 **Work Agent**——Coding 是一等车道，不是天花板 |
| **控制方式** | **纯 LLM 驱动**——无人工维护的 Graph / 角色路由 / 产品 Mode |
| **抽象** | **一切皆工具**——`delegate_agent`、`ask_user`、记忆、Table Store、MCP、文件… |
| **状态** | **日志即状态**——Turn Log → 断点恢复、完整回放、改结果继续 |
| **触达面** | Web · 桌面 · CLI · TUI · IM 通道 · Document Stage |

MIT · Anthropic 与 OpenAI 兼容接口 · 数据默认在 `~/.danmo-work/`

---

## 30 秒看懂为什么不一样

当下开源 Agent 多半停在**写代码**：终端结对、IDE 插件、沙箱里的软件工程师——Loop 很强，主业很窄。

Danmo Work 保留**一流 Coding Agent 量级**的执行底盘，再问更大的问题：**人和模型如何在真实工作里共思**——代码、调研、文档、幻灯片、运维、连接器——跨长程，且留下可信任的执行轨迹？

| 你得到 | 而不是 |
|--------|--------|
| 同一思维链 + 硬隔离子 Agent | 并行 Session / 不透明 Handoff |
| Document Stage（文档 / 幻灯片 / 表格 / 预览） | 聊天框里倒一堆 Markdown |
| 可检视 Memory + schema-free Table Store | 黑盒产品记忆，或再接一套向量库 |
| MCP 连接器 + cron/webhook 自动化 | 循环外硬拼脚本 |
| 微信 · 飞书 · 企微 · QQ 同一 Loop | 「先公网回调再说」的 IM 接入 |
| 恢复 / 回放 / 编辑 Tool Result | 重开对话碰运气 |

**主流：** 开发者或产品编排，LLM 执行。  
**Danmo Work：** LLM 在同一思维链上编排；你提供能力单元；人通过 `ask_user` 对等参与。

---

## 试用

| 平台 | 通道 | 方式 |
|------|------|------|
| **macOS**（Apple Silicon） | **Homebrew**（有 brew 时推荐） | 见下 |
| **macOS**（Apple Silicon） | `.dmg` | [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest) |
| **Windows** | 安装包 `.exe` | [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest) |
| **Linux 服务端** | `.tar.gz` | [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest) |

### macOS — Homebrew

```bash
brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git
brew install --cask danmo-work
```

之后升级：`brew update && brew upgrade --cask danmo-work`。

尚未 Apple 公证：首次请在 Finder 中右键 app → **打开**（或到「系统设置 → 隐私与安全性」允许）。

### macOS — DMG

从 [Releases](https://github.com/danmo-ai/danmo-work/releases/latest) 下载 `Danmo.Work_*_arm64.dmg`，拖入「应用程序」，首次右键 → 打开。

### 源码运行

需同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)：

```bash
make dev-web   # → http://localhost:5801/app/
```

在 UI 填 LLM API Key（或编辑 `~/.danmo-work/config.yaml`）。完整步骤见 [快速开始](#快速开始)。

---

## 适合谁

- 需要 Agent 交付**工作产物**（报告 / 幻灯片 / 演示），而不只是 PR 的人
- 想要一流 Coding Agent 量级的 CLI/TUI，又希望**同一条 Loop 覆盖 Diff 之后的工作**的开发者
- 内网环境要接飞书 / 企微 / QQ、**不想暴露公网回调**的团队
- 希望 Memory、Table Store、Turn Log **看得见、改得动**的重度用户
- 受够 Graph / Role 框架与模型规划「打架」、只想给模型喂 Tools 的人

---

## Work Agent，而不只是 Coding —— 差异对比

典型开源 Coding Agent 擅长**以代码为中心的循环**。Danmo Work 的 Loop 在同一量级——再在同一条思维链上叠加**工作运行时 + 人机共思体验**。

| 维度 | 典型开源 Coding Agent | Agent 框架（LangGraph / CrewAI / AutoGen） | **Danmo Work** |
|------|----------------------|---------------------------------------------|----------------|
| 主业 | 写代码、PR、终端 | 应用 / 工作流编排 | **长程工作（含 Coding）+ 产物** |
| Agent Loop | 强，偏代码 | 开发者写 Graph / 角色 | **一流 Coding 量级 + 纯 LLM 规划 Tool Call** |
| 子 Agent | 额外 Session 或 Skill | Handoff / Crew | 同一思维链上的 `delegate_agent`，硬隔离 |
| 人机 | 审批 / 聊天 | 预设节点 | `ask_user` Tool——模型决定时机 |
| 产物 | 仓库 Diff | 应用自定义 | Diff **+ Document Stage**：文档 · 幻灯片 · 表格 · 预览 |
| 记忆 | 产品私有或没有 | 缓冲 / 外部向量库 | 显式 `memory_*` + 作用域 SQLite + UI |
| 业务数据 | 文件 / 自建库 | LangGraph Store 等 | 内置 **Table Store**（`store.db`，schema-free） |
| 连接器 | MCP / 插件（参差） | 自建 | MCP 目录 + 密钥 + 权限 + 自动化 |
| IM 入口 | 少 / 自建 | 少 | **微信 · 飞书 · 企微 · QQ**（出站 WS） |
| 持久化 | Session / 容器 | 可选 Checkpointer | **Turn Log = 状态**（恢复 · 回放 · 编辑） |
| 许可 / 部署 | 多为 OSS | OSS 库 | **MIT，自托管**，Web/桌面/CLI/TUI |

日常写代码 → 用 CLI/TUI 当 Coding Agent。  
任务变成**代码之外的工作**——文档、幻灯片、数据、连接器、IM → 留在同一个 Work Agent 里。

---

## 产品价值

1. **交付工作，而不只是对话** —— Stage 原生文档、Markdown 幻灯片、表格、HTML 预览，落在项目文件系统里  
2. **透明建立信任** —— Tool Call 全量持久化；中途恢复；改 Tool Result 再继续  
3. **能力扩展不叠加复杂度** —— 新能力 = 新 Tool / Skill / MCP，不是新的图编排语言  
4. **人在哪聊，Agent 就在哪** —— 手机微信或飞书卡片同一 Loop；工具仍在本机执行  
5. **本地优先、数据自持** —— 配置、库、Turn Log、密钥在 `~/.danmo-work/`；自带模型 Key  

---

## 界面一览

架构 · 亮点 · 产能——双语动画演示（优先 HTML 可切换语言；GIF/MP4 用于传播）：

![产品演示（中文）](docs/demo/product-tour-zh.gif)

交互版（中/EN 切换）：[`docs/demo/product-tour.html`](docs/demo/product-tour.html) · [MP4](docs/demo/product-tour-zh.mp4)

三栏工作台：项目侧栏 · Agent 执行 Stream · 右侧面板（计划 / 文件 / **记忆** / 变更 / 终端）。中间 **Document Stage** 按文件类型换 toolbar。

### Document Stage — 文档 / 幻灯片 / 表格 / 预览

从 Files 打开项目文件 → 进入中间 Stage。可编辑类型走分格式编辑器 + AI；通用 HTML / 图片 / 外链走 **Preview**（URL 栏 + Design mode）。AI 润色走普通 **session turn**。

| 类型 | 真相源 | 编辑器 / 视图 |
|------|--------|---------------|
| **Doc** | GFM `.md` | TipTap（编辑会话内 MD ↔ HTML） |
| **Slides** | 以 `---` 分页的 Markdown | 编辑 Markdown + 播放 HTML |
| **Sheet** | `.csv` / `.danmo-sheet.json` | 表格网格 |
| **Preview** | 通用 `.html`、图片、外链等 | iframe / 图片；点选元素进 Composer |

工具栏组装 `[office-edit]` prompt → `POST /sessions/:id/turns`。AI 前自动保存脏内容；作用域可为选区 / 全文 / 当前页 / 整表。Turn 结束后 Stage 重载并恢复滚动（幻灯片保留页码）。

### 点选页面，直接说改哪里

在 Stage **Preview** 里点选 DOM 元素，写批注，确认注入 Composer。模型带着精确 HTML/CSS 上下文去改——**点选 → 批注 → 修改**。

![网页元素批注](docs/screenshots/ui-browser-annotate.png)

| 调研报告 | 交互演示 | 网页小游戏 |
|---------|---------|-----------|
| ![市场报告](docs/screenshots/ui-market-report.png) | ![烹饪演示](docs/screenshots/ui-cooking-demo.png) | ![贪吃蛇](docs/screenshots/ui-snake-game.png) |

- **调研报告** — 网页抓取、结构化写作、HTML 实时预览  
- **交互演示** — 分步演示，含播放控制  
- **网页小游戏** — 生成可玩页面，再用元素批注迭代  

### 通道（微信 · 飞书 · 企业微信 · QQ）

在 IM 里调用同一套 Agent Loop——工具在本机跑，Turn Log 留在 Teams。会话按 `(频道, 账号, peer)` 隔离，多通道绑同一项目也**不会串会话**。

| 通道 | 接入方式 | 要点 |
|------|----------|------|
| **微信** | 手机微信（iLink 长轮询） | 账号默认项目 + `/project`；文字菜单审批；图片/文件入站 |
| **飞书** | 出站 WebSocket（无需公网 URL） | 卡片/表单、审批、进度、媒体、`/project` |
| **企业微信** | 出站 WebSocket | 管理后台智能机器人 → 设置；先流式占位再最终回复 |
| **QQ** | 出站 Gateway WebSocket | 键盘审批、C2C 流式、群禁止工具、`/project` |

| 桌面端（微信会话） | 手机端（微信对话） |
|-------------------|-------------------|
| ![Teams 中的微信会话](docs/screenshots/wx1.png) | ![微信里的 DQ-Teams AI](docs/screenshots/wx2.png) |

### 专家、技能、连接器与数据面

在 UI 里编辑提示词、Agentskills（`SKILL.md`）、沙箱与委派——交给模型的是**能力单元**，不是工作流图。Composer 可用 `@` / 按钮召唤技能。

| 专家提示词 | 技能库 | 运行时与沙箱 |
|-----------|--------|-------------|
| ![Explorer 系统提示词](docs/screenshots/ui-expert-prompts.png) | ![playable-slides 技能](docs/screenshots/ui-skill-editor.png) | ![运行时设置](docs/screenshots/ui-runtime-settings.png) |

- **专家团** — 本地 + 市场 Agent；概览 / 提示词 / 技能 / 工具 / 知识库  
- **技能库** — 内置与自定义 Agentskills；指令、文件、工具绑定  
- **连接器（MCP）** — 目录（Composio / OpenConnector / GitHub / Notion / 飞书…）；`tools/list` → `mcp_<server>_<tool>` 挂进 Loop；加密 secrets；`external` 风险 → 权限门禁  
- **自动化** — cron / webhook 真正开 session turn，可 Turn Log 回放  
- **记忆** — `memory_update` / `memory_read`（作用域 user · project · agent）；记忆 Tab  
- **Table Store** — schema-free `table_*`，独立 `store.db`，存摘要流水、计数、游标（≠ Memory，≠ 文件）  
- **运行时** — Turn 上限、Tool 输出硬上限（`runtime.tools.max_output_chars`，默认 50k）、最大委派深度、记忆 TopK、OS 沙箱与网络策略  

---

## 设计哲学

### 一切皆工具（Everything is a Tool）

| 传统概念 | 统一抽象 |
|---------|----------|
| Sub-Agent 委派 | `delegate_agent` |
| 用户交互 | `ask_user` |
| 技能 | `read_skill` / Skill 绑定 |
| 知识检索 | `search_kb` |
| 持久记忆 | `memory_update` / `memory_read` |
| 业务流水 | `table_upsert` / `table_query` / … |
| 文件 | `read_file` / `write` / `edit` / … |
| 外部 API | `http_request` / MCP / `web_fetch` · `web_search` |

一种抽象（Tool），一种循环（Agent Loop），一种执行存储（Turn Log）。新能力 = 新 Tool。

### 纯 LLM 驱动（Pure LLM-Driven）

没有开发者维护的 Graph、角色路由或 mode 开关——模型在同一条 Loop 上规划 Tool Call：

```
用户输入
    ↓
[LLM] → 规划 Tool Call DAG
    ↓
逐 Tool 执行（Agent Loop）
    ↓
需要澄清？→ ask_user
    ↓
需要记忆？→ memory_*  |  需要流水？→ table_*
    ↓
需要委派？→ delegate_agent
      → 新 Turn（system + goal；不继承父对话）
      → 独立工具 / 技能 / 知识库 → 同一套 Agent Loop
      → 只回 Report → 父 Agent 继续
    ↓
完成 → 交付结果
```

Code 与 Work 是配置与 `ask_user` 默认值的自然表现——无需显式 `mode`。

### 日志即状态（Log is state）

- 每步 Tool Call（输入、输出、耗时、决策理由）完全持久化  
- 异常可恢复——任意步骤重试  
- 完整回放，便于调试与审计  
- 人工可编辑任意 Tool Result，Agent 从该点继续  

### Memory / Table Store / Knowledge 分家

| 存储 | 职责 |
|------|------|
| **Memory** | 模型主动保留的持久偏好 / 约定 |
| **Table Store** | 可查询业务行（摘要、游标）于 `store.db` |
| **Knowledge** | 人工维护、绑定到 Agent 的文档（`search_kb`） |
| **Compaction** | 上下文截断时的会话内摘要——不是长期记忆 |

---

## 概念模型

```
Project/
  └── Session（长程任务，跨天/周）
        ├── Turn-1  ← 一轮 [输入 → Agent 应答]
        │     ├── Step: LLM 调用 (function calling)
        │     ├── Step: Tool 执行 → 结果注入
        │     └── ...
        ├── Turn-2  ← 几天后追问
        ├── ~ Checkpoint（压缩锚点）~
        └── Turn-N
```

| 概念 | 定义 |
|------|------|
| **Project** | 任务集合，绑定文件系统目录 |
| **Session / Task** | 围绕一个目标的多轮交互 |
| **Turn** | 一轮 [输入 → Agent 应答]，内含 N 个 LLM Step |
| **Step** | Turn 内一次 LLM 请求+响应 |
| **委派 Agent** | `delegate_agent` Tool；子隔离执行，Report 回传 |
| **ask_user** | 向用户提问也是 Tool；暂停等待回复 |
| **Memory / Table Store** | 跨会话事实 vs schema-free 业务行 |

---

## 架构

```
server/   cli/   tui/    frontend/ (Vue 3 + Vite)
    \       \     /       /
     \       \   /       /
      ---- core/bootstrap ----
              |
  core/service ─── core/runtime ─── core/adapter
       |              |                 |
  core/port ←─────────┘    core/adapter/llm
       |                  (Anthropic / OpenAI 兼容 / Mock)
  core/store/sqlite + turnlog + store.db
```

| 层 | 目录 | 说明 |
|----|------|------|
| 入口 | `server/` `cli/` `tui/` | HTTP (Gin) / CLI / TUI |
| 前端 | `frontend/` | Vue 3 + Vite + Document Stage |
| 启动 | `core/bootstrap/` | DI、配置 |
| 服务 | `core/service/` | Session、Project、Agent、Skill、MCP、通道… |
| 运行时 | `core/runtime/` | Turn 循环、Prompt、压缩、权限、Tool |
| 领域 / 端口 | `core/domain/` `core/port/` | 实体与接口 |
| 适配 | `core/adapter/` | LLM + IM（飞书 / QQ / 微信 / 企微） |
| 存储 | `core/store/` | SQLite（`work.db`）+ Turn Log + Table Store（`store.db`） |

深入阅读：[`docs/core-design.md`](docs/core-design.md)。

---

## 前置条件

- Go 1.26+
- Node.js 20+（前端 / 桌面）
- 同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)（`file:../../dq-ui/packages/*`）

```text
Workspace/
  Danmo-Work/
  dq-ui/
```

## 快速开始

```bash
make dev-web          # 后端 :7801 + Vite :5801 → http://localhost:5801/app/
make dev-desktop      # 后端 + Tauri 桌面
make backend          # 纯后端（方便调试器）

make dev-cli          # 命令行（无需 server）
make dev-tui          # 终端界面（无需 server）
make stop             # 停止所有 DQ_DEV 进程
```

```bash
mkdir -p ~/.danmo-work
cp config.example.yaml ~/.danmo-work/config.yaml
# 在 UI 或配置文件中填入 LLM Provider API Key
```

## 构建与打包

```bash
make build-all              # 前端 dist + Go server/cli/tui
make build-go               # 仅三件套 Go 二进制
make pack-macos-desktop     # .dmg / .app
make pack-linux-server      # tar.gz
make pack-windows-desktop   # .exe
make clean                  # 删除 out/
```

```text
out/
  frontend/dist/     # Vite 生产构建（挂载于 /app/）
  server/            # danmo-work / danmo-work-cli / danmo-work-tui
  desktop/bundle/    # Tauri 安装包
  dist/              # Linux server 发布包
  run/               # 开发用 pid / log / wrappers
```

## 测试

```bash
make test               # 分层检查 + go test ./...
make test-integration   # 集成测试
```

### Harbor Agent 对比（Terminal-Bench 2.0）

官方 **terminal-bench@2.0**（**89** 题）。**题不进 git**，本机同步后用 Harbor + Podman。通过 = Mean reward ≥ 1.0。

步骤：[`evals/dq_harbor/README.md`](evals/dq_harbor/README.md) · 成绩：[`evals/dq_harbor/COMPARE_RESULTS.md`](evals/dq_harbor/COMPARE_RESULTS.md)。

```bash
podman machine start                                    # macOS 如需要
make eval-harbor-base
GH_TOKEN=$(gh auth token) make eval-harbor-sync-tb2
make eval-harbor-bin
export WORK_MODEL=deepseek/deepseek-v4-flash WORK_API_KEY=... WORK_BASE_URL=https://api.deepseek.com
make eval-harbor-smoke
./evals/dq_harbor/compare_agents.sh                     # 对比 OpenCode / OpenHands
```

## 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `WORK_CONFIG` | `~/.danmo-work/config.yaml` | YAML 配置 |
| `WORK_DB_PATH` | `~/.danmo-work/work.db` | 引擎 SQLite |
| `WORK_STORE_DB_PATH` | `~/.danmo-work/store.db` | Agent Table Store |
| `WORK_DATA_DIR` | `~/.danmo-work/data` | 项目与 turn 日志 |
| `DQ_BACKEND_PORT` | `7801` | 开发后端端口 |
| `DQ_FRONTEND_PORT` | `5801` | 开发前端端口 |
| `VITE_API_BASE_URL` | `""` | 前端 API 基址（空 = 同源） |

### 自定义技能目录

每个 New Turn 扫描 Agentskills（`skill-name/SKILL.md`），**不写 SQLite**，并入 `<available_skills>`：

| 路径 | 范围 |
|------|------|
| `~/.agents/skills/` | 用户 |
| `~/.danmo-work/skills/` | 用户 |
| `<项目根>/.agents/skills/` | 项目 |
| `<项目根>/.danmo-work/skills/` | 项目 |

同名后者覆盖。改磁盘后下一 turn 生效。

## 桌面端（Tauri）

```bash
make dev-desktop
# 或已有外部后端时：
SKIP_BACKEND=1 make dev-desktop
```

## CI / 发布

`.github/workflows/release.yml` — `v*` tag 或 `workflow_dispatch`：

| Job | 产物 |
|-----|------|
| macOS desktop | `out/desktop/bundle/*.dmg`、`*.app` |
| Linux server | `out/dist/danmo-work-linux-*.tar.gz` |
| Windows desktop | `out/desktop/bundle/*.exe` |

**macOS 通道：** GitHub Releases `.dmg`，或 Homebrew cask [`Casks/danmo-work.rb`](Casks/danmo-work.rb)（每次 `v*` 发版自动 bump；用上方长名字 tap 本仓库）。

## 文档

| 文档 | 说明 |
|------|------|
| [docs/core-design.md](docs/core-design.md) | 核心设计：Agent 架构、通道、Stage |
| [docs/agent-table-store-plan.md](docs/agent-table-store-plan.md) | schema-free Table Store（`store.db`） |
| [docs/channel-qq-feishu-plan.md](docs/channel-qq-feishu-plan.md) | QQ / 飞书 / 微信通道（Phase A–C） |
| [docs/launch-posts.md](docs/launch-posts.md) | 社区发帖稿 |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Harbor Terminal-Bench 2.0 |
| [AGENTS.md](AGENTS.md) | 贡献者 / Agent 速查 |
| [config.example.yaml](config.example.yaml) | 完整配置参考 |

## 许可证

[MIT](LICENSE)

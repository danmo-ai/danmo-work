# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**开源 AI Work Agent** —— 一流 Coding Agent 量级的执行底盘，面向长程工作。可自托管，多 Agent，MIT。

人机共思工作台：同一条 Agent Loop 既能做文件 / Shell / 多 Agent 编码，也能在 Document Stage 上**协作编辑文档、幻灯片、表格**（提案 → 审阅 → 保留/回滚）。每一步 Tool Call 全量落盘——可恢复、可回放、可改结果再继续。

> 代码、调研报告、幻灯片、表格、演示页、自动化——Web / 桌面 / CLI·TUI，或微信 / 飞书 / 企微 / QQ——同一条思维链。

| | |
|--|--|
| **产品定位** | 通用型 **Work Agent**——Coding 是一等车道，不是天花板 |
| **控制方式** | **纯 LLM 驱动**——无人工维护的 Graph / 角色路由 / Mode |
| **抽象** | **一切皆工具**——`delegate_agent`、`ask_user`、记忆、Table Store、MCP… |
| **状态** | **日志即状态**——Turn Log → 断点恢复、完整回放、改结果继续 |
| **Office** | Document Stage + AI Diff（保留 / 回滚 / hunk）；文本为真相源 |
| **触达面** | Web · 桌面 · CLI · TUI · IM 通道 · Document Stage |

MIT · Anthropic 与 OpenAI 兼容接口 · 数据默认在 `~/.danmo-work/`

---

## 安装

| 平台 | 包 |
|------|-----|
| **macOS**（Apple Silicon） | Homebrew（推荐）或 `.dmg` |
| **Windows** | 安装包 `.exe` |
| **Linux**（x86_64） | AppImage / `.deb` |

全部二进制：[GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest)。

### macOS

```bash
brew tap danmo-ai/tap
brew install --cask danmo-work
# 升级：brew update && brew upgrade --cask danmo-work
```

备用：`brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git`

或从 Releases 下载 `Danmo.Work_*_arm64.dmg` →「应用程序」。尚未 Apple 公证：首次请右键 → **打开**。

### Windows

下载 `Danmo.Work_*_x64-setup.exe`。启用 Authenticode（[SignPath](docs/windows-authenticode.md)）前，SmartScreen 可能拦截——**更多信息 → 仍要运行**。

### Linux

```bash
chmod +x Danmo.Work_*_amd64.AppImage && ./Danmo.Work_*_amd64.AppImage
# 或：sudo apt install ./Danmo.Work_*_amd64.deb
```

需要带 WebKitGTK 的桌面环境。应用内自动更新走 AppImage 通道。

### 源码运行

需同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)：

```bash
make dev-web   # → http://localhost:5801/app/
```

在 UI 或 `~/.danmo-work/config.yaml` 填入 LLM API Key。详见 [开发](#开发)。

---

## 亮点

| 你得到 | 而不是 |
|--------|--------|
| 同一思维链 + 硬隔离子 Agent | 并行 Session / 不透明 Handoff |
| Document Stage + **AI Diff**（文档 / 幻灯片 / 表格） | 聊天框里倒一堆 Markdown |
| 相对回合前快照的保留 / 回滚 / 分块接受 | AI 静默覆盖、无法撤回 |
| 可检视 Memory + schema-free Table Store | 黑盒记忆或再接一套向量库 |
| MCP + cron/webhook 自动化 | 循环外硬拼脚本 |
| 微信 · 飞书 · 企微 · QQ 同一 Loop | 「先公网回调再说」的 IM 接入 |
| 恢复 / 回放 / 编辑 Tool Result | 重开对话碰运气 |

**主流：** 开发者编排，LLM 执行。  
**Danmo Work：** LLM 在同一思维链上编排；你提供 Tools；人通过 `ask_user` 对等参与。

| 维度 | 典型 Coding Agent | Agent 框架 | **Danmo Work** |
|------|-------------------|------------|----------------|
| 主业 | 写代码 / PR / 终端 | 工作流图 | 长程工作 + 产物 |
| Loop | 强，偏代码 | 开发者写 Graph / 角色 | Coding 量级 + 纯 LLM 规划 Tool Call |
| 子 Agent | 额外 Session / Skill | Handoff / Crew | 同一链上的 `delegate_agent` |
| 人机 | 审批 / 聊天 | 预设节点 | `ask_user`——模型决定时机 |
| 产物 | 仓库 Diff | 应用自定义 | Diff + Document Stage + AI Diff |
| 持久化 | Session / 容器 | 可选 Checkpointer | **Turn Log = 状态** |
| 部署 | 多为 OSS | OSS 库 | **MIT，自托管** |

---

## 工作台

![产品演示（中文）](docs/demo/product-tour-zh.gif)

[交互演示](docs/demo/product-tour.html) · [MP4](docs/demo/product-tour-zh.mp4) · [Office 共审演示](docs/demo/office-coedit-tour.html)?lang=zh&tour=1

三栏：项目侧栏 · Agent Stream · 右侧面板（计划 / 文件 / **记忆** / 变更 / 终端）。中间 **Document Stage** 按文件类型换 toolbar。

### 人 ↔ AI Office 共审编辑

不是静默覆盖，也不是 CRDT 多人编辑——Document Stage 上四拍：

1. **Intent** —— 选区 / 当前页 / 单元格范围 + 指令 → `[office-edit]`
2. **Propose** —— 同一条 Agent Loop；回合前自动快照
3. **Review** —— 查看 Diff · 保留 · 回滚 · 按 hunk 接受
4. **Commit** —— 保留留下 SoT；回滚恢复快照；轨迹进 Turn Log

| 类型 | 真相源 | 作用域 |
|------|--------|--------|
| **Doc** | GFM `.md` | 选区或全文 |
| **Slides** | 以 `---` 分页的 Markdown | 当前页 |
| **Sheet** | `.csv` / `.danmo-sheet.json` | 选区范围 |
| **Preview** | `.html`、图片、外链 | 点选 DOM → 批注 → Composer |
| **Code / Diff** | 源码 / git / AI Diff | AI Diff 审阅 |

方案说明：[`docs/human-ai-coedit-plan.md`](docs/human-ai-coedit-plan.md)。

### Preview 点选批注

在 Stage **Preview** 里点选 DOM 元素，写批注，注入 Composer——模型带着精确 HTML/CSS 上下文去改。

![网页元素批注](docs/screenshots/ui-browser-annotate.png)

| 调研报告 | 交互演示 | 网页小游戏 |
|---------|---------|-----------|
| ![市场报告](docs/screenshots/ui-market-report.png) | ![烹饪演示](docs/screenshots/ui-cooking-demo.png) | ![贪吃蛇](docs/screenshots/ui-snake-game.png) |

### 通道

在 IM 里调用同一套 Agent Loop；工具在本机跑。会话按 `(频道, 账号, peer)` 隔离，不会串聊。

| 通道 | 接入 | 要点 |
|------|------|------|
| **微信** | iLink 长轮询 | 默认项目、文字菜单审批、媒体入站 |
| **飞书** | 出站 WebSocket | 无需公网 URL；卡片、审批、`/project` |
| **企业微信** | 出站 WebSocket | 智能机器人；流式占位 → 最终回复 |
| **QQ** | 出站 Gateway WS | 键盘审批、C2C 流式、群禁止工具 |

| 桌面端 | 手机端 |
|--------|--------|
| ![微信会话](docs/screenshots/wx1.png) | ![微信对话](docs/screenshots/wx2.png) |

### 专家、技能、连接器

交给模型的是**能力单元**，不是工作流图。Composer 可用 `@` 召唤技能。

| 专家提示词 | 技能库 | 运行时 |
|-----------|--------|--------|
| ![专家](docs/screenshots/ui-expert-prompts.png) | ![技能](docs/screenshots/ui-skill-editor.png) | ![运行时](docs/screenshots/ui-runtime-settings.png) |

- **专家团** —— 本地 + 市场 Agent（提示词 / 技能 / 工具 / 知识库）
- **技能库** —— Agentskills（`SKILL.md`）；内置与自定义
- **MCP** —— 目录 → `mcp_<server>_<tool>`；加密 secrets；权限门禁
- **自动化** —— cron / webhook → 真正的 session turn
- **记忆** —— `memory_*`（user / project / agent）；记忆 Tab
- **Table Store** —— schema-free `table_*`，独立 `store.db`
- **运行时** —— Turn 上限、Tool 输出上限、委派深度、沙箱 / 网络

---

## 设计

### 一切皆工具

| 概念 | Tool |
|------|------|
| 子 Agent | `delegate_agent` |
| 用户交互 | `ask_user` |
| 技能 / 知识 | `read_skill` / `search_kb` |
| 记忆 / 流水 | `memory_*` / `table_*` |
| 文件 / API | `read_file` / `edit` / MCP / `web_*` |

一种抽象，一种循环，一种存储（Turn Log）。新能力 = 新 Tool。

### 纯 LLM 驱动

没有开发者维护的 Graph 或 mode 开关——模型规划 Tool Call：

```
用户输入 → [LLM] 规划 Tool Call DAG → 执行
  → 澄清？ask_user
  → 记忆？memory_*  |  流水？table_*
  → 委派？delegate_agent（隔离子 Agent → Report）
  → 完成
```

### 日志即状态

每步 Tool Call（输入、输出、耗时、决策理由）完全持久化。中途恢复、完整回放，或编辑 Tool Result 再继续。

### Memory / Table Store / Knowledge

| 存储 | 职责 |
|------|------|
| **Memory** | 模型主动保留的持久偏好 / 约定 |
| **Table Store** | 可查询业务行于 `store.db` |
| **Knowledge** | 人工维护的文档（`search_kb`） |
| **Compaction** | 会话内摘要——不是长期记忆 |

### 概念模型

```
Project/
  └── Session（跨天/周）
        ├── Turn-1  ← [输入 → 应答]
        │     ├── Step: LLM 调用
        │     └── Step: Tool 执行 → 结果注入
        ├── Turn-2
        ├── ~ Checkpoint（压缩）~
        └── Turn-N
```

| 概念 | 定义 |
|------|------|
| **Project** | 任务集合，绑定文件系统目录 |
| **Session** | 围绕一个目标的多轮交互 |
| **Turn** | 一轮 [输入 → 应答]，内含 N 个 Step |
| **Step** | Turn 内一次 LLM 请求+响应 |
| **委派 Agent** | `delegate_agent` 隔离子执行，Report 回传 |

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
| 前端 | `frontend/` | Vue 3 + Document Stage |
| 启动 | `core/bootstrap/` | DI、配置 |
| 服务 | `core/service/` | Session、Project、Agent、Skill、MCP、通道… |
| 运行时 | `core/runtime/` | Turn 循环、Prompt、压缩、权限、Tool |
| 领域 / 端口 | `core/domain/` `core/port/` | 实体与接口 |
| 适配 | `core/adapter/` | LLM + IM（飞书 / QQ / 微信 / 企微） |
| 存储 | `core/store/` | `work.db` + Turn Log + `store.db` |

深入阅读：[`docs/core-design.md`](docs/core-design.md)。

---

## 开发

**前置条件：** Go 1.26+、Node.js 20+、同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)：

```text
Workspace/
  Danmo-Work/
  dq-ui/
```

### 快速开始

```bash
make dev-web          # 后端 :7801 + Vite :5801 → http://localhost:5801/app/
make dev-desktop      # 后端 + Tauri 桌面
make backend          # 纯后端
make dev-cli          # 命令行（无需 server）
make dev-tui          # 终端界面（无需 server）
make stop             # 停止所有 DQ_DEV 进程

mkdir -p ~/.danmo-work
cp config.example.yaml ~/.danmo-work/config.yaml
# 在 UI 或配置文件中填入 LLM API Key
```

已有外部后端时：`SKIP_BACKEND=1 make dev-desktop`。

### 构建与打包

```bash
make build-all              # 前端 + Go server/cli/tui
make build-go               # 仅 Go 二进制
make pack-macos-desktop     # .dmg / .app
make pack-linux-desktop     # AppImage / .deb
make pack-windows-desktop   # .exe
make clean                  # 删除 out/
```

```text
out/
  frontend/dist/     # Vite 生产构建（挂载于 /app/）
  server/            # danmo-work / danmo-work-cli / danmo-work-tui
  desktop/bundle/    # Tauri 安装包
  env/               # 可选 OCI agent 环境 tar
  run/               # 开发用 pid / log / wrappers
```

### 测试

```bash
make test               # 分层检查 + go test ./...
make test-integration   # 集成测试
```

Harbor Terminal-Bench 2.0（89 题，本机同步——不进 git）：[`evals/dq_harbor/README.md`](evals/dq_harbor/README.md) · 成绩：[`COMPARE_RESULTS.md`](evals/dq_harbor/COMPARE_RESULTS.md)。

```bash
make eval-harbor-base
GH_TOKEN=$(gh auth token) make eval-harbor-sync-tb2
make eval-harbor-bin
export WORK_MODEL=... WORK_API_KEY=... WORK_BASE_URL=...
make eval-harbor-smoke
./evals/dq_harbor/compare_agents.sh
```

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `WORK_CONFIG` | `~/.danmo-work/config.yaml` | YAML 配置 |
| `WORK_DB_PATH` | `~/.danmo-work/work.db` | 引擎 SQLite |
| `WORK_STORE_DB_PATH` | `~/.danmo-work/store.db` | Table Store |
| `WORK_DATA_DIR` | `~/.danmo-work/data` | 项目与 turn 日志 |
| `DQ_BACKEND_PORT` | `7801` | 开发后端端口 |
| `DQ_FRONTEND_PORT` | `5801` | 开发前端端口 |
| `VITE_API_BASE_URL` | `""` | 前端 API 基址（空 = 同源） |

**自定义技能目录**（Agentskills，每 turn 扫入内存——不写 SQLite）：

| 路径 | 范围 |
|------|------|
| `~/.agents/skills/` · `~/.danmo-work/skills/` | 用户 |
| `<项目>/.agents/skills/` · `<项目>/.danmo-work/skills/` | 项目 |

同名后者覆盖。

### CI / 发布

`.github/workflows/release.yml` — `v*` tag 或 `workflow_dispatch` → macOS `.dmg`/`.app`、Linux AppImage/`.deb`、Windows `.exe`。

macOS 亦可 Homebrew：`brew tap danmo-ai/tap`（[`danmo-ai/homebrew-tap`](https://github.com/danmo-ai/homebrew-tap)；cask：[`Casks/danmo-work.rb`](Casks/danmo-work.rb)）。

---

## 文档

| 文档 | 说明 |
|------|------|
| [docs/core-design.md](docs/core-design.md) | Agent 架构、通道、Stage |
| [docs/human-ai-coedit-plan.md](docs/human-ai-coedit-plan.md) | Office 共审设计 |
| [docs/agent-table-store-plan.md](docs/agent-table-store-plan.md) | Table Store（`store.db`） |
| [docs/channel-qq-feishu-plan.md](docs/channel-qq-feishu-plan.md) | IM 通道 |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Harbor Terminal-Bench 2.0 |
| [AGENTS.md](AGENTS.md) | 贡献者速查 |
| [config.example.yaml](config.example.yaml) | 完整配置参考 |

## 许可证

[MIT](LICENSE)

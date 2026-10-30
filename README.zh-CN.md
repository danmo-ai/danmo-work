# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**开源 AI 工作 Agent**——多 Agent 协作、跑长程任务的本地工作引擎。写代码、做调研、出报告、做幻灯片、填表格、写小说，还能**给在这里开发的 Web 应用跑 UI 测试**，一条 Agent Loop 全包了。你可以全程看着它干，随时纠正，中断了也能接着跑。自托管，MIT 协议。

- **编排交给模型。** 没有工作流画布这回事。子专家（`delegate_agent`）、向你提问（`ask_user`）、MCP 连接器，本质上都是工具——调哪个、什么时候调，模型规划时自己说了算。
- **Turn Log 就是状态。** 每次工具调用写入 SQLite（`history.db`）。进程崩了能恢复、回合能回放、改工具结果、隔几天接着跑——JSONL 仅作导出。
- **入口随你挑。** Web · 桌面（Tauri）· CLI · TUI · 飞书 / QQ / 微信 / 企微——同一套引擎，工具永远跑在你自己的机器上。

[![产品截图轮播 — 多 Agent Plan、Trajectory、File Stage、网页标注、专家、技能库、沙箱、IM](docs/screenshots/carousel.gif)](docs/screenshots/carousel.html?lang=zh)

[交互轮播（带说明）](docs/screenshots/carousel.html?lang=zh) · [架构演示](docs/demo/product-tour.html?lang=zh) · [Office 协作演示](docs/demo/office-coedit-tour.html?lang=zh&tour=1)

> 代码、报告、幻灯片、表格、小说、演示、给你的应用做 UI 测试——一条链做完。

---

## 亮点

| | |
|--|--|
| **多 Agent 团队** | **Team** 主专家按需委派内置专家——文档、实现、调研、评审、浏览器、桌面控制、UI 测试、GitHub、网文……各自在独立 sub-turn 里干活，主链上下文保持精简 |
| **计划 → 执行** | Plan 模式用 `create_plan` / `update_plan` 把方案写进右侧 Plan 面板；点 **执行** 送入 Composer 开工。日常清单仍用 `todowrite` |
| **File Stage + AI Diff** | 文档 / 幻灯片 / 表格 / 源码 / 网页 / 媒体 / Diff 同台——`routeProjectFile` 路由；本地 HTML 走应用内预览；回合前快照 → 审阅 hunk → 保留 / 回滚 / 接受 |
| **小说工作台** | Scrivener 式书架：书 → 卷 → 单元队列、人物墙与关系图、每阶段一个主 CTA。立项 → 规划 → 写作 → 定稿，搭配题材技法库、去 AI 味门禁与分模型回合 |
| **给项目做 UI 测试** | 根据**你项目的代码变更**，**UI Test** 专家自动把 Markdown 用例写进该项目的 `ui-tests/`，再用 **browser use**（`browser_*`）与 **computer use**（`computer`）在 Linux **desktop** 沙箱执行，交回带证据的 PASS/FAIL——测的是你的项目，不是 Danmo Work 自身 |
| **随时向你请示** | `ask_user` 带选项、带表单；危险 Shell、桌面控制或外部 MCP 执行前走审批门禁——桌面与 IM 同一套流程 |
| **记忆、表与知识库** | `memory_*` 按 user / project / agent 三级作用域；无 schema 的 `table_*` 流水在 `store.db`；Markdown 知识库按章节 FTS 检索 |
| **技能、插件与连接器** | 技能 / 插件市场（官方目录、Tech Leads Club、ClawHub、ArtCraft 办公 CLI）；磁盘 Ambient 技能；按 Agent 绑定的 MCP；Codex 风格插件 `hooks.json` |
| **实打实的沙箱** | OS 级隔离（Seatbelt / Landlock / bwrap / WSL2）或 Podman/Docker 容器；`cli` Alpine Shell 或 `desktop`（Xvfb + Chromium CDP + noVNC）配置；网络 deny / 域名白名单；四种权限模式 |
| **长程会话** | Turn Log 与流式事件在 `history.db`；Compaction 检查点；崩溃后 `RecoverRunning`；会话可跨天、跨周 |
| **自动化与 IM** | cron 定时 + webhook 触发；飞书 / QQ / 微信 / 企微出站直连——进度卡片、提问、审批全在聊天里 |
| **模型、数据都是你的** | Anthropic 原生 + OpenAI 兼容，并按厂商绑定 **协议方言**（DeepSeek、GLM（智谱）、通义、Kimi、Gemini、Grok、MiniMax、本地 Ollama……）；数据都在 `~/.danmo-work/` |

---

## 设计

整个系统就三条设计理念（完整架构见 [docs/core-design.md](docs/core-design.md)）：

1. **一切都是工具。** 文件、Shell、浏览器、桌面、记忆、业务表、知识库、子专家（`delegate_agent`），甚至你本人（`ask_user`），共享同一个接口。调谁、何时调，模型说了算。
2. **模型驱动一切。** 没有开发者预先写死的控制流：主专家自己规划、自己委派、自己来找你请示。写代码和跑长程任务，是同一条 Loop 的两种深度，压根不需要「模式切换」。
3. **日志即状态。** 每个 Turn 是 SQLite（`history.db`）里只追加的轨迹。Compaction 检查点、崩溃恢复、回放都是原生能力，不是事后补丁。

### 专家与团队

主专家撑起整个会话，子专家负责专项，随叫随到、上下文隔离（完整清单与用法见 [docs/experts.md](docs/experts.md)）：

| 专家 | 职责 |
|------|------|
| **Team**（主） | 默认可协作；适合跨文件、多步骤任务 |
| **Document** | 职场写作交付：报告默认 Markdown；幻灯片/表格走绑定技能；含邮件/消息润色（原 Comms 已并入） |
| **Implementer** | 按规格改代码（TDD / debugging 技能）；同文件有序改动用 `edit_batch` |
| **Explorer** | 只读摸清代码库 |
| **Researcher** | 深度检索与研究 |
| **Reviewer** | 代码与产物审查 |
| **Browser** | 粘性多步浏览器 Tab（`browser_*`，经 CDP） |
| **Computer** | 桌面 GUI 控制——找窗口、点击、键入、截屏（每次审批） |
| **UI Test** | 代码变更 → 自动写进**你的**项目的 Markdown 用例 → 用 `browser_*` + `computer` 在 Linux desktop 沙箱自动执行 |
| **Data** | CSV / JSON 分析与报表 |
| **GitHub** | Issue / PR / Actions / Release |
| **Danmo Make** | 本机图片 / 视频 / 音频生成（需另装 Danmo Make） |
| **Danmo Novel** | 长篇小说（需另装 Danmo Novel）：立项 → 规划 → 一批细纲 → 写单元 → 定稿（candidate-accept） |
| **CodeGraph**（市场） | 代码智能：定义、引用、影响面 |
| **ArtCraft**（市场） | 无头 Word / Grid / Deck / Pdf 办公 CLI，插件 + MCP 一包装 |

Composer 里 `@` 一下，或者干脆说一句「让文档专家写份报告」，主专家就知道该委派了。勾选专家 chip 走 **透传（relay）**——主专家原文转发你的目标；不勾 chip，则由 Team 自己协调拆任务。主专家只看子专家交回来的报告，主链上下文保持精简、对 KV Cache 友好。想在 Teams 里自建子专家也行，技能、工具、知识库、连接器随你绑。

专家之外，同一个库里还装着**技能**（工作流：document-writing、playable-slides、TDD、deep-research、writing-plans……）和**连接器**（MCP 集成）。可以从市场装（推荐 `kind: plugin`），可以按 Agent 绑，也可以把技能丢进 `~/.danmo-work/skills/` 或 `~/.agents/skills/`。内置子专家以插件形式落在 `~/.danmo-work/plugins/`，可带 Codex 风格 `hooks.json` 做回合上下文注入。

### Plan 模式

在 Composer 打开 **Plan**：本回合只读（外加 `create_plan` / `update_plan`）。模型把 Markdown + todos 写进右侧 **Plan** 面板；点 **执行** 退出计划模式并预填 Composer，按谈妥的方案开工——不是聊完就散的一坨文字。

### File Stage

三栏布局：项目 · Agent 执行流 · 右侧面板（计划 / 文件 / 记忆 / 表存储 / Git / 终端 / 轨迹），正中间是 **File Stage**，由 `routeProjectFile` 按文件路由：`.md` / `.csv` 自研编辑；`.udoc.json` / `.uslides.json` / `.usheet.json` 为 Univer IR 可编辑；`.docx` / `.pptx` / `.xlsx` 只读，转为 IR 后可编；源码进 CodeMirror；HTML / 图片分别为 **web** / **media**（本地页面走应用内预览路由）。网页上还能点选 DOM 元素做标注，发回 Composer。

改 Office 文件 = 一次普通 Agent 回合 + 一道审阅：

1. **意图** — 选中文字 / 一页 / 几个单元格，写指令 → `[office-edit]`
2. **提案** — Agent 动手改；回合开始前自动留快照
3. **审阅** — 看 Diff：保留、回滚、按 hunk 接受，都行
4. **提交** — 保留即落盘，回滚还原快照；全程轨迹留在 Turn Log

### 小说工作台

会话顶栏点 **工作台**（或 Composer 输入 `/novel`）。布局与 Office 相同，专为长篇创作：

- **左** — 书 → 卷 → 单元队列（状态 · `unit_id` · 章范围 · 一句话功能）
- **中** — 卷索引 / 单元细纲卡 / 正文 / **人物墙** / 可缩放 **关系图**
- **右** — 按磁盘状态推断的唯一主按钮 + 注入预览（题材文 / 单元卡 / on_stage 人物）
- **底** — Composer 通栏；写作与定稿分 turn，方便你自己换模型

流水线：`novel-setup` → `novel-plan`（人物 + 大纲，卷纲需人批准）→ `novel-write`（细纲 / 正文，CONTEXT 打包落盘）→ `novel-review`（qc-pack → 审 → Commit）。技法在 `kb-novel-craft`（共性篇 + 八题材专文，刑侦探案可追加人味文风）；机器门禁盯住连续性、字数配额与去 AI 味。

### 给在这里开发的应用做 UI 测试

Danmo Work 既是车间，也是**按代码变更驱动**的 UI 测试跑手：

1. **先在项目里做出 UI** — 页面 / SPA 照常交付（Implementer / Team）
2. **按变更测** — 例如说「根据这次改动写 UI 测试并跑一遍」，或 `@` **UI Test**
3. **从 diff 自动写用例** — 专家读 git/工作区变更，映射 UI 影响面，把 Markdown 写进/更新到项目的 `ui-tests/specs/`
4. **browser + computer 自动执行** — 优先 `browser_*`（无障碍 ref）；布局 / canvas 用 `computer`（`runtime.sandbox.profile=desktop`）
5. **证据交回** — `ui-tests/runs/<stamp>/report.md`，按场景 PASS/FAIL（可附截图）

应用入口走 `/api/v1/proxy/…`，静态页走 `/api/v1/projects/<id>/raw/…`。测的是用 Danmo Work 开发的应用，不是给 Danmo Work 本仓库做 CI。不是 Playwright 脚手架；只做 Web。详见 [docs/experts.md](docs/experts.md)、[docs/agent-desktop-sandbox.md](docs/agent-desktop-sandbox.md)。

### 安全

- **软门禁** — 四种权限模式（`discuss` / `plan` / `interactive` / `auto`），危险命令、桌面控制与外部 MCP 先问再跑
- **硬沙箱** — OS 级隔离或 Podman/Docker 容器（`cli` / `desktop` 配置）；网络 `deny`、全开或域名白名单
- `auto_approve` 绝不会静默放行危险命令；IM 里同样能审批

### 通道、远程与自动化

- **IM 同一条 Loop** — 飞书、QQ、微信、企微都是出站直连你自己的账号，不用折腾公网回调；进度卡片、提问、审批全在聊天里呈现，工具照旧跑在本机。
- **Remote Hub** — 配合 [danmo-hub](https://github.com/danmo-ai/danmo-hub) 配对，让远端客户端穿透 NAT 驱动这台电脑。
- **自动化** — cron 定时、webhook 触发，人不在也能让会话自己跑起来。

---

## 安装

| 平台 | 方式 |
|------|------|
| **macOS**（Apple Silicon） | Homebrew 或 `.dmg` |
| **Windows** | 安装包 `.exe` |
| **Linux**（x86_64） | AppImage / `.deb` |

安装包见 [GitHub Releases](https://github.com/danmo-ai/danmo-work/releases/latest)。

### macOS

```bash
brew tap danmo-ai/tap
brew install --cask danmo-work
# 升级：brew update && brew upgrade --cask danmo-work
```

或者 `brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git`，也可以直接从 Releases 下载 `Danmo.Work_*_arm64.dmg`。目前还没过 Apple 公证，首次打开请右键 → **打开**。

### Windows / Linux

- **Windows：** `Danmo.Work_*_x64-setup.exe` — Authenticode 签名启用前，SmartScreen 可能拦一下 → **更多信息 → 仍要运行**。
- **Linux：** `chmod +x Danmo.Work_*_amd64.AppImage && ./…`，或 `sudo apt install ./Danmo.Work_*_amd64.deb`（需要 WebKitGTK）。

### 首次使用

打开应用，在界面（或 `~/.danmo-work/config.yaml`）里填上 LLM API Key、选个模型就行。项目、会话、记忆落在 `~/.danmo-work/`（`work.db` 控制面、`history.db` 回合日志、`store.db` 业务表）。

### 从源码运行

需要同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)：

```bash
make dev-web   # 后端 :7801 + Vite :5801 → http://localhost:5801/app/
```

---

## 开发

**依赖：** Go 1.26+、Node.js 20+、同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)。

```bash
make dev-web          # 后端 :7801 + Vite :5801 → http://localhost:5801/app/
make stop             # 停掉所有 DQ_DEV 进程
mkdir -p ~/.danmo-work && cp config.example.yaml ~/.danmo-work/config.yaml
```

构建、打包、测试、环境变量与 CI 见 [`AGENTS.md`](AGENTS.md)，架构见 [`docs/core-design.md`](docs/core-design.md)。

---

## 文档

| 文档 | 说明 |
|------|------|
| [docs/core-design.md](docs/core-design.md) | Agent 架构、工具系统、通道、File Stage |
| [docs/experts.md](docs/experts.md) | 专家使用说明、内置清单、小说工作台、UI Test |
| [docs/agent-desktop-sandbox.md](docs/agent-desktop-sandbox.md) | Desktop 沙箱配置（Chromium CDP + computer + noVNC） |
| [docs/remote/README.md](docs/remote/README.md) | Remote Hub 配对与隧道协议 |
| [docs/screenshots/README.md](docs/screenshots/README.md) | 产品截图轮播（README 头图） |
| [docs/demo/README.md](docs/demo/README.md) | 架构演示 HTML / GIF / MP4 |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Terminal-Bench 2.0 评测套件 |
| [AGENTS.md](AGENTS.md) | 贡献者速查 |
| [config.example.yaml](config.example.yaml) | 完整配置参考 |

## 许可证

[MIT](LICENSE)

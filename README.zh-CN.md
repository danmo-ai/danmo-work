# Danmo Work

[English](README.md) | [中文](README.zh-CN.md)

[![Release](https://img.shields.io/github/v/release/danmo-ai/danmo-work?label=release)](https://github.com/danmo-ai/danmo-work/releases/latest)
[![License](https://img.shields.io/github/license/danmo-ai/danmo-work)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/danmo-ai/danmo-work?filename=go.mod)](go.mod)
[![Stars](https://img.shields.io/github/stars/danmo-ai/danmo-work?style=social)](https://github.com/danmo-ai/danmo-work)

**开源 AI 工作 Agent** — 以一流 Coding Agent 为底盘，面向长程真实工作。可自托管、多 Agent 协作，MIT 协议。

同一条 Agent Loop：改文件、跑 Shell，也能在 Document Stage 上共改文档 / 幻灯片 / 表格（AI Diff）。**Turn Log 即状态** — 可恢复、可回放，改完 Tool 结果还能接着跑。Web · 桌面 · CLI/TUI · 微信 / 飞书 / 企微 / QQ。

> 写代码、做调研、出幻灯片、填表格、做演示、跑自动化——都在同一条工作链上。

![产品演示（中文）](docs/demo/product-tour-zh.gif)

[交互演示](docs/demo/product-tour.html) · [MP4](docs/demo/product-tour-zh.mp4) · [Office 协作演示](docs/demo/office-coedit-tour.html)?lang=zh&tour=1

---

## 为什么选 Danmo

| | |
|--|--|
| **不只是写代码** | 报告、幻灯片、表格、自动化同一条链——写代码是车道，不是天花板 |
| **Document Stage + AI Diff** | 提案 → 审阅 → 保留 / 回滚 / 分块接受；文本始终是真相源 |
| **Turn Log 即状态** | 每一步 Tool Call 落盘——中断可接着跑，不用重开对话碰运气 |
| **主链更轻** | 专家按需 `delegate_agent`，不往每一轮塞满工具；前缀稳定、利于 KV Cache → 更省 Token，历史也不容易被裁没 |
| **IM 共用同一 Loop** | 微信 · 飞书 · 企微 · QQ — 出站接入，不必折腾公网回调 |
| **MIT 自托管 + MCP** | 数据在 `~/.danmo-work/`；兼容 Anthropic / OpenAI；用 MCP 扩展能力 |

多数产品是人排流程、模型干活。  
**Danmo Work** 反过来：模型在同一条链上自己编排；你提供工具；要人拍板时用 `ask_user`。

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

也可：`brew tap danmo-ai/danmo-work https://github.com/danmo-ai/danmo-work.git`  
或从 Releases 下载 `Danmo.Work_*_arm64.dmg`。尚未 Apple 公证，首次启动请右键 → **打开**。

### Windows / Linux

- **Windows：** `Danmo.Work_*_x64-setup.exe` — Authenticode 启用前 SmartScreen 可能拦截 → **更多信息 → 仍要运行**。
- **Linux：** `chmod +x Danmo.Work_*_amd64.AppImage && ./…` 或 `sudo apt install ./Danmo.Work_*_amd64.deb`（需 WebKitGTK）。

### 从源码运行

需要同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)：

```bash
make dev-web   # → http://localhost:5801/app/
```

在界面或 `~/.danmo-work/config.yaml` 填入 LLM API Key。见 [开发](#开发)。

---

## 看看长什么样

三栏：左侧项目 · 中间 Agent 执行流 · 右侧面板（计划 / 文件 / **记忆** / 变更 / 终端）。中央 **Document Stage** 按文件类型切换工具栏。

**人与 AI 一起改 Office**（不是悄悄覆盖，也不是 CRDT 多人实时）：

1. **意图** — 选区 / 页 / 表 + 指令 → `[office-edit]`
2. **提案** — 同一条 Agent Loop；回合前自动快照
3. **审阅** — 查看 Diff · 保留 · 回滚 · 按 hunk 接受
4. **提交** — 保留即落盘；回滚恢复快照；轨迹进 Turn Log

**入口：** Web · 桌面（Tauri）· CLI · TUI · Document Stage · IM。  
**通道：** 飞书、QQ、微信、企微 — 同一 Agent Loop；工具仍在本机执行。详见 [docs/core-design.md](docs/core-design.md) §12。

预览页可点选 DOM、标注后发到 Composer，模型拿到精确 HTML/CSS 上下文。

---

## 专家

系统提示和工具说明会跟着**每一次**模型调用走。内置专家（自带技能；需要时再挂 MCP）**不会**常驻主链——主 Agent 用 `delegate_agent` 按需请来，专家上下文单独开，主链前缀更稳，也更吃得开 KV Cache。

使用方式（Composer `@` / 专家图标、协作开关、完整清单）：见 **[专家使用说明](docs/experts.md)**。

| 专家 | 作用 |
|------|------|
| **Document** | 报告、幻灯、表格等职场文档（Office Stage 同源 Markdown）。 |
| **Novel Writing** | 长篇/网文工程化写作（章合同 → 草稿 → 审稿 → Commit；需环境已内置或安装 `novel`）。 |
| **GitHub** | 处理 Issue、Pull Request、Actions、Release 等 GitHub 上的日常协作。 |
| **Danmo Make** | 在本机生成或编辑图片、视频、音频（需单独安装 Danmo Make）。 |
| **CodeGraph** | 查定义、调用关系、改动影响面。从**市场**安装专家（连带技能 + 连接器；连接器 **deps 脚本**拉取约 10 MB CLI，[CodeGraph-Rust](https://github.com/sunerpy/codegraph-rust)）。第一次委派时建索引；图还没好也能先靠读文件 / 搜索顶上。 |

另外还有：技能与专家（Composer 里 `@`）、MCP 连接器、记忆（`memory_*`）、Table Store（`table_*`）、cron / webhook 自动化。界面里配的是能力积木，不是工作流画布。

---

## 开发

**依赖：** Go 1.26+、Node.js 20+、同级 [`dq-ui`](https://github.com/danmo-ai/dq-ui)。

```bash
make dev-web          # 后端 :7801 + Vite :5801 → http://localhost:5801/app/
make stop             # 停掉所有 DQ_DEV 进程
mkdir -p ~/.danmo-work && cp config.example.yaml ~/.danmo-work/config.yaml
```

构建、打包、测试、环境变量与 CI 见 [`AGENTS.md`](AGENTS.md)。架构见 [`docs/core-design.md`](docs/core-design.md)。

---

## 文档

| 文档 | 说明 |
|------|------|
| [docs/core-design.md](docs/core-design.md) | Agent 架构、通道、Document Stage |
| [docs/experts.md](docs/experts.md) | 专家使用说明与内置清单 |
| [docs/demo/README.md](docs/demo/README.md) | 产品演示 HTML / GIF / MP4 |
| [evals/dq_harbor/README.md](evals/dq_harbor/README.md) | Harbor Terminal-Bench 2.0 |
| [AGENTS.md](AGENTS.md) | 贡献者速查 |
| [config.example.yaml](config.example.yaml) | 完整配置参考 |

## 许可证

[MIT](LICENSE)

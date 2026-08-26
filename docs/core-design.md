# Danmo Work — Core Design

> 统一 Agent 架构：一切皆工具，模型驱动一切

---

## 1. 项目定位

Danmo Work 是一个**通用型 AI Work Agent**，兼具基本的 AI Coding 能力。引擎核心是**多 Agent 协作**，完成**长程复杂任务**。

**核心设计理念**：一切都是工具，模型驱动一切。人也是工具（通过 `ask_user` 要求用户参与），有自己人设能力的 Sub Agent 也是工具（通过 `delegate_agent` 委派）。所有能力统一为 tool 接口，由模型自主决策调用。

> **不是 Agent 工具，不是 IDE，是「人机共思」的实时协作操作系统。**

| 产品 | 可视化什么 | 协作方式 |
|------|-----------|---------|
| Google Docs | 文档内容 | 多人同时编辑文字 |
| Figma | 设计图层 | 多人同时操作画布 |
| **Danmo Work** | **思维过程** | **多人同时编辑 Agent 的「认知」** |

---

## 2. 核心架构哲学

### 2.1 一切皆工具（Everything is a Tool）

| 传统概念 | 本架构中的统一抽象 |
|---------|------------------|
| Sub-Agent | `delegate_agent` Tool：接收参数，返回结果，可被委派 |
| 用户交互 | `ask_user` Tool：带选项、推荐、字段，返回用户反馈 |
| 技能/能力 | `read_skill` / Skill 绑定：封装特定领域能力 |
| 知识检索 | `search_kb` Tool：知识库检索，返回上下文（章级 BM25） |
| 持久记忆 | `memory_update` / `memory_read`：跨会话事实（user / project / agent） |
| 文件操作 | `read_file` / `write` / `edit` / `apply_patch` / `file_op`（move/copy/delete）/ `exec_shell` |
| 外部 API / 连接器 | `http_request`（通用 REST）/ **连接器**（产品名；实现多为 MCP）/ `web_fetch`·`web_search`；禁止一 API 一 builtin Tool |

**关键设计**：所有 Tool 具有统一接口签名，模型通过 Tool Call 驱动整个系统循环。

### 2.2 模型驱动一切（LLM-Centric Control）

```
用户输入
    ↓
[LLM 解析意图] → 规划 Tool Call
    ↓
逐 Tool 执行（Agent Loop）
    ↓
需要澄清？→ ask_user Tool（上下文决定阻塞/非阻塞策略）
    ↓
需要委派？→ delegate_agent Tool（参数决定资源配额和权限）
    ↓
完成 → 交付结果
```

**核心原则**：LLM 是唯一的决策中心，控制流由模型生成，而非开发者预设。

### 2.3 Tool Call 日志化（Persistent Execution Trace）

- 每一步 Tool Call 的输入、输出、耗时完全持久化（Turn Log JSONL）
- 支持异常恢复：任意步骤失败可从断点重试（`ResumeTurn`）
- 支持可回放：完整执行轨迹可可视化浏览
- 支持人工纠正：权限审批与 `ask_user` 可在任意步骤介入

### 2.4 Code / Work 是伪区分

> **Code 和 Work 不是两种模式，而是同一架构在不同参数配置下的自然表现。**

| 场景 | 参数配置 | 表现特征 |
|------|---------|---------|
| **Code**（实时辅助） | 浅调用、硬超时、无默认推荐、必填字段 | 即时响应，高交互频率 |
| **Work**（后台任务） | 深调用、软超时、有默认推荐、全可选字段 | 异步执行，低交互频率 |

无需显式 `mode` 参数。`ask_user` 的推荐/默认值存在与否即信号——有推荐是 Work 语义（默认继续），无推荐是 Code 语义（必须阻塞）。

---

## 3. 与主流框架的根本区别

### 3.1 控制流：谁在做决策？

| 维度 | 主流框架 | 本架构 |
|------|---------|--------|
| **决策中心** | 开发者写控制流，LLM 填充内容 | LLM 生成控制流，开发者只提供 Tool |
| **编排方式** | 显式图/链/角色定义 | 隐式，模型自主规划 Tool Call |
| **子 Agent 调度** | 开发者配置 Handoff/路由规则 | 模型自主决定何时委派哪个 sub_agent |
| **用户交互时机** | 开发者在特定节点插入交互 | 模型自主决定何时 `ask_user` |
| **异常处理** | 开发者预设重试/降级逻辑 | 模型自主决策，日志支持事后纠正 |

**本质差异**：主流框架是「开发者编排，LLM 执行」；本架构是「LLM 编排，开发者提供能力单元」。

### 3.2 抽象层级

| 框架 | 基本抽象 | 抽象层级 |
|------|---------|---------|
| LangChain | Chain | 高 |
| LangGraph | Graph Node | 高 |
| CrewAI | Role | 高 |
| AutoGen | ConversableAgent | 中 |
| OpenAI Agents SDK | Agent + Handoff | 中 |
| smolagents | Code/Tool Agent | 中 |
| **本架构** | **Tool（唯一抽象）** | **低** |

主流框架在 LLM 之上叠加多层抽象；本架构信任 LLM 的规划能力，只提供最基本的「能力单元」（Tool）。

### 3.3 状态与人机协作

| 维度 | 主流框架 | 本架构 |
|------|---------|--------|
| **状态存储** | 内存为主，可选持久化 | 原生持久化，日志即状态 |
| **可恢复性** | 会话丢失 = 状态丢失 | 任意步骤可恢复，断点续传 |
| **协作模式** | 人下指令，机器执行（主从） | 人进入思维流，共同迭代（对等） |
| **纠正方式** | 外部干预，改代码重跑 | 原生能力：审批 / ask_user / 改结果继续 |
| **信任建立** | 预设规则限制 | 完全透明，随时可纠正 |

### 3.4 为什么主流框架没有走这条路

| 原因 | 说明 |
|------|------|
| **历史惯性** | 从 Chatbot 渐进演化，天然带「对话轮次」基因 |
| **产品定位** | 主流是「辅助工具」，非「自主执行器」 |
| **控制欲** | 开发者/框架设计者想要显式控制流 |
| **基础设施门槛** | 完整可视化 + 可回放需要极强的工程投入 |

---

## 4. 统一概念模型

| 概念 | 定义 |
|------|------|
| **Project** | 任务的集合，绑定一个文件系统目录 |
| **Session** | 围绕一个目标的多轮 Agent 交互，绑定 Project + Agent + Model；可跨数天甚至数周 |
| **Turn** | 一轮 [用户输入 → Agent 应答和输出]，内部包含 N 个 LLM Step（function calling） |
| **Step** | Turn 内的一次 LLM 请求+响应（含 function calling），是 LLM context 的原子单位 |
| **Agent** | 人设 + 技能 + 工具绑定 + 知识库；可 `primary` 或 `subagent`；`canDelegate` 决定能否委派 |
| **委派 Agent** | 委派动作是 `delegate_agent` tool；子 Agent 在隔离 Turn 下运行，最终报告作为 tool result 反馈父 Turn |
| **ask_user** | 向用户提问也是 tool；Agent 调用后暂停等待用户响应，用户输入作为 tool result 继续 |
| **Memory** | Agent 主动写入的跨会话事实；`memory_update` / `memory_read`；作用域 user / project / agent；SQLite `memories` 表（`work.db`） |
| **Table Store** | Agent 业务流水（schema-free）；`table_upsert` / `table_get` / `table_query` / `table_delete` / `table_list`；独立 `store.db` |

**层次关系**：

```
Project/
  └── Session (长程任务，跨天/周)
        ├── Turn-1
        │     ├── Step: LLM 调用 (function calling)
        │     ├── Step: Tool 执行 → 结果注入
        │     └── ...
        ├── Turn-2        ← 用户几天后发起追问
        ├── ~ Checkpoint 压缩锚点 ~
        └── Turn-N

委派:
  Lead Turn (深度=0)
    └── Step: delegate_agent(agent_id, goal)   ← tool call
          └── Sub Turn (深度=1)
                独立执行 → Report 返回父 Turn

ask_user:
  Agent Turn
    └── Step: ask_user(question, [options])    ← tool call
          Agent Loop 暂停 → 用户输入 → 作为 tool result 继续
```

**委派约束**：最大深度可配置（默认 3，`team.maxDelegationDepth`），禁止循环委派。

**三种消息严格区分**：

| 类型 | 作用域 | 持久化 | 对 UI |
|------|--------|--------|-------|
| LLM Message | Session 内 LLM context | 内存（跨 Turn）；压缩后 Checkpoint 落盘 | **绝不直接暴露** |
| Stream Event | 事件流 | SQLite（可选）+ 内存 | SSE / Timeline |
| Turn Log | Turn 持久化 | 文件追加（JSONL） | API 查询 / 恢复 |

---

## 5. 仓库结构与分层

```
Danmo-Work/
├── server/                 # HTTP API 入口 (Gin)
│   ├── main.go
│   └── api/v1/             # REST handlers
├── cli/                    # CLI 入口
├── tui/                    # TUI 入口
├── core/
│   ├── bootstrap/          # DI 装配
│   ├── domain/             # 领域实体
│   ├── port/               # Engine / LLM / Repository 接口
│   ├── service/            # SessionManager、AgentManager、…
│   ├── runtime/            # SessionRunner、TurnRunner、Tool、Permission、Compaction
│   ├── adapter/            # LLM providers、IM 通道、config loader
│   │   ├── llm/            # Anthropic / OpenAI-compat / Mock + tool-args JSON repair
│   │   ├── feishu/ qq/ weixin/ wecom/
│   │   └── config/
│   ├── store/              # SQLite + turnlog
│   └── paths/              # 用户数据目录
├── frontend/               # Vue 3 + Vite（含 Document Stage / office surfaces）
├── desktop/                # Tauri 薄壳
├── scripts/
├── docs/
└── Makefile
```

### 分层依赖

```
server/api  →  core/service  →  core/port
                ↓                 ↓
          core/runtime      core/store (实现)
                ↓
          core/domain / core/adapter
```

| 层 | 目录 | 职责 |
|----|------|------|
| Entry | `server/` `cli/` `tui/` | HTTP / CLI / TUI |
| Bootstrap | `core/bootstrap/` | DI 装配 |
| Services | `core/service/` | 会话、项目、Agent、技能、审批、MCP、配置、**ChannelIngress** |
| Runtime | `core/runtime/` | Agent Loop、Tool 执行、权限、压缩、输出硬上限 |
| Domain | `core/domain/` | 实体与值对象 |
| Ports | `core/port/` | Engine、LLM、Repository、Stream、**Channel*** |
| Adapters | `core/adapter/` | LLM、IM 通道（飞书 / QQ / 微信 / 企微）、配置加载 |
| Store | `core/store/` | SQLite 元数据、Turn Log、Checkpoint、channel_bindings |

---

## 6. 引擎运行时

### 6.1 请求流

```
HTTP / CLI / TUI
    → core/service/*Manager
    → port.Engine (runtime.Engine)
         → runTurn → TurnRunner (Step 循环)
              → LLM + permission.Gate + tool.Registry
              → builtin / MCP / delegate_agent / ask_user
    → SQLite (元数据) + 文件系统 Turn Log + Compaction Checkpoint
    → StreamEvent → SSE / UI
```

### 6.2 SessionRunner（`core/runtime/session_runner.go`）

```
StartSession / StartTurn
  → 异步 runTurn:
      1. 创建 Turn（SQLite + JSONL start）
      2. 组装 messages（system prompt + KB hits + 历史 + user goal）
      3. TurnRunner.Run
      4. 追加消息到 session 上下文
      5. afterTurn（可能触发 session 级压缩）

ResumeTurn  — 从 Turn Log 回放 tool_call / tool_result 后继续
RecoverRunning — 未完成 tool 收口、僵尸 Turn 标失败、过期审批清理、卡住 Session 修复
```

### 6.3 TurnRunner（`core/runtime/turn_runner.go`）

```
Step 循环 1..MaxSteps (Agent.Steps 或默认 20):
  1. 可选 Turn 内压缩（tool result 去重/截断/配对）
  2. LLM Chat（含 function calling）
  3. 无 tool call → 最终文本报告，结束
  4. 有 tool_calls[] → 按 LLM 并行 tool-call 规范执行（见 6.3.0）
  5. 立即 hard-cap Tool Result（见 6.3.1）→ 注入 messages / Stream / Turn Log
  末步强制 toolChoice=none + 最大步数提醒
```

Doom-loop 检测、权限门禁、审批阻塞、`ask_user` 阻塞均在此层完成。

#### 6.3.0 LLM 并行 Tool Call 规范

模型在同一条 assistant 消息里返回多个 `tool_calls` 时，视为**显式并行意图**。人机交互前置，避免和副作用 tool 赛跑：

```
1. 串行门禁：doom / unknown / permission / approval（审批全部问完）
2. 串行 start + Execute：ask_user（pending→running 后执行并答完）
3. 串行 start：其余 tools 全部 pending→running
4. 并行 Execute：其余 tools 的 handler.Execute
5. 串行提交：completed|error + messages / Turn Log（按 call 顺序）
```

| 规则 | 行为 |
|------|------|
| 并行范围 | **只有**非交互 `Execute`；门禁、审批、start 状态、`ask_user`、结果提交均串行 |
| 审批 | 前置：同批所有需审批的 call 先 `WaitApproval`，再进入 Execute |
| `ask_user` | 前置：start 后串行 Execute，再给其余 tool 发 start / 并行 |
| tool start | 前置串行：`pending` → `running` 在 Execute 之前发完 |
| tool 结果 | 后置串行：全部 Execute 结束后按 call 顺序 `completed`/`error` + Turn Log |
| Doom | 按 call 顺序累计；命中后该 call 及后续不再 Execute |
| 取消 | ctx 取消后为未完成 call 补 `cancelled` |
| `delegate_agent` | 独立 child `TurnRunner`，不改写父 Registry |

#### 6.3.1 本地 Tool 输出硬上限

`exec_shell` / MCP 等可能一次吐出超大文本；若等到 Session/Turn 压缩才截断，首轮就可能撑爆 context。

| 项 | 行为 |
|----|------|
| 配置 | `runtime.tools.max_output_chars`（默认 **50000**） |
| 时机 | `Execute` 之后、进入 LLM context / UI stream / Turn Log **之前** |
| UI | Settings → Runtime → Local Tools 滑条（5k–200k） |
| 与压缩关系 | 硬上限是**首道闸**；compaction 的渐进截断仍作用于后续 steps |

#### 6.3.2 Tool-call 参数 JSON 修复

OpenAI-compat 路径上，模型常把 `write`/`edit` 等内容里的未转义引号、裸换行写进 `arguments`，严格 `encoding/json` 会整次 Chat 失败。`core/adapter/llm` 在 `parseArgs` 严格解析失败后走 best-effort `repairJSONObject`（转义字符串内裸引号/控制字符、去尾逗号、补全截断括号），再失败才报错。

### 6.4 核心服务（`core/service/`）

| Manager | 职责 |
|---------|------|
| `SessionManager` | Session CRUD，触发 Engine 启动/续聊 |
| `TurnManager` | Turn 元数据（SQLite） |
| `TurnLogManager` | 文件系统 Turn Log |
| `ProjectManager` | Project + 数据目录解析；Stage 文件 `PUT .../files/content` |
| `AgentManager` | Agent CRUD + 内置模板 |
| `SkillManager` | Skill 与 Skill 文件 |
| `ApprovalManager` | 审批记录 |
| `LLMConfigManager` | 模型配置 |
| `MCPManager` | MCP Server 配置 |
| `KnowledgeManager` | 知识库（MD SoT + 章级 FTS） |
| `ConfigManager` | YAML 配置 |
| `ChannelIngress` | IM 入站 → Session Turn → 经 `ChannelEndpoint` 出站；`HandleInteraction` |
| `*Bridge` / `*Endpoint` | 各平台连接与差异化投递（飞书 / QQ / 微信 / 企微） |

装配入口：`core/bootstrap/bootstrap.go` → `bootstrap.Core`。

---

## 7. 工具系统

**一切都是工具，模型驱动一切。** 文件系统、网络、知识库、代码执行、委派 Agent、向人类提问——统一为 tool 接口。

### 7.1 内置 Tool 目录

全局注册（`bootstrap` → `Engine.RegisterTool`）：

| Tool | 用途 |
|------|------|
| `exec_shell` | Shell 执行 |
| `read_file` / `write` / `edit` / `apply_patch` | 文件读写与补丁 |
| `read_image` | 读取本地图片交给视觉模型（多模态 `ToolResult.Parts`） |
| `grep` / `glob` | 代码搜索 |
| `todowrite` | 任务清单 |
| `computer` | 桌面 GUI 控制（找/聚焦窗口、鼠标、键盘、截屏）；高风险、仅本机、每次审批 |
| `memory_update` / `memory_read` | 跨会话持久记忆（user / project / agent 三级作用域） |
| `table_upsert` / `table_get` / `table_query` / `table_delete` / `table_list` | 业务流水表（schema-free JSON；独立 `store.db`） |
| `web_fetch` / `web_search` | 读网页 / 搜索 |
| `browser_navigate` / `browser_snapshot` / `browser_act` / `browser_screenshot` / `browser_close` | 语义浏览器操控（Session 级 sticky Tab；默认仅内置 `browser` 专家绑定） |
| `http_request` | 通用 HTTP/REST（文本/JSON；非二进制） |
| `ask_user` | 向用户提问（全局，无需 Agent 绑定） |
| `sleep` | 等待 |
| `read_skill` | 读取技能说明（**全局默认**，无需 Agent 绑定） |

#### 7.1.1 桌面控制（`computer`）

单一工具、`action` 枚举（Anthropic Computer Use 对齐）：`list_windows` / `focus_window` /
`screenshot` / `mouse_move` / `left_click` / `right_click` / `middle_click` /
`double_click` / `left_click_drag` / `scroll` / `type` / `key` / `cursor_position` /
`wait`。截屏经 `ToolResult.Parts`（`image/png`）走多模态链路，与 `read_image` 相同。

- **坐标系**：整屏截图为屏幕绝对坐标；窗口截图为图像相对坐标，后续动作带同一 `window_id` 时由
  后端按窗口 bounds 自动换算为绝对坐标。
- **仅本机、无沙箱**：与容器隔离不兼容，权限门以 `desktop_control` 原因**始终 ask、永不
  auto-approve**（等同 `unsandboxed` 类别）；`discuss/plan` 模式直接拒绝。
- **平台后端**（`core/runtime/computer/`，无新增 CGO 依赖）：Linux X11 用 `xdotool` + 截图
  CLI（imagemagick/scrot/ffmpeg/xwd）；macOS 用 osascript/`screencapture`（鼠标需 `cliclick`，
  且需授予 Accessibility 与 Screen Recording TCC）；Windows 用 PowerShell + user32/System.Drawing。
  无可用后端时 `ComputerStatus` 报 degraded，动作返回结构化错误。
- 默认 `runtime.computer.enabled=false`，需显式开启。

### 7.1.0 能力三层（Core / Bound / Ambient）

**产品名词**：UI 统一称 **连接器**（Connectors）；MCP 仅为协议/实现细节（tool 名仍可 `mcp_*`）。连接器 = 已安装实例（鉴权、开关、动作列表）；连接器目录 = 一键预设。

Agent 可用能力按三层合成；Skill 与连接器共用同一 Ambient 开关。

| 层 | 内容 | 何时生效 |
|----|------|----------|
| **Core** | `ask_user`、`memory_*`、`table_*`、`read_skill`、`search_kb`（有 KB 时）；`delegate_agent`（`canDelegate`） | 始终（与绑定无关） |
| **Bound** | Agent `skillIds`（DB 技能）+ `tools[]`（builtin）+ `mcpServers[]`（连接器 id） | 始终按 Agent 配置 |
| **Ambient** | 磁盘技能目录 + **已启用且 `ambientMount!=false` 的连接器** | 仅当 `inheritAmbient`（默认：`primary=true`，`subagent=false`） |

`inheritAmbient` 可在 Agent JSON / YAML（`inherit_ambient`）/ Teams UI 覆盖；`null` 表示按 Mode 默认。

#### 自定义技能目录（Ambient，New Turn 扫描）

每个 New Turn 在构建 system prompt 前实时扫描磁盘技能目录（**不写 SQLite**），与 Agent 已绑定的 DB 技能内存合并后注入 `<available_skills>`——**仅 Ambient 开启时**扫描磁盘。

| 路径 | 范围 |
|------|------|
| `~/.agents/skills/` | 用户 |
| `~/.danmo-work/skills/` | 用户 |
| `<projectRoot>/.agents/skills/` | 项目（`WorkDir`） |
| `<projectRoot>/.danmo-work/skills/` | 项目（`WorkDir`） |

同名 ID 覆盖顺序（后者赢）：绑定 DB → `~/.agents` → `~/.danmo-work` → 项目 `.agents` → 项目 `.danmo-work`。目录缺失或坏 `SKILL.md` 跳过。Skills 管理页仍只显示 DB（内置 / 手建 / 市场）。

市场源（`market.sources`）除官方 git catalog（dq-market）外，可启用 `kind: techleads`（Tech Leads Club 精选目录，默认开启）与 `kind: clawhub`（ClawHub 公共注册表，默认关闭）。安装写入 SQLite 技能库，与 Ambient 磁盘扫描无关。

#### 连接器绑定粒度：**按连接器 id（独立字段）**

- Agent 侧：`mcpServers: ["github", "notion"]`（YAML `mcp_servers`，字段名保留技术 id）；**不支持通配符**。
- `tools[]` 只绑 builtin（`tool_id`）；旧版 `tools[].mcp_server` 读入时迁移到 `mcpServers`。
- 单个动作的启用/禁用在连接器配置里（discover 后 persist），不绑到 Agent。
- Ambient 开：`MountAllMCP`（跳过 `ambientMount=false` 的连接器）；Ambient 关：仅 `MountServers(agent.MCPServers)`。
- 连接器预设可声明 `ambientMount: false`（bound-only）与 `toolTimeout`；安装时写入实例，无按连接器 id 硬编码。
- 产品内置连接器（如 `danmo-make`、`github`）由 bootstrap `ensureBuiltinConnectors` 以固定 server id 自动 seed；专家/技能/知识库走 embedded 资源 home `core/resource/home`（`agents/` + `skills/` + `knowledge/`，UI「内置」= 存在 template）：启动时 `SyncBuiltinToFS` 按 manifest hash 同步到 `~/.danmo-work/{agents,skills,knowledge}`，专家/技能与用户资源同路径扫描，知识库目录与插件 KB 同构（`_meta.json` + `*.md`），经通用 `ScanBuiltinKnowledgeDir` 摄入（seed-if-missing，新文档即时建索引）。**CodeGraph** 已迁到市场：安装专家时经 `skillDeps` + `connectorDeps` 拉齐技能与连接器；连接器安装会执行包内 **平台 deps 脚本**（脚本自行下载/解压 CLI、或 apt 等）；可单独安装连接器。`first_launch` 脚本保留作通用初始化钩子（不再解压 CodeGraph）。首次 `delegate_agent`→`codegraph`（已安装时）异步 `codegraph init`，完成前专家降级 `read_file`/`grep`。`github` 专家包 = 技能 + **绑定型**官方远程 MCP（server id `github`，`AmbientMount=false`，市场不再提供 `github-mcp`）+ 降级链 **MCP → `gh` → `git`**：`GitHubMCPReady` 为真用 `mcp_github_*`，否则 `gh`（`ResolveGhBin`），再否则 `git`（`ResolveGitBin`，仅 remote/push/fetch；Issue/PR 仍报阻塞）；首次委派注入 `[github-access: mcp|gh|git|none]`。市场目录会过滤 `BuiltinConnectorIDs` 与遗留 `github-mcp`，并拒绝安装。

| Tool / 能力 | 条件 |
|------|------|
| `search_kb` | Core（绑定知识库时） |
| `memory_update` / `memory_read` | Core |
| `table_upsert` / `table_get` / `table_query` / `table_delete` / `table_list` | Core |
| `ask_user` / `read_skill` | Core |
| `delegate_agent` | Core + `canDelegate` |
| 其它 builtin | Bound：`tools[].toolId` |
| 连接器动作 | Ambient（`ambientMount` 允许时），或 Bound：`mcpServers[]` |
| 磁盘技能 | Ambient（`inheritAmbient`） |

### 7.1.1 外部 API 分层（避免 Tool 元数据膨胀）

不要为每个第三方 HTTP endpoint 注册独立 builtin Tool——schema 会膨胀、Agent 绑定变长、权限策略难以统一。

| 层级 | 用途 | 示例 |
|------|------|------|
| `http_request` | 临时/探索/任意 REST；一个稳定 schema 覆盖多数 API | `POST` JSON、带 `Authorization` 的 GET |
| `web_fetch` / `web_search` | 读页面与搜索，不做通用 REST 客户端 | 文档页、搜索结果 |
| `browser_*` | 多步网页交互（navigate / a11y snapshot refs / act / screenshot）；CDP 为 adapter（`cdp_url` 附着或自动 launch 无头） | SPA 表单、多页状态；由内置专家 `browser` 经 `delegate_agent` 使用 |
| MCP Tool | 产品级、需强 inputSchema / 复杂鉴权的集成 | 业务动作封装 |
| 领域 builtin | 极少；仅高频产品能力 | `web_search` 提供商 |
| Skill 文档 | API 用法与约定（按需 `read_skill`），不占常驻 tool schema | OpenAPI 说明 |

`http_request` 边界：UTF-8 文本 body；非文本响应只返回 Content-Type/字节数；不做 base64/multipart。写方法与敏感 header（如 `Authorization`）提升为 high risk 并走审批。优先于 `exec_shell curl`。

### 7.2 `ask_user`

```
Agent 调用 ask_user(question, options?, form_fields?)
  → 发布 ask_user.pending 事件
  → 阻塞等待 ResolveAskUser(askID, answer)
  → 用户输入作为 tool result 继续 Agent Loop
```

支持自由文本、选项、结构化表单字段。与权限审批分离：`ask_user` 不走 Risk 门禁。

### 7.3 `delegate_agent`

```
Lead Agent (CanDelegate=true)
  → delegate_agent(agent_id, goal, context?)
      → 查 AgentManager，校验循环委派 + maxDelegationDepth
      → RunSubTurn：子 Agent 独立 Turn + 独立 Tool Registry
      → 返回 Report（结构化，含 <session_result>）
      → 发布 delegate.started / delegate.completed
```

子 Agent 拥有自己的人设、技能、工具集和知识库。父 Agent 只看到委派结果，看不到子 Agent 私有能力细节。

### 7.4 持久记忆（`memory_update` / `memory_read`）

与 Knowledge（人工文档）、Compaction（会话内压缩）分离。记忆由模型在「值得记住」时显式写入，**不**每轮自动注入 system prompt（避免噪声与旧版 Episodic 近零召回问题）。

| 作用域 | `scope_id` | 用途 |
|--------|------------|------|
| `user` | `default` | 跨项目偏好、沟通习惯、禁忌 |
| `project` | 当前 `ProjectID` | 项目约定、架构决策、已验证事实 |
| `agent` | 当前 `Agent.ID` | 该 Agent 角色专属工作方式 |

```
memory_update(scope, key, content, mode?=set|append)
  → runtime 注入合法 scope_id（禁止伪造其他 project/agent）
  → SQLite memories UNIQUE(scope, scope_id, key) upsert

memory_read(scope?, key?, query?)
  → 默认可见：user + 当前 project + 当前 agent
  → 关键词匹配；条数上限 runtime.memory.read_top_k（默认 10）
```

**引导**：system prompt 固定块 `<memory-policy>` + tool Description + Agent 模板（何时记 / 勿记 / 作用域选择）。

**UI**：右侧工作区「记忆」Tab（`MemoryPanel`）按作用域分组展示；`GET/DELETE /api/v1/memories`；`memory_update` 完成后自动刷新。

**勿记**：一次性任务细节、大段代码、密钥、可从仓库再读的文件内容、短暂 todo（用 `todowrite`）。

### 7.4.1 Table Store（`table_*`）

与 Memory 分家：Table Store 存**可查询业务流水**（如每日邮件摘要），Memory 存偏好/约定。物理库为独立 `~/.danmo-work/store.db`（`WORK_STORE_DB_PATH`），避免灌库拖慢 `work.db` 控制面。

```
table_upsert(scope, table, key, data, mode?=replace|merge)
table_get / table_query / table_delete / table_list
  → runtime 注入 scope_id；硬配额见 runtime.table.*
```

### 7.5 Tool 统一接口（概念）

```
Tool:
  name, description, parameters (JSON Schema)
  permissions / risk metadata
  execute(params, context) → ToolResult
```

调度循环即 Agent Loop：LLM 规划 → Tool 执行 → 结果反馈 → LLM 再规划。

---

## 8. 权限与审批

摘要（Soft Gate × Hard Enforcement）：

- **Soft**：`permission.Gate` — discuss/plan 拒绝写/执行；强沙箱内安全 shell 放行；危险命令 / 弱沙箱 / external·MCP 询问。
- **Hard**：OS sandbox（FS）+ `network` 三态（deny / allowlist 正向代理 / allow）；主机 HTTP 与 shell 共用出站策略。
- **`auto_approve`**：跳过部分审批等待，但**不**自动放行 `dangerous_command` / `unsandboxed`。
- **`ask_user`** 是协作语义；Approval 是安全门禁。

---

## 9. 持久化与恢复

### 9.1 存储分工

| 存储 | 路径 / 位置 | 内容 |
|------|------------|------|
| SQLite | `~/.danmo-work/work.db` | agents, sessions, projects, turns, approvals, skills, knowledge, **memories**, mcp, llm_configs, stream_events |
| Turn Log | `{dataDir}/{project}/sessions/{sessionID}/{turnID}.jsonl` | start / tool_call / tool_result / end |
| Checkpoint | 同 Session 目录 `checkpoint_*.json` | 跨 Turn 压缩摘要 |
| 配置 | `~/.danmo-work/config.yaml` | 运行时配置（含 `runtime.memory.read_top_k`） |

### 9.2 设计原则

| 设计点 | 原则 |
|--------|------|
| Session 长生命周期 | 用户随时可发起下一 Turn；Session 跨天/周 |
| Turn 是执行单元 | 异常、取消、超时针对 Turn |
| 日志即状态 | Turn Log 可回放恢复；LLM Message 不直接持久化为真相源 |
| 启动恢复 | `RecoverRunning`：僵尸 Turn、过期审批、卡住 Session |

### 9.3 恢复

```
ResumeTurn:
  closeIncompleteToolPairs（崩溃残留 tool 按正常失败收口）
  → 从 JSONL 加载 tool_call / tool_result → 重建消息 → 继续 TurnRunner

RecoverRunning:
  过期 Approval → permission.decided(false)
  嵌套 tool_runs Turn → 收口未完成 tool → failed + turn.failed
  父 Turn → closeIncompleteToolPairs
      （JSONL 补 tool_result + stream tool.error；
       delegate_agent 同步 child failed + delegate.completed）
  → LoadForRecovery → ResumeTurn
  卡住 Session（无 running Turn）→ 状态修复
```

---

## 10. 上下文管理

### 10.1 Turn 内压缩（`TurnRunner.compactMessages`）

Tool 刚执行完时已按 `runtime.tools.max_output_chars` 做硬上限（§6.3.1）。压缩是后续 steps 的二次治理：

```
每 Step（step > 1，且 compaction.enabled）:
  1. 配对完整性: 过滤孤儿 tool_result（始终）
  2. 低于高水位（MaxTokens * TriggerRatio）: 不裁剪（含去重；保持 LLM 前缀稳定以利 KV cache）
  3. 超高水位时（Harness-style pressure compaction）:
     a. 去重: 同 tool+input → 保留最新，旧结果摘要
     b. 渐进截断: 超大 tool result（超过 toolTruncate，默认 8192）→ head + marker + tail；最近 keepRecentToolSteps 批（默认 3）豁免
     c. 仍超阈值 → 删除最旧 assistant+tool 对直到低水位；最近 keepRecentToolSteps 批豁免
```

Compaction 仅修改内存中的 LLM messages；tool 执行时的完整输出仍落在 durable turn log。
当上下文中的结果被 prune/snip 后，Agent 可调用 `recall_tool_result(call_id)` 从 log 取回（受 `max_output_chars` 执行时硬上限约束；不自动写回上下文）。

### 10.2 Session 级压缩（`CompactionManager`）

```
触发（afterTurn）:
  - token > MaxTokens * TriggerRatio
  - 或 Turn 数达到 TurnInterval / SubInterval

切点: 逆序累计 token ≥ CutTokens，禁止切在 tool_result 内部
摘要: LLM → CompactionCheckpoint 落盘
注入: 下一 Turn system prompt 带入 Checkpoint 摘要
事件: context.compacted
```

配置见 `domain.ConfigCompactionSection`（enabled、maxTokens、triggerRatio、lowWaterRatio、cutTokens、turnInterval、subInterval、toolTruncate）。Turn 内裁剪用 triggerRatio / lowWaterRatio；Session 压缩保留窗口用 cutTokens。

### 10.3 记忆 vs 压缩 vs 知识库

| 机制 | 写入方 | 生命周期 | 检索 |
|------|--------|----------|------|
| **Memory** | Agent tool（显式） | 跨 Session，SQLite | `memory_read` / UI |
| **Compaction Checkpoint** | 引擎自动摘要 | Session 内 | 注入 system prompt |
| **Turn log recall** | 工具执行时落盘 | Turn 内 durable | `recall_tool_result`（读 log，不自动注入上下文） |
| **Knowledge** | 人工文档（`~/.danmo-work/knowledge/` MD SoT） | 绑定 Agent KB | 章级 `search_kb` + 自动 top-K 注入（FTS5 BM25；可选向量混合） |

会话内被压缩丢掉的消息召回（BM25-at-compaction）为后续议题，与 durable Memory 独立。Turn 内 compaction 仅改内存 messages；完整 tool 输出仍在 turn log，可用 `recall_tool_result(call_id)` 取回（受 `max_output_chars` 执行时硬上限约束）。

### 10.4 KV Cache 友好分区（设计目标）

```
Zone A — Frozen:   [system] Agent Persona + 排序后的 skills/agents + 静态 policy + checkpoint/todos/file-changes（仅压缩时变）
Zone B — Append:   保留区历史（未压缩则跨 turn 前缀不变）
Zone C — Scratch:  当前 Turn user/tool + 调用末尾 ephemeral user（turn-context / plan-mode / kb）
```

---

## 11. 事件流

运行时通过 Stream Event 推送进度（SSE / UI Timeline）：

| 事件族 | 示例 |
|--------|------|
| Step | `step.started` / `step.ended` |
| Tool | tool 开始/结束、结果 |
| 权限 | `permission.ask` |
| 用户 | `ask_user.pending` |
| 委派 | `delegate.started` / `delegate.completed` |
| 压缩 | `context.compacted` |
| Session / Turn | 开始、完成、失败 |

历史事件可经 API 查询；Turn Log 提供完整可审计轨迹。IM 通道通过 `ProgressUpdater` / `StreamSender` 将同类进度映射为平台卡片或流式气泡（见 §12）。

---

## 12. IM 通道（ChannelEndpoint）

手机 IM 与桌面共用同一套 Agent Loop；差异收敛在 Endpoint，**Ingress 无平台业务分支**。契约见 `core/port/channel.go`。

### 12.1 编排

```
平台事件 / 长连接
  → Adapter 归一化为 InboundMessage | InteractionEvent
  → ChannelIngress.HandleInbound / HandleInteraction
       → 解析 peer 项目绑定（meta.project_id → channel 默认 project）
       → 启动/续聊 Session Turn（工具仍在本机）
       → StreamEvent → ProgressUpdater / StreamSender
       → 终态 OutboundMessage → ChannelEndpoint.Deliver
```

| 接口 | 职责 |
|------|------|
| `ChannelEndpoint` | `Capabilities` + `Deliver(OutboundMessage)` |
| `StreamSender` | 渐进流：Start → Update* → Finish |
| `ProgressUpdater` | 更富的 tool/进度卡片（优先于纯文本 Update） |
| `ChannelInteractor` | 通道内呈现 `ask_user`（卡片 / 键盘 / 编号菜单） |
| `ChannelApprover` | 通道内工具审批（once / session / deny） |
| `HandleInteraction` | 按钮/键盘回调 → ResolveAskUser / DecideApproval / `/project`；**不新建 Turn** |

会话键：`(channel, account, peer)`。多通道绑同一项目**不串会话**。入站媒体落盘到 `data/channels/...`。

### 12.2 能力分化（现状）

| 通道 | 连接 | 渐进流 | 审批 / ask | 备注 |
|------|------|--------|------------|------|
| **飞书** | 出站长连接 | 进度卡 PATCH（可挂审批按钮） | schema 2.0 交互卡片 / 表单 | `auto_approve` 默认 false |
| **QQ** | Gateway WS | `native_c2c_stream` | Keyboard | 群 `require_mention` / `deny_tools` |
| **企微** | 出站 WS | 原生 `msgtype: stream` | 流内文字菜单 | ~5s 占位规则 |
| **微信** | iLink 长轮询 | 「正在处理…」→ 终态（无中途编辑） | 回复 `1/2/3` 文字菜单 | peer `/project` 覆盖账号默认项目 |

出站统一为 `OutboundMessage`（text / markdown / card）；Endpoint 按 `Capabilities` 降级（如微信 card → 编号文本）。

### 12.3 非目标（通道层）

独立原生 App / 小程序、非官方 QQ 协议、用 IM 复刻完整桌面 Trace/Memory/Settings、完整二进制出站上传、微信原生卡片（iLink 无）。

---

## 13. Document Stage（统一画布）

中间画布为唯一内容视图 **Document Stage**：按文件类型切换 toolbar + surface（文档 / 幻灯片 / 表格 / 源码 / 差异 / 预览）。AI 改稿（doc/slides/sheet）走**普通 session turn**，不另开 `/office/ai`。右侧不再有独立 Browser tab。

Agent 自动化浏览器（Settings / `web_fetch` 渲染 + 内置 `browser` 专家的 sticky `browser_*`；CDP attach 或本地 launch）与此 UI 无关，保持独立。

### 13.1 路由与格式

Files 树点击 → `routeOfficeFile`（`frontend/src/utils/office-route.ts`）：

| Kind | engine | 真相源（SoT） | Surface | 默认模式 |
|------|--------|---------------|---------|----------|
| `doc` | `md` | GFM `.md` | TipTap | view（可切 edit） |
| `doc` | `univer-doc` | `.udoc.json`（`IDocumentData` 信封） | Univer Docs | edit |
| `doc` | `ms-office` | `.docx` | 只读预览；转 `.udoc.json` 后可编 | **view** |
| `slides` | `univer-slides` | `.uslides.json`（`ISlideData` 信封） | Univer Slides Stage | edit |
| `slides` | `ms-office` | `.pptx` | 只读预览；转 `.uslides.json` 后可编 | **view** |
| `sheet` | `csv` | `.csv` | 自研网格 | edit |
| `sheet` | `univer-sheet` | `.usheet.json`（`IWorkbookData` 信封） | Univer Sheets | edit |
| `sheet` | `ms-office` | `.xlsx` | 只读预览；转 `.usheet.json` 后可编 | **view** |
| `code` | `code` | 常见源码 / 配置文本 | CodeMirror 6 | view |
| `diff` | `diff` | `git diff` / AI snapshot | DiffSurface | view |
| `preview` | `preview` | `.html`、图片、外链等 | iframe / 图片预览 | view |

**已移除：** Marp / `*-slides.md` 幻灯片 SoT；`.danmo-sheet.json`（打开时一次性迁到 `.usheet.json`）。

布局：`stage` 时 Stream | Stage | 右侧；`immersive`（Present / 沉浸）时仅 Stage。默认打开文件不隐藏 Stream。

Changes 面板点击变更文件 → 打开 `diff` Stage（`GET /projects/:id/git-diff?path=&staged=`）。

### 13.2 AI 编辑回合

```
Stage 工具栏（润色 / 修改 / …）
  → 若 dirty：先 auto-save（失败则确认）——避免 Agent 读到旧盘、reload 冲掉未保存编辑
  → buildOfficeEditPrompt → [office-edit] 块（action / path / kind / engine / scope / selection / review）
  → POST /sessions/:id/turns（可选 snapshotPaths）
  → **pre-turn snapshot**（office-edit path + Stage 路径）
  → Document Agent + document-writing / playable-slides / sheet-writing skills
  → Turn 完成后 Stage reload；恢复 scroll 与 slides pageIndex
  → 若文件相对快照有变更 → **AI 审阅条**（View Diff / Keep / Revert）
```

| Scope | 含义 |
|-------|------|
| `selection` | 当前选区 |
| `document` | 全文 |
| `slide` | 当前幻灯片页 |
| `sheet` | 整表或表格选区（`range: A1:B3`） |

`engine: ms-office` 禁止写回 OOXML；须先「转为 Univer IR」。

保存 API：`PUT /api/v1/projects/:id/files/content`。

**人机审阅（AI Diff）**：回合前快照存于 `sessions/<sid>/snapshots/<turnId>/`；`GET .../ai-review/diff` 对比快照与当前盘；Revert 写回快照；Diff Stage 支持按 hunk 接受。

源码改码：不挂 Office AI 工具栏；用户通过 Composer 选区批注（`## Selected Code` + File/Lines）或对话驱动 Agent `edit`/`apply_patch`；打开 Stage 的 Composer 回合同样会 snapshot 该 path。

### 13.3 非目标（本阶段）

不以 OOXML 为可编辑 SoT（仅只读 + 转 IR）；Univer Pro 依赖；`.univer` SQLite 多 Unit 容器；Yjs 协同；LSP / IDE 壳；完整 Git 客户端（commit/push/conflict UI）；继续维护 Marp / `.danmo-sheet.json`。

---

## 14. 产品形态与价值

工作台：侧栏 · Stream · 右侧 Tab（计划 / 文件 / **记忆** / 变更 / 终端）· 中心 **Document Stage**（按 kind 换 toolbar）。记忆 Tab 展示当前可见作用域下的持久记忆条目。IM 通道是同一 Agent Loop 的远端输入面。

| 层级 | 价值 |
|------|------|
| 基础 | Agent 自动执行任务 |
| 进阶 | 可视化审计，可解释 |
| 核心 | 人机实时协作，共同决策（桌面 Stage + IM 审批 / ask） |
| 壁垒 | 工作流即知识，可沉淀、复用、分享；Agent 记忆跨会话延续 |

**架构优势**：

1. **极简**：一种抽象（Tool），一种循环（Agent Loop），一种存储（日志）
2. **透明**：每一步可观察、可干预、可延续
3. **弹性**：异常可恢复，错误可纠正，状态不丢失
4. **动态**：模型自主规划，自适应任务复杂度
5. **沉淀**：成功轨迹可复用为模板与自动化
6. **多入口**：Web / 桌面 / CLI / TUI / IM 共用同一引擎

---

## 15. 参考项目设计借鉴

| 设计点 | 参考来源 | 借鉴 |
|--------|---------|------|
| Task / Turn 追加写 | pi (JSONL) + DeepCode (append-only) | Turn Log JSONL |
| 委派 = 一条 tool_call | oh-my-openagent | `delegate_agent` + 子 Turn Report |
| 三区 KV cache | CodeWhale | Zone A frozen + Zone B append |
| 切点算法 | pi (findCutPoint) | 逆向累计，禁止切 tool_result |
| 压缩摘要 | opencode + pi | Checkpoint JSON + 增量合并 |
| Tool result 治理 | DeepCode + CodeWhale | 硬上限 + 去重 / 截断 / 配对 / 头尾截断 |
| 分层架构 | Ports & Adapters | service → port ← runtime / store |
| 显式 Memory Tool | Cursor / Claude Memory | `memory_update` / `memory_read` + 三级作用域 |
| IM 出站长连接 | 飞书 / 企微 / QQ Bot | ChannelEndpoint + Capabilities 降级 |
| 文档 SoT = Markdown | 常见 MD-first 编辑器 | Document Stage；AI 走 session turn |

---

## 16. 总结

> **在「一切皆工具，模型驱动一切，日志原生持久化」的架构下，Code 和 Work 是伪区分。**
>
> 同一套架构，同一套 Tool，不同参数配置，自然呈现不同行为特征。
>
> 系统不需要显式模式，只需要：统一的 Tool 抽象、参数化的调度策略、完全透明的执行轨迹。

| | 主流框架 | Danmo Work |
|--|---------|---------------|
| **哲学** | LLM 当作组件，开发者编排 | LLM 是唯一决策中心 |
| **抽象** | Agent / Chain / Graph / Role 多层 | 单一 Tool 抽象 |
| **人机关系** | 人下指令，机器执行 | 人进入思维流，共同迭代 |
| **调试** | 外部干预，改代码重跑 | 原生能力，改数据 / 审批继续 |
| **信任** | 预设规则限制 | 完全透明，随时纠正 |
| **入口** | 多为单一 Chat / IDE | Web · 桌面 · CLI · TUI · IM 通道 · Document Stage |

---

*架构版本：v2.2（Session / Turn / Tool + Memory + ChannelEndpoint + Document Stage + tool-output hard cap）*

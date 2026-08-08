# 专家使用说明 / Expert usage

Danmo Work 用**主专家（lead / primary）**跑会话，用**子专家（subagent）**做专项工作。子专家**不会**常驻主链：主专家通过 Core 工具 `delegate_agent` 按需委派，专家上下文单独开，主链前缀更稳，也更利于 KV Cache。

更多架构说明见 [core-design.md](./core-design.md)。

---

## 1. 主专家 vs 子专家

| 类型 | `mode` | 典型用途 |
|------|--------|----------|
| 主专家 | `primary` | Composer 里选择的会话主角：`team`（可协作）、`default`（单兵）、`planner`（只读规划） |
| 子专家 | `subagent` | 被 `delegate_agent` 召唤：文档、代码实现、GitHub、调研等 |

只有主专家打开 **`canDelegate`（启用专家协作）** 时，运行时才会挂载 `delegate_agent`，并在 system prompt 注入 `<available_agents>`。内置 **Team** 默认开启；**Default / Planner** 默认关闭。可在 **Teams → 专家** 中开关。

---

## 2. 如何在 Composer 召唤专家

召唤的本质：在用户消息前注入委派提示词，引导主专家调用：

```text
delegate_agent(agent_id="<id>", goal="...")
```

### 前置条件

1. 会话主专家已启用专家协作（推荐选 **Team**）。
2. 目标专家是 `subagent`（Teams 库中的协作专家）。

### 入口

| 入口 | 行为 |
|------|------|
| 工具栏**专家**图标（人物） | 打开可搜索的多选列表；选中后出现「专家」chip |
| 输入 `@` | 统一浮层：上区**技能**、下区**专家**；选中后去掉 `@query`，加入 chip |
| 发送 | 前缀顺序：**专家委派提示 → 技能提示 → 用户正文** |

无协作权限时，专家按钮会提示切换到 Team，或在 Teams 为主专家开启协作；此时不会注入无效的 `delegate_agent` 前缀。

### 与技能 `@` 的区别

- **技能**：前缀要求 `read_skill`，主专家自己按技能 SOP 执行。
- **专家**：前缀要求 `delegate_agent`，把专项工作交给子专家独立上下文。

也可不点 UI：直接在对话里写「请委派 document 专家写报告」——有协作能力的主专家同样会调 `delegate_agent`。

---

## 3. 内置专家清单

### 主专家

| id | 名称 | 说明 |
|----|------|------|
| `team` | Team | 默认可协作；适合跨文件、多步骤任务 |
| `default` | Default | 单兵全工具；默认不委派 |
| `planner` | Planner | 只读规划；不写文件/不 shell |

### 子专家（可召唤）

| id | 名称 | 职责 | 典型技能 / MCP |
|----|------|------|----------------|
| `document` | Document | 报告、幻灯、表格等职场文档 | `document-writing`, `playable-slides`, `sheet-writing` |
| `comms` | Comms | 邮件、消息、通知润色 | — |
| `implementer` | Implementer | 按规格改代码 | TDD / debugging |
| `explorer` | Explorer | 只读摸代码库 | debugging |
| `researcher` | Researcher | 检索与调研 | `deep-research` |
| `reviewer` | Reviewer | 代码/产物审查 | `requesting-code-review` |
| `data` | Data | CSV/JSON 分析与报表 | shell 等 |
| `github` | GitHub | Issue / PR / Actions 等 | skill `github` + MCP `github` |
| `danmo-make` | Danmo Make | 本机图片/视频/音频生成 | skill + MCP `danmo-make`（需单独安装 Make） |
| `novel` | 小说创作 | 长篇/网文：章合同→草稿→审稿→Commit（内置子专家，Team 召唤） | `novel-writing`, craft KB `kb-novel-craft` |

市场安装的专家（如 **CodeGraph**）同样会出现在可召唤列表中（`mode=subagent` 且带 `marketSource`）。

---

## 4. 小说创作专家（简介）

当环境中存在 `novel` 专家时：

1. 主专家选 **Team**（或开启协作）。
2. Composer `@` / 专家图标选中「小说创作」，描述本章目标后发送。
3. 专家按 `novel-writing` 技能走：立项 → 大纲 → 章合同 → 正文 → 审稿门禁 → table/memory/文件 Commit。
4. 技法检索走知识库 `kb-novel-craft`（节奏、爽点、去 AI 味等）；本书设定用项目文件 + `table_*`。

详细 SOP 见技能 `novel-writing` 的 `references/`（应用内 `read_skill`）。

---

## 5. 市场与自定义专家

- **市场（Market）**：安装 `kind: expert` 包会写入专家定义，并可拉取 `skillDeps` / 连接器。
- **自定义**：在 Teams 新建子专家，绑定技能、工具、知识库与 MCP；主专家开启协作后即可被 `@` / 图标召唤。
- **Ambient**：子专家默认不继承全量 Ambient MCP；需要的连接器应写在专家的 `mcpServers` 绑定里（如 GitHub、Danmo Make）。

---

## 6. 界面与工具展示

会话时间线里，`delegate_agent` 工具卡片会显示为「召唤专家 / Summon expert」。完成与否以工具结果与子专家产出文件为准，而不是主专家的口头声明。

---

## 7. 彩蛋：会话「工作台」（小说书架）

会话顶栏右侧有 **工作台** 图标（也可 Composer 输入 `/novel`）。打开后与 Office 相同布局：

- **左**：事件流  
- **右**：工作台宿主（当前仅「小说」；以后可扩展其它工作台）  
- **底**：Composer 通栏  

小说工作台自包含，按技能流水线分组：

`立项 → 大纲/资产/金手指 → 章合同 → 写/续写 →（读章）审稿/润色 → Commit`

并内嵌阅读 bible / state / canon / outline / continuity / reviews / 章节。动作只 Prefill Composer（可勾选 `novel` chip），**不跳转** Files / Doc Stage。书仍落在 `novel/<book-id>/`（标准英文目录：`canon/`、`outline/`、`chapters/`、`continuity/`、`reviews/`；章合同=`chapters/chNNN-contract.yaml`）。

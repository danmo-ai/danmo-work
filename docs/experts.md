# 专家使用说明 / Expert usage

Danmo Work 用**主专家（lead / primary）**跑会话，用**子专家（subagent）**做专项工作。子专家**不会**常驻主链：主专家通过 Core 工具 `delegate_agent` 按需委派，专家上下文单独开，主链前缀更稳，也更利于 KV Cache。

更多架构说明见 [core-design.md](./core-design.md)。

---

## 1. 主专家 vs 子专家

| 类型 | `mode` | 典型用途 |
|------|--------|----------|
| 主专家 | `primary` | Composer 里选择的会话主角：内置 `team`（可协作）；自定义 primary 专家会一并出现在选择器中 |
| 子专家 | `subagent` | 被 `delegate_agent` 召唤：文档、代码实现、GitHub、调研等 |

只有主专家打开 **`canDelegate`（启用专家协作）** 时，运行时才会挂载 `delegate_agent`，并在 system prompt 注入 `<available_agents>`。内置 **Team** 默认开启；可在 **Teams → 专家** 中开关。Composer 的 **Plan 模式**开关会进一步把当前回合限制为只读工具，并注入计划模式提示词。

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

**专家 chip = 明确委派（透传模式）**：选中专家 chip 后发送，Composer 会注入 **relay** 前缀，要求 Team 将 `delegate_agent.goal` **原文转述**用户任务正文，禁止 Team 扩写、拆步或改写意图。小说工作台预填 + novel chip 同样走此模式。

若希望 Team **自行协调拆任务**（例如只说「帮我写小说」、不选专家 chip），则不要勾选专家 chip，让 Team 按 `<delegation-policy>` 正常分工。

无协作权限时，专家按钮会提示切换到 Team，或在 Teams 为主专家开启协作；此时不会注入无效的 `delegate_agent` 前缀。

### 与技能 `@` 的区别

- **技能**：前缀要求 `read_skill`，主专家自己按技能 SOP 执行。
- **专家**：前缀要求 `delegate_agent`，把专项工作交给子专家独立上下文。

也可不点 UI：直接在对话里写「请委派 document 专家写报告」——有协作能力的主专家同样会调 `delegate_agent`。

---

## 3. 内置专家清单

### 主专家（home embed）

| id | 名称 | 说明 |
|----|------|------|
| `team` | Team | 默认可协作；适合跨文件、多步骤任务 |

`team` 仍在 `core/resource/home` 随 `SyncBuiltinToFS` 同步。共享技能（debugging、document-writing、TDD…）也留在 home，供多个专家按 id 绑定。

### 子专家（全部为内置插件）

同步到 `~/.danmo-work/plugins/<name>/`，不可卸载。

| 分组 | id | 名称 | 同包资源 |
|------|-----|------|----------|
| 编码 | `implementer` | Implementer | 专家（技能用 home：TDD / debugging） |
| 编码 | `explorer` | Explorer | 专家 |
| 编码 | `reviewer` | Reviewer | 专家 |
| 编码 | `github` | GitHub | 专家 + skill + bound MCP |
| 调研/自动化 | `researcher` | Researcher | 专家（技能用 home：deep-research） |
| 调研/自动化 | `browser` | Browser | 专家 + skill + `browser_*` |
| 调研/自动化 | `computer` | Computer | 专家 + `computer-use` + `computer` |
| 职场写作 | `document` | Document | 职场**写作交付**（报告默认 `.md`；幻灯片/表格走绑定技能；含原 Comms）。专家提示词只做路由与禁令；写法/IR 细节在 home 技能与 `kb-office-ir` |
| 职场写作 | `data` | Data | 专家 |
| 创作 | `novel` | Novel Writing | 专家 + `novel-setup` / `novel-plan` / `novel-write` / `novel-review` + KB `kb-novel-craft` |
| 创作 | `danmo-make` | Danmo Make | 专家 + skill + bound MCP |

**Comms 已并入 Document**：职场沟通写作不再单独召唤 `comms`。

市场安装的专家（如 **CodeGraph**）同样会出现在可召唤列表中。

---

## 4. Novel Writing 专家（简介）

当环境中存在 `novel` 专家时：

1. 主专家选 **Team**（或开启协作）。
2. Composer `@` / 专家图标选中「Novel Writing」，描述本章目标后发送。
3. 专家按阶段技能走：`novel-setup` 立项 → `novel-plan` 设定/总纲/卷纲 → `novel-write` 章纲/正文 → `novel-review` 审稿/润色/Commit。  
   Continuity 账本为 `continuity/ledger.md`；换阶段时用户自行切换 Composer 模型。
4. 技法检索走知识库 `kb-novel-craft`（节奏与结构、爽点与追读、文风与去 AI 味等）；本书设定用项目文件 + `table_*`。

详细 SOP 见各技能的 `references/`（应用内 `read_skill`）。

---

## 5. 市场、内置插件与自定义专家

- **内置插件（Builtin plugins）**：所有内置**子专家**均以 Agent Plugins 布局落在 `~/.danmo-work/plugins/<name>/`。能力包可同带 `skills/`、`mcp.json`、knowledge；薄专家只带 `ai.danmo.work/experts/`，共享技能仍从 home 解析。
- **孤儿技能 / 连接器**：跨专家复用的 SOP（TDD、debugging、document-writing…）与产品连接器目录仍走原 home / catalog 模式，不强制塞进某个专家插件。
- **市场（Market）**：安装 `kind: plugin`（推荐）或 `skill` / `connector`。历史上的 `kind: expert` / `bundle` 已由插件取代。
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

小说工作台是 **流程控制台**（不是纯文件浏览器）：

1. **流程轨（8 步）**：立项 → 设定 → 总纲 → 卷纲 → 章纲 → 正文 → 审稿 → 定稿  
2. **门禁点**：`knowledge` / `asset` / `qc`（读 `novel-state.yaml` 的 `gates` + 磁盘启发式）  
3. **主 CTA**：引擎计算下一合法动作，注入 Composer + 选中 `novel`  
4. **章状态机**：无章纲 → 章纲草案 → 待写 → 待审 → 审未过/待提交 → 已提交  
5. **多模型**：换阶段时用户在 Composer **自行**切换模型；工作台不自动换模  

技能流水线：`novel-setup` → `novel-plan` → `novel-write` → `novel-review`。批次冻结只写 `novel-state.frozen_batch`。写正文消费 gate `### CONTEXT`；PASS 审稿不落盘。

动作 Prefill Composer（可勾选 `novel` chip）：**技能 + 意图/流程 + 书/章路径**，末尾附带 **工作台约束块**；不硬编码 skill reference 路径（由技能 Intent→Load 表决定）。**不跳转** Files。书落在 `novel/<book-id>/`（`canon/`、`outline/`、`chapters/`、`continuity/ledger.md`、`reviews/`；章纲=`chapters/chNNN-outline.yaml`）。

| 门禁 | UI 推断 | Agent 真执行 |
|------|---------|--------------|
| asset | `canon/cast/` 有文件 | 读上场人物卡（table 镜像可选） |
| qc | review `### VERDICT` | `review-gates.md`（PASS 短 stub） |
| batch | `novel-state` `artifacts.batch_freeze=frozen` | `novel-write/references/batch-freeze.md` |

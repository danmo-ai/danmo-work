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
2. Composer `@` / 专家图标选中「Novel Writing」，描述本单元目标后发送。
3. 专家按五阶段走：`novel-setup` 立项（gate `init` 建树）→ `novel-plan` 规划一轮（人物卡 + 总纲 + 卷纲；人批准卷纲后 gate `accept-volume` 提升人物、种细纲头）→ `novel-write` 一批细纲（≤4 个，`lint-units`）/ 写单元（`preflight` CONTEXT → **一份正文**）→ `novel-review` 定稿一轮（`qc-pack` → 审 → 一次 Commit → `postcommit`）。写作与定稿分 turn，便于写作用更好模型。  
   账本：`continuity/facts.md`（事实 / 游标）+ `continuity/summaries/vNN.md`（章摘要）+ `continuity/commits/`；换阶段时用户自行切换 Composer 模型。
4. 技法检索走知识库 `kb-novel-craft`（共性篇 + 题材专有篇；`novel-state.genre` 决定 preflight 注入哪一篇题材文，`craft_lane=crime-human` 追加「刑侦人味文风」）；本书设定用项目文件。

详细 SOP 见各技能的 `references/`（应用内 `read_skill`）。

---

## 5. 市场、内置插件与自定义专家

- **内置插件（Builtin plugins）**：所有内置**子专家**均以 Agent Plugins 布局落在 `~/.danmo-work/plugins/<name>/`。能力包可同带 `skills/`、`mcp.json`、knowledge；薄专家只带 `ai.danmo.work/experts/`，共享技能仍从 home 解析。
- **孤儿技能 / 连接器**：跨专家复用的 SOP（TDD、debugging、document-writing…）与产品连接器目录仍走原 home / catalog 模式，不强制塞进某个专家插件。
- **市场（Market）**：安装 `kind: plugin`（推荐）或 `skill` / `connector`。历史上的 `kind: expert` / `bundle` 已由插件取代。
- **自定义**：在 Teams 新建子专家，绑定技能、工具、知识库与 MCP；主专家开启协作后即可被 `@` / 图标召唤。
- **Ambient**：子专家不继承全量 Ambient（磁盘技能 / ambient MCP）；需要的连接器应写在专家的 `mcpServers` 绑定里（如 GitHub、Danmo Make）。

---

## 6. 界面与工具展示

会话时间线里，`delegate_agent` 工具卡片会显示为「召唤专家 / Summon expert」。完成与否以工具结果与子专家产出文件为准，而不是主专家的口头声明。

---

## 7. 彩蛋：会话「工作台」（小说书架）

会话顶栏右侧有 **工作台** 图标（也可 Composer 输入 `/novel`）。打开后与 Office 相同布局：

- **左**：书 → 卷 → 单元队列（每个单元一行：状态点 + `unit_id` + 章范围 + 一句话功能）  
- **中**：按当前选择切换视图：卷索引表 / 单元细纲卡 / 正文 / **人物墙**（按 `role` 分组的卡 + 三锚点 + 关系表）/ **关系图**  
- **右**：**单一主按钮** + 三行注入预览（题材文 / 单元卡 / on_stage 人物）  
- **底**：Composer 通栏  

小说工作台是 **流程控制台**（不是纯文件浏览器）：

1. **三阶段模型**：`planning`（无 canon 卷纲 → 规划一轮）→ `outlining`（卷纲已批准、仍有 `proposed` 细纲 → 一批细纲）→ `units`（逐单元 写 → 定稿）；本卷全部 `reviewed` 后回到 `planning` 出下一卷卷纲，或 `idle`
2. **`selectPrimaryAction`**：从 `novel-state.yaml` + 磁盘（`outline/volumes/*.md`、`outline/units/*.yaml` 的 `status`、`units/*.md`、`canon/cast/*.md` 的 `status`/`role`）推出唯一下一步；没有第二个 CTA
3. **单元状态机**：`proposed` → `accepted` → `drafted` → `reviewed`（对应 待细纲 → 待写 → 待定稿 → 已定稿）
4. **多模型**：写单元与定稿分 turn；用户在 Composer **自行**切换模型；工作台不自动换模

技能流水线：`novel-setup`（init）→ `novel-plan`（人物 + 总纲 + 卷纲 → 人批准 → `accept-volume`）→ `novel-write`（一批细纲 `lint-units` / 写单元 `preflight`）→ `novel-review`（`qc-pack` → 审 → 一次 Commit → `postcommit`）。写正文只消费 gate `### CONTEXT`；PASS 审稿不落盘。

动作 Prefill Composer（可勾选 `novel` chip）：**技能 + 意图/流程 + 书/卷/单元路径**。书落在 `novel/<book-id>/`（`canon/cast/<stem>.md`、`outline/volumes/vNN.md`、`outline/units/`、`units/`、`continuity/facts.md`、`continuity/summaries/`、`reviews/`）。章是 `units/vNN-U#.md` 里的 `## 第N章`，中间一行 `---`。

| 阶段 | UI 推断 | Agent 真执行 |
|------|---------|--------------|
| planning | 无 `outline/volumes/vNN.md` 或「本卷人物」含 `candidate` | `novel-plan` → 人批准 → gate `accept-volume --volume vNN` |
| outlining | 有 `status: proposed` 细纲 | `novel-write/references/unit-outline.md` → gate `lint-units --volume vNN` |
| units · 写 | 第一个 `accepted` 且无正文 | gate `preflight --unit` → 一份正文 |
| units · 定稿 | 第一个 `drafted` | gate `qc-pack --unit` → 审 → Commit → `postcommit --unit` |
| 人物墙 / 关系图 | 解析 `canon/cast/*.md` 头部与「关系」表 | 只读投影；关系两列由 Commit 回写 |

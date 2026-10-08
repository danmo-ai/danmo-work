# Continuity Commit

Commit = tools landed. 定稿轮里 **一次** `apply_patch` 覆盖：
- `continuity/summaries/vNN.md`（本单元每章一个 `## chNNN` 块）
- `continuity/facts.md`（公开事实 + cursor + Cast snapshot（含年龄/职位）+ Open loops；**不写章摘要**）
- 相关人物卡 `canon/cast/<stem>.md` 关系表两列（仅当 `state_deltas` 涉及关系质态）
- 单元细纲 `state_deltas`（结构化核对：与正文一致的身份跃迁）
- `continuity/commits/vNN-U#.md`（本单元执行日志；可含「本单元身份转变摘要」）
- 单元细纲 `status=reviewed`
- `novel-state.yaml`

## When

After review PASS (and optional polish) for the active unit. PASS 不要求 `reviews/` 文件。

一次任务提交整单元。**不要**拆成多次 postcommit。

## 分层：谁写哪里

| 文件 | 写什么 | 不写什么 |
|------|--------|----------|
| `continuity/summaries/vNN.md` | 每章 `## chNNN` 块（五要素）；卷收束时末尾追加 `### vNN 卷总结` | 执行日志、事实表 |
| `continuity/facts.md` | Public facts 新增 / 升级；cursor；Cast snapshot 增量（**年龄 / 职位 / 位置** 等按 `state_deltas` 重放到本单元末）；Open loops；每卷一行 `vNN → continuity/summaries/vNN.md` 索引 | `## chNNN` 块（旧书跑 `--action migrate` 拆出） |
| `canon/cast/<stem>.md` | 「当前质态」「最近变化点（含本单元 id）」两列；**不改**开卷年龄/本职基线 | 编年史、逐章流水、用改「本职」冒充当前职位 |
| `outline/units/vNN-U#.yaml` | 核对并必要时改写结构化 `state_deltas`（`stem`/`field`/`from`/`to`）与正文一致 | 把 `to` 写进人物卡基线 |
| `continuity/commits/vNN-U#.md` | gate 结果、四计数、锁词、字数、扩写 / deslop 处置、偏离登记、**本单元身份转变摘要** | 读者事实 |

### 1. `continuity/summaries/vNN.md`

新卷第一次 Commit 时新建（首行 `# Chapter summaries — vNN`），同时在 `facts.md` 的 `## Chapter summaries` 节加一行索引。之后每单元把章范围内每一章追加一块（按章号顺序，锚点用上一章块首行）：

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系 + 情绪增量 + 环境变化）
- 伏笔: FS-id PLANTED/ADVANCED/PAID（回收力度与埋设强调成正比）
- 钩子: 章末留下的具体悬念（必须可被下章 hook-in 直接承接）
- 下章指向: 接钩的第一拍
- 线索: thread PLANTED/ADVANCED/RESOLVED（可选）
```

五要素为 gate `postcommit` 硬检；第 6 行可选。

### 2. `continuity/facts.md`

1. **Public facts** — new shown_fact / inference from this chapter only
2. **Tracking** — cursor（`last_committed_ch` / 当前卷单元 / next action），Cast snapshot 增量，cannot-rewind
3. **Open loops** — plant / advance / pay off（≤5 open；`dangling` = bug）；细纲 `info_control.foreshadowing` 的 FS-id 必须出现在表中

细纲每条 `state_deltas` 的「谁」（`stem`）必须出现在 Cast snapshot（gate 硬检）。Snapshot「关系质态」列只做写作用的压缩游标：从变化的那条卡关系边抄一句，不在这里新编。

**身份跃迁回写：** 对照正文与细纲，确认本单元职位/年龄/位置等变化成立后：
1. 细纲 `state_deltas` 用结构化项（`field: title|age|location|power|relation|…` + `from`/`to`）；旧字符串 `"stem: 从X→Y"` 仍接受。
2. Cast snapshot 对应列改为本单元结束后的值（= 基线 + 截至本单元所有已 Commit delta 的重放结果）。
3. **不要**改人物卡「开卷年龄 / 本职」基线。

gate `postcommit`：身份类 delta 的 `to` 与 snapshot 年龄/职位/位置不一致 → advisory。

### 3. 人物卡关系回写

`state_deltas` 涉及关系质态变化（`field: relation` 或文本含信任±/站队/债务/秘密共享）时，**同一 patch** 更新**两张**相关卡「关系」表的「当前质态」与「最近变化点」两列；「最近变化点」须含本单元 id（如 `v01-U4 同盟成形`）。两边质态可以不同。关系状态的事实源是人物卡，facts 不另存。**不写编年史**。

gate `postcommit`：关系类 delta 而对应卡「最近变化点」无本单元 id → warning。

### 4. `continuity/commits/vNN-U#.md`（执行日志）

定稿短路径：用 finalize pack 的 `### COMMIT_SKELETON` 整段 `write` 到 `path:`（勿 `read_skill` 猜 `assets/`，勿读旧 commits）。完整字段说明见同目录 `commit-log.md`（仅深审 / 人工查阅）。结构：

- 本单元 commit 时间、unit_id、章范围
- gate 结果（preflight / qc-pack / postcommit 的 VERDICT 摘要）
- 四计数（em_dash / ai_vocab / english_leak / simile）
- 锁词扫描结果（命中数 / 未命中）
- 字数实测（单元 runes / 各章 runes vs floor/ceiling）
- 扩写技术（若用了）：用了哪几种、每章≤3 种
- deslop 处置（若用了）：删了哪类 AI 味
- 偏离登记（若有）：正文与细纲哪里不一致、如何回写
- 下一单元指引（一句话）

**facts.md 不重复这些**。facts 只回答"读者现在知道什么"，commits 回答"我们这一轮做了什么"。

## Tool actions（少交互）

1. Ensure final text in `units/vNN-U#.md`（章标题与 `---` 仍在）。
2. Set `outline/units/vNN-U#.yaml` `status=reviewed`.
3. Append `## chNNN` blocks to `continuity/summaries/vNN.md`（新卷则新建 + facts 索引行）。
4. Patch `continuity/facts.md`：事实 + cursor + snapshot + loops。
5. Patch 相关人物卡关系两列（若有关系变化）。
6. Write `continuity/commits/vNN-U#.md`。
7. Update `novel-state.yaml`（`last_committed_ch` = 章范围末章，`active_unit` 指向下一单元或清空，gates）。
8. Optional: `memory_update` / `table_upsert` — **默认不做**.
9. Gate `--action postcommit --unit vNN-U#`. FAIL → do not claim Commit.

## 时间线纪律（Commit 前自查）

5 规则：章内时间只前进；章间隔须明示（「三日后」）；多 POV 同时事件不矛盾；季节/月相/昼夜对齐；旅行时间物理可行。

细纲须有 `gap_from_prev`；须有 `story_day` 或 `time_label`（accepted+ gate blocking）。非 `flashback` 时 `story_day` 相对前序单元不得回跳（gate `timeline_monotonic`）。

### Commit 前一致性自查（对照 qc-pack `### CONTINUITY`）

| 项 | 通过标准 |
|----|----------|
| 身份 | 开场=CONTINUITY identity；单元末落地 `to`；snapshot 年龄/职位/位置已刷新 |
| 知情 | POV「不知」未在正文泄漏；知情范围列与揭示一致 |
| 伤势/资源 | 开场对齐 snapshot；变化在正文或 deltas 有交代 |
| 时钟 | `story_day`/`gap` 与正文时间句一致；闪回已标 |

| 常见错误 | 如何抓 | 如何修 |
|----------|--------|--------|
| 到达太快 | 对照地图距离与交通方式 | 加过渡章或改距离设定 |
| 事件顺序矛盾 | summaries 摘要块顺序 vs 正文 | 改后发生章节的措辞 |
| 昼夜错位 | 上章深夜、下章同场正午无间隔说明 | 补时间间隔句 |
| 伤愈太快 | Cast snapshot 伤势栏 vs 正文行动 | 保留伤势代价或加 healing 设定 |
| 分身两地 | 同角色同时间两个地点 | 以 facts 位置栏为准改正文 |
| 职位/年龄穿帮 | CONTINUITY identity vs 正文称呼 | 以 identity + 已 Commit delta 为准；本单元 `to` 只在单元末落地 |
| 开场写穿 | 开场已用 `to` 终局身份 | 回改正文或调整 delta / 场面顺序 |

## 卷收束（人工确认后执行）

卷末章 Commit 完成后提示用户：「可做卷收束」。用户确认后：

1. 全卷通读 + 对 facts.md / summaries/vNN.md 做 6 项核验（伏笔/线索/时间线/人物状态/世界规则/钩子链）。
2. 在 `continuity/summaries/vNN.md` **末尾**追加 `### vNN 卷总结`（500–800 字：事件主线 / 人物状态 / 带入下卷的线索 / 未回收伏笔+预期回收卷）。章块不搬动。
3. `reviews/backpatch.md` 队列清零或显式延期（FORCED PASS 遗留须处理）。
4. 下一卷卷纲由 `novel-plan` 出；批准后 `--action accept-volume --volume vNN+1`。

## 组装门（全书/卷交付前）

- 伏笔无 `open` 超 1 卷未推进、无 `dangling`
- 线索无无由 ACTIVE；PARKED 有叙事理由且已恢复
- 全部单元细纲 `status=reviewed` 且范围内摘要块齐五要素
- 全书已定稿单元 `qc-pack --unit` exit 0
- 其余 14 项见 `review-gates.md` Assembly Checklist

## Resume

Cold start: gate `--action doctor` → `novel-state` → `continuity/facts.md`（事实 + cursor + snapshot + loops；勿通读 `summaries/`）→ 当前卷 `summaries/vNN.md` 只看上一章块 → next 单元细纲.
doctor ADVISORY 出现 `[migrate]` → 先跑 `--action migrate`（章摘要拆卷、补 genre / on_stage / role / 本卷人物），核对 `continuity/commits/migrate-<date>.md`。
If legacy `ledger.md` exists without `facts.md` → migrate: split into `facts.md` (facts) + `commits/` (per-unit logs).

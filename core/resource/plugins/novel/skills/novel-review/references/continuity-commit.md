# Continuity Commit

Commit = tools landed. Prefer **one** `apply_patch` covering：
- `continuity/facts.md`（每章摘要 + cast snapshot + open loops）
- `continuity/commits/vNN-U#.md`（本单元执行日志）
- 单元细纲 `status=reviewed`
- `novel-state.yaml`

## When

After review PASS (and optional polish) for the active unit. PASS 不要求 `reviews/` 文件。

一次任务提交整单元。**不要**拆成多次 postcommit。

## Snapshot 分层

### 1. 写入 `continuity/facts.md`（读者事实，不写执行日志）

Update `continuity/facts.md` in one pass:

1. **Public facts** — new shown_fact / inference from this chapter only
2. **Tracking** — cursor, cast snapshot deltas, cannot-rewind
3. **Open loops** — plant / advance / pay off（≤5 open；`dangling` = bug）；细纲 `info_control.foreshadowing` 的 FS-id 必须出现在表中
4. **Chapter summaries** — append fixed block（五要素齐全，gate 硬检；第 6 行「线索」可选）:

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系 + 情绪增量 + 环境变化）
- 伏笔: FS-id PLANTED/ADVANCED/PAID（回收力度与埋设强调成正比）
- 钩子: 章末留下的具体悬念（必须可被下章 hook-in 直接承接）
- 下章指向: 接钩的第一拍
- 线索: thread PLANTED/ADVANCED/RESOLVED（可选）
```

细纲每条 `state_deltas` 的「谁」必须出现在 Cast snapshot。

**关系回写**：`state_deltas` 涉及关系质态变化（信任±/站队/债务/秘密共享）时，**同一 patch** 更新相关人物卡「关系」表的「当前质态」与「最近变化点」两列——关系状态的事实源是人物卡，facts 不另存。**不写编年史**，只改质态一句话 + 单元级节点。

### 2. 写入 `continuity/commits/vNN-U#.md`（执行日志）

本单元做了什么、gate 结果、四计数、锁词扫描、扩写技术 → 全部写这里，**不进 facts.md**。
模板见 `commit-log.md`。结构：

- 本单元 commit 时间、unit_id、章范围
- gate 结果（preflight/precommit/postcommit 的 VERDICT 摘要）
- 四计数（em_dash / ai_vocab / english_leak / simile）
- 锁词扫描结果（命中数 / 未命中）
- 字数实测（单元 runes / 各章 runes vs floor/ceiling）
- 扩写技术（若用了）：用了哪几种、每章≤3 种
- deslop 处置（若用了）：删了哪类 AI 味
- 偏离登记（若有）：正文与细纲哪里不一致、如何回写
- 下一单元指引（一句话）

**facts.md 不重复这些**。facts 只回答"读者现在知道什么"，commits 回答"我们这一轮做了什么"。

**Patch 锚点规范**：摘要块追加到 `## Chapter summaries` 节末尾，锚点用上一章块首行（`## chNNN-1 …` 起 5–7 行）作为上文；Open loops / Cast snapshot 用整行替换。

## Tool actions（少交互）

1. Ensure final text in `units/vNN-U#.md`（章标题与 `---` 仍在）。
2. Set `outline/units/vNN-U#.yaml` `status=reviewed`.
3. Patch `continuity/facts.md`：该单元章范围内每一章一块摘要（事实 + tracking + loops 一并更新）。
4. Write `continuity/commits/vNN-U#.md`：本单元执行日志（按 `commit-log.md` 模板）。
5. Update `novel-state.yaml`（`last_committed_ch` = 章范围末章，`active_unit` 可清空或指向下一单元，gates）。
6. Optional: `memory_update` / `table_upsert` — **默认不做**.
7. Gate `--action postcommit --unit vNN-U#`. FAIL → do not claim Commit.

## 时间线纪律（Commit 前自查）

5 规则：章内时间只前进；章间隔须明示（「三日后」）；多 POV 同时事件不矛盾；季节/月相/昼夜对齐；旅行时间物理可行。

| 常见错误 | 如何抓 | 如何修 |
|----------|--------|--------|
| 到达太快 | 对照地图距离与交通方式 | 加过渡章或改距离设定 |
| 事件顺序矛盾 | facts 摘要块顺序 vs 正文 | 改后发生章节的措辞 |
| 昼夜错位 | 上章深夜、下章同场正午无间隔说明 | 补时间间隔句 |
| 伤愈太快 | Cast snapshot 伤势栏 vs 正文行动 | 保留伤势代价或加 healing 设定 |
| 分身两地 | 同角色同时间两个地点 | 以 facts 位置栏为准改正文 |

## 卷收束（体积治理，人工确认后执行）

卷末章 Commit 完成后提示用户：「可做卷收束」。用户确认后：

1. 全卷通读 + 对 facts.md 做 6 项核验（伏笔/线索/时间线/人物状态/世界规则/钩子链）。
2. 写 `### vNN 卷总结`（500–800 字：事件主线 / 人物状态 / 带入下卷的线索 / 未回收伏笔+预期回收卷）进 facts.md `## Volume summaries`。
3. 该卷 `## chNNN` 明细块**移入** `continuity/summaries/vNN.md` 归档（facts 只留卷总结）。
4. `reviews/backpatch.md` 队列清零或显式延期（FORCED PASS 遗留须处理）。
5. gate `postcommit` 对已归档章节仍然有效（自动查归档）。

## 组装门（全书/卷交付前）

- 伏笔无 `open` 超 1 卷未推进、无 `dangling`
- 线索无无由 ACTIVE；PARKED 有叙事理由且已恢复
- 全部单元细纲 `status=reviewed` 且范围内摘要块齐五要素
- 全书已定稿单元 `scan-deslop --unit` exit 0
- 其余 14 项见 `review-gates.md` Assembly Checklist

## Resume

Cold start: gate `--action doctor` → `novel-state` → `continuity/facts.md`（**Volume summaries + 当前卷明细**，勿读归档全量）→ next 单元细纲.
If legacy `ledger.md` exists without `facts.md` → migrate: split into `facts.md` (facts) + `commits/` (per-unit logs).

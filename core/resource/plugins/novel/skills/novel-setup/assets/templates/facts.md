# Continuity facts — {{title}}

**Reader-known facts + serial cursor only.** Rebuild from **Committed** chapters (`status=reviewed`). Never copy `canon/author-lore.md` into this file.
**本文件只存事实，不存执行日志。** 本单元做了什么、gate 结果、四计数、扩写技术 → 写 `continuity/commits/<unit_id>.md`，不追加到本文件。

Drafting loads this file (via gate CONTEXT). Leaking a future reverse here is a P0.

## Public facts

| Kind | Fact | First seen | Notes |
|------|------|------------|-------|
| shown_fact | | ch | 正文明确展示 |
| reader_inference | | ch | 读者可推断，正文未确认 |
| character_claim | | ch | 角色声称 / 相信 |
| rumor | | ch | 流言 |
| misdirection | | ch | 有意误导 |

Kinds: `shown_fact` | `reader_inference` | `character_claim` | `rumor` | `misdirection`.

## Tracking

### Cursor

- last_committed_ch:
- current volume / unit:
- next action:

### Cast snapshot（上场角色；只记当前质态，不写编年史）

| 角色 | 位置 | 目标 | 伤势/资源 | 知情范围 | 关系质态（一行） |
|------|------|------|-----------|----------|------------------|
| | | | | | |

### Cannot rewind

Problems in accepted prose that later chapters must patch (not silently rewrite):

-

## Open loops（伏笔单一状态表）

| FS-id | 内容 | 状态 | 埋点章 | 计划回收 | 实际回收 |
|-------|------|------|--------|----------|----------|
| FS-001 | | planted | ch | vNN | |

Status: `planted` | `advanced` | `paid` | `dropped`.
Open count ≤ 5. `dangling`（埋了再无下文、未登记 dropped）= bug，卷收束/组装门硬检不得存在。
伏笔回收力度与埋设时的强调程度成正比。

## Chapter summaries

章摘要**不写在本文件**。Commit 时直接写 `continuity/summaries/vNN.md`（每章一个 `## chNNN` 块，五要素为 gate 硬检必填）；本节只留每卷一行索引。卷收束只在该卷文件末尾追加 `### vNN 卷总结`。本文件体积 ≈ O(事实 + 人物 + 伏笔)，不随章数增长。

- v01 → continuity/summaries/v01.md

`## chNNN` 块格式（写进 `summaries/vNN.md`）：

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系 + 情绪增量 + 环境变化）
- 伏笔: FS-id PLANTED/ADVANCED/PAID（回收力度与埋设强调成正比）
- 钩子: 章末留下的具体悬念（必须可被下章 hook-in 直接承接）
- 下章指向: 接钩的第一拍
- 线索: thread PLANTED/ADVANCED/RESOLVED（可选）
```

卷总结（卷收束时追加到 `summaries/vNN.md` 末尾）：

```markdown
### vNN 卷总结（500–800 字）
- 事件主线：…
- 人物状态：各主要角色卷终状态一行
- 带入下卷的线索：…
- 未回收伏笔 + 预期回收卷：FS-id → vNN
```

旧书若本文件仍有 `## chNNN` 块：跑 `novel_gate.py --action migrate` 拆到卷文件。gate `postcommit` 到 `summaries/vNN.md` 核对。

## After each Commit

1. Add only facts the new chapter showed or let a careful reader infer.
2. Promote `reader_inference` → `shown_fact` when the prose confirms.
3. Update cursor, cast snapshot deltas（关系质态列只从变化的卡关系边抄一句）, open loops, cannot-rewind.
4. `## chNNN` 摘要块写到 `continuity/summaries/vNN.md`；新卷第一次 Commit 时在上面索引加一行。
5. Do not write author-only truths, unused 终局储备, or next-volume unlocks.
6. **执行日志**（gate 结果/四计数/扩写技术/锁词扫描）写到 `continuity/commits/<unit_id>.md`，不进本文件。
7. 卷末章 Commit 后提示用户：可做卷收束（在 `summaries/vNN.md` 末尾写 `### vNN 卷总结`）。

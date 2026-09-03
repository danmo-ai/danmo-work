# Continuity ledger — {{title}}

**Reader-known + serial cursor only.** Rebuild from **Committed** chapters (`status=reviewed`). Never copy `canon/author-lore.md` into this file.

Drafting loads this file — leaking a future reverse here is a P0.

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

### Cast snapshot（上场角色）

| 角色 | 位置 | 目标 | 伤势/资源 | 知情范围 |
|------|------|------|-----------|----------|
| | | | | |

### Cannot rewind

Problems in accepted prose that later chapters must patch (not silently rewrite):

-

## Open loops

| ID | Type | Summary | Planted | Status |
|----|------|---------|---------|--------|
| | hook / FS / debt | | ch | open |

Open count ≤ 5. Status values: `open` | `advanced` | `paid` | `dropped`。
`dangling`（埋了再无下文、未登记 dropped）= bug，卷收束/组装门硬检不得存在。
伏笔回收力度与埋设时的强调程度成正比。

## Volume summaries

卷收束时追加（人工确认后归档该卷明细，见下）：

```markdown
### vNN 卷总结（500–800 字）
- 事件主线：…
- 人物状态：各主要角色卷终状态一行
- 带入下卷的线索：…
- 未回收伏笔 + 预期回收卷：FS-id → vNN
```

## Chapter summaries

Append a fixed block per Commit（五要素为 gate 硬检必填；第 6 行可选）:

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系 + 情绪增量 + 环境变化）
- 伏笔: FS-id PLANTED/ADVANCED/PAID（回收力度与埋设强调成正比）
- 钩子: 章末留下的具体悬念（必须可被下章 hook-in 直接承接）
- 下章指向: 接钩的第一拍
- 线索: thread PLANTED/ADVANCED/RESOLVED（可选）
```

**体积治理**：本文件只保留**当前卷**的 `## chNNN` 明细。卷收束（人工确认）后：
该卷摘要块移入 `continuity/summaries/vNN.md` 归档，本节只留 `### vNN 卷总结`。
ledger 体积 ≈ O(当前卷章数 + 卷数×800字)，不随全书无限增长。
gate `postcommit` 同时接受 ledger 与 `continuity/summaries/` 归档中的摘要块。

## After each Commit

1. Add only facts the new chapter showed or let a careful reader infer.
2. Promote `reader_inference` → `shown_fact` when the prose confirms.
3. Update cursor, cast snapshot deltas, open loops, cannot-rewind.
4. Append `## chNNN` summary block（追加到 `## Chapter summaries` 节末尾，锚点用上一章块的首行 `## chNNN`）。
5. Do not write author-only truths, unused 终局储备, or next-volume unlocks.
6. 卷末章 Commit 后提示用户：可做卷收束（写 `### vNN 卷总结` + 归档明细）。

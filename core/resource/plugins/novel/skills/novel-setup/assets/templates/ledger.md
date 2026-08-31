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

Open count ≤ 5. Status values: `open` | `advanced` | `paid`.

## Chapter summaries

Append a fixed 5-line block per Commit:

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系）
- 伏笔: FS-id plant/advance/payoff
- 钩子: 章末留下的具体悬念
- 下章指向: 接钩的第一拍
```

## After each Commit

1. Add only facts the new chapter showed or let a careful reader infer.
2. Promote `reader_inference` → `shown_fact` when the prose confirms.
3. Update cursor, cast snapshot deltas, open loops, cannot-rewind.
4. Append `## chNNN` summary block.
5. Do not write author-only truths, unused 终局储备, or next-volume unlocks.

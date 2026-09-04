# Continuity Commit

Commit = tools landed. Prefer **one** `apply_patch` covering ledger + contract + novel-state.

## When

After review PASS (and optional polish), before starting the next chapter. PASS 不要求 `reviews/` 文件。

## Snapshot（写入 ledger，勿拆多文件）

Update `continuity/ledger.md` in one pass:

1. **Public facts** — new shown_fact / inference from this chapter only  
2. **Tracking** — cursor, cast snapshot deltas, cannot-rewind  
3. **Open loops** — plant / advance / pay off（≤5 open；`dangling` = bug）；合同 `info_control.foreshadowing` 的 FS-id 必须出现在表中  
4. **Chapter summaries** — append fixed block（五要素齐全，gate 硬检；第 6 行「线索」可选）:

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系 + 情绪增量 + 环境变化）
- 伏笔: FS-id PLANTED/ADVANCED/PAID
- 钩子: 章末留下的具体悬念（可被下章 hook-in 直接承接）
- 下章指向: 接钩的第一拍
- 线索: thread PLANTED/ADVANCED/RESOLVED（可选）
```

合同每条 `state_deltas` 的「谁」必须出现在 Cast snapshot。

**关系回写**：`state_deltas` 涉及关系变化（信任±/站队/债务/秘密共享）时，**同一 patch** 更新相关人物卡「关系」表的「当前」列——关系状态的事实源是人物卡，ledger 不另存；不回写会导致下次 preflight 注入过期的关系行。

**Patch 锚点规范**：摘要块追加到 `## Chapter summaries` 节末尾，锚点用上一章块首行（`## chNNN-1 …` 起 5–7 行）作为上文；Open loops / Cast snapshot 用整行替换。避免对超大 ledger 做模糊匹配。

## Tool actions（少交互）

1. Ensure final text in `chapters/chNNN.md`.  
2. Set `chapters/chNNN-contract.yaml` `status=reviewed`.  
3. Patch `continuity/ledger.md` (facts + tracking + loops + summary).  
4. Update `novel-state.yaml` (`last_committed_ch`, stage, gates).  
5. Optional: `memory_update` / `table_upsert` — **默认不做**.  
6. Gate 脚本 `--action postcommit --chapter N`. FAIL → do not claim Commit.

## 时间线纪律（Commit 前自查）

5 规则：章内时间只前进；章间隔须明示（「三日后」）；多 POV 同时事件不矛盾；季节/月相/昼夜对齐；旅行时间物理可行。

| 常见错误 | 如何抓 | 如何修 |
|----------|--------|--------|
| 到达太快 | 对照地图距离与交通方式 | 加过渡章或改距离设定 |
| 事件顺序矛盾 | ledger 摘要块顺序 vs 正文 | 改后发生章节的措辞 |
| 昼夜错位 | 上章深夜、下章同场正午无间隔说明 | 补时间间隔句 |
| 伤愈太快 | Cast snapshot 伤势栏 vs 正文行动 | 保留伤势代价或加 healing 设定 |
| 分身两地 | 同角色同时间两个地点 | 以 ledger 位置栏为准改正文 |

## 卷收束（体积治理，人工确认后执行）

卷末章 Commit 完成后提示用户：「可做卷收束」。用户确认后：

1. 全卷通读 + 对 ledger 做 6 项核验（伏笔/线索/时间线/人物状态/世界规则/钩子链）。
2. 写 `### vNN 卷总结`（500–800 字：事件主线 / 人物状态 / 带入下卷的线索 / 未回收伏笔+预期回收卷）进 ledger `## Volume summaries`。
3. 该卷 `## chNNN` 明细块**移入** `continuity/summaries/vNN.md` 归档（ledger 只留卷总结）。
4. `reviews/backpatch.md` 队列清零或显式延期（FORCED PASS 遗留须处理）。
5. gate `postcommit` 对已归档章节仍然有效（自动查归档）。

## 组装门（全书/卷交付前）

- 伏笔无 `open` 超 1 卷未推进、无 `dangling`
- 线索无无由 ACTIVE；PARKED 有叙事理由且已恢复
- 全部章 `status=reviewed` 且摘要块齐五要素
- 全书 `scan-deslop --from 1 --to N` exit 0
- 其余 14 项见 `review-gates.md` Assembly Checklist

## Resume

Cold start: gate `--action doctor` → `novel-state` → `ledger.md`（**Volume summaries + 当前卷明细**，勿读归档全量）→ next contract.  
If legacy continuity files exist without ledger → merge into `ledger.md` then archive.

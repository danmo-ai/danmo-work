# Chapter write (Draft A)

写正文：**先跑 gate preflight，只消费它打印的 `### CONTEXT` + 本章合同。** 不要为走流程扫全书树。

## Preflight

1. `exec_shell` gate `--action preflight --chapter N`（`novel-setup/references/gate.md`）。exit ≠ 0 → **停止**。接手旧书另跑 `--action doctor`。
2. 读 stdout 的 `### CONTEXT`（接钩 / 人物现场 / 开放债务 / 本章硬约束 / 单元功能）。**这是本轮唯一额外上下文。**
3. 读本章 `chapters/chNNN-contract.yaml`（须 `accepted`；`unit_id` 对上卷纲单元）。
4. 可选：`search_kb` **至多 1** 次（默认「文风与去 AI 味」；章末钩/接钩可换「爽点与追读」）。
5. **ch001–ch003** → 另 `read_skill` `opening-chapters.md` + KB「节奏与结构」。
6. beats 含场景标签（对话/打斗/系统/`scene:establish` 等）→ `search_kb` **情绪与场景** 对应小节（见 KB `06`）；**不要**再 `read_skill` 单独场景路由页。
7. 仅当合同 `continuity_risks` 非空 → 才可 `read_file` 点名旧章。

**禁止：** `canon/author-lore.md`、整本 bible 终局细节、全卷纲、全书 `canon/` 通读、强制 `table_*`。

### 写作前 9 问（Draft 前内心过一遍，全部能答才动笔）

1. 本章工作窗口：合同 beats + CONTEXT 给的上章接钩是什么？
2. 上场角色的 Current State（位置/伤势/资源/知情）与 ledger Cast snapshot 一致？
3. 本章要碰的伏笔 FS-id（plant/advance/payoff）与 Open loops 表对得上？
4. 有没有该推进的 ACTIVE 线索（每 2–3 章须可见进展）？
5. 本章涉及的世界规则已在 canon/world.md 立过？新规则是否在影响剧情**之前**建立（禁机械降神）？
6. 开篇是否直接承接上章 hook.out（500 字内动作/对白接钩）？
7. 时间线：章间隔、昼夜、旅程时长与上章不矛盾？
8. POV 知情范围：第一人称不知他人想法；全知换头有场景分隔？
9. 有无任何与 ledger/合同矛盾的状态发明？（卫星文件里没有的事实就不存在）

写入 `novel-state.yaml`：

```yaml
gates:
  knowledge: pass|fail|unknown
  asset: pass|fail|unknown
  qc: unknown
blockers: []
last_preflight: "[YYYY-MM-DD chNNN] state:writing | contract:accepted | gate:PASS"
```

## Draft

- `write` `novel/<book-id>/chapters/chNNN.md`（零填充 ≥3 位）。
- 对齐 CONTEXT + 合同：`beats` / `forbidden` / POV 知情范围 / `pleasure_point` / `hook.out` / `word_target`。
- Modes：**full**（默认）/ **fast**（仅用户要求）。

## After draft

1. 合同 `status=drafted`（`table_upsert` 可选，默认不做）。
2. Hand off `novel-review`（审 / 润色 / Commit）。同 turn 不强制满审；勿开下一章除非 Commit 或用户明示批次。

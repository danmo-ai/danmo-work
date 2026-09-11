# Batch draft（批量正文首稿）

冻结批次内、**用户明示批量写 / Workbench `batch-write`** 时：同 turn 写多章**首稿**到 `drafted`，然后 **stop**。扩写 / 审 / 润色 / Commit → 另开 `novel-review`（`batch-review.md`）。

## 前置

1. `novel-state.yaml` 有有效 `frozen_batch`（`from`–`to`）且 `artifacts.batch_freeze: frozen`。无则先 `batch-freeze.md`。
2. 区间内每章 `chapters/chNNN-outline.yaml` 已 `status=accepted`、`unit_id` 齐全。
3. 单章模式（用户只点写一章）→ 用 `chapter-write.md`，不要本协议。

## 流程（一轮）

1. `exec_shell` gate：
   ```bash
   python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
     --workdir . --book-id <slug> --action preflight --from A --to B
   ```
   exit ≠ 0 → 读失败章 `### CHAPTER N` BLOCKING，**停该章及之后**；已写章保留。不要整批重跑 doctor。
2. 按章序消费输出：每个 `### CHAPTER N` 下的 `### CONTEXT` + 读该章 `chNNN-outline.yaml`。
3. 按章 `write` `chapters/chNNN.md`（零填充 ≥3 位）。接钩：本章 CONTEXT「接钩（上章）」——首稿阶段以**上章纲 `hook.out`** 为准（不要求上章已 Commit）。
4. 每章写完：章纲 `status=drafted`。
5. 更新 `novel-state.yaml`：
   ```yaml
   last_preflight: "[YYYY-MM-DD chAAA-chBBB] state:writing | batch-draft | gate:PASS"
   ```
6. **本轮到此结束。** Hand off：建议用户换模后跑 `batch-review`（或逐章 `review-polish-commit`）。

## 纪律

- 禁止同 turn 扩写 / deslop / 审稿 / Continuity Commit。
- 禁止为走流程扫全书树；禁止 `author-lore`。
- 风格指纹：区间 preflight 每章 CONTEXT 已注入；若本轮上下文被裁剪未见指纹 → `read_file canon/style-fingerprint.md` 一次即可（整批共用）。
- ch001–ch003：整批含开篇章时，另 `read_skill` `opening-chapters.md` **一次**（不要每章重复）。
- `search_kb` 整批 ≤1：含 ch1–3 →「节奏与结构」；否则默认「文风与去 AI 味」，`craft_lane=crime-human` →「刑侦人味文风」。
- 单章写失败（工具/质量自检）→ 停该章及之后；已落盘首稿保留 `drafted`。

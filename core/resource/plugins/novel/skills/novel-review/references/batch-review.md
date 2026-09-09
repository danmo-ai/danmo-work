# Batch review（批量扩写 · 审 · Commit）

对已 `drafted`（或 `review_fail`）的一批章：**同 turn 按章序**做扩写→审→可选润色→Continuity Commit。  
与 `novel-write` / `batch-draft` **分开**——不要在写作 turn 定稿。

## 范围

1. 优先 `frozen_batch.from`–`to` 内、章纲 `status=drafted`（或审稿 FAIL 待修）的章。
2. 用户指定区间时按用户区间；缺正文或未起草的章跳过并记入 EVIDENCE。
3. 单章 → 用 `review-gates.md` + `continuity-commit.md`，不必本协议。

## 流程（一轮）

### A. 可选整批反 AI 预扫

```bash
python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action scan-deslop --from A --to B
```

有 P0 hits → 先按章修，再继续。不要整批重跑 `doctor`。

### B. 可选整批 precommit（拿 COUNTS）

```bash
python3 "${WORK_HOME}/plugins/novel/skills/novel-setup/scripts/novel_gate.py" \
  --workdir . --book-id <slug> --action precommit --from A --to B
```

按 `### CHAPTER N` 消费 COUNTS / BLOCKING。FAIL 章：扩写或修 P0 后对该章再跑 `--chapter N`。

### C. 按章序循环（禁止跳章 Commit）

对区间内每一章 **从小到大**：

1. 薄稿 / 字数不足 → `expansion.md`，再单章 `precommit --chapter N`。
2. 按 `review-gates.md` 做 10 维评分（引用 gate `### COUNTS`）。
   - **≥85 PASS** → 不写 `reviews/`；更 `gates.qc: pass`。
   - **FAIL / 深审 / REVISE** → 落盘 `reviews/chNNN-review.md`，**停后续章的 Commit**（本轮已 Commit 的章保留）；修复后用户可再点批量或单章续跑。
3. 可选 `polish-deslop.md`（可先用区间 scan-deslop 对照）。
4. **一次** `apply_patch` Commit（`continuity-commit.md`）+ gate `postcommit --chapter N`（**仅单章，无区间**）。exit ≠ 0 → 不宣称该章定稿，停后续。
5. 再进入下一章（ledger 已含本章，供下一章连续性质检）。

### D. 收尾

更新 state（`last_committed_ch` = 本批最后成功 Commit 的章）。Hand off：下一批冻结 / 卷收束。

## 纪律

- 同 turn 可多章定稿，但 **Commit 必须章序**，禁止并行或跳章。
- PASS 不落盘 review 文件。
- `search_kb` 按步各 ≤1；整批尽量复用已加载的 `review-gates.md`。
- 写与审分模：本技能不换模型；用户应在写作 turn 之后另开本 turn。

# Unit review（扩写 · 审 · Commit）

对已 `drafted` 的 **一个单元**：同 turn 可扩写→审→可选润色→一次 Continuity Commit。
与 `novel-write` **分开**——不要在写作 turn 定稿。一轮只处理 `active_unit` 这一份正文。

## 范围

1. `outline/units/<active_unit>.yaml` 的 `status=drafted`（或审稿 FAIL 待修），正文是 `units/<active_unit>.md`。
2. 不要按章拆任务。章摘要在 Commit 时从这一份里抽出，一次写完。

## 流程

1. `qc-pack --unit vNN-U#`（一次输出 precommit + scan-deslop：`### COUNTS` / `### HITS` / `expand_needed` / `polish_needed`）。
2. 总字数 &lt; `word_target` × 0.8，或某章薄于 `word_share` × 0.7 → 先 `expansion.md`（扩场面，不注水）。
3. 审：`review-gates.md`。PASS 不写 review 文件。FAIL / 深审写 `reviews/vNN-U#-review.md` 并停。
4. 可选润色后，`continuity-commit.md`：一次补丁覆盖该单元全部 `## chNNN`，细纲 `status=reviewed`，`last_committed_ch` 推到章范围末章。
5. `postcommit --unit vNN-U#` exit 0 才算定稿。

禁止跳到下一单元。下一单元另开写作 turn。

---
name: novel-review
source: builtin
description: Triggers on 定稿 / 重新定稿 / 审阅定稿 / finalize. pack-finalize → patch listed gaps → parallel check-* → seal-commit. New session. Not for first drafts (novel-write) or 细纲 (novel-outline).
license: MIT
compatibility: Requires write, edit, read_file, exec_shell; ask_user
metadata:
  author: danmo-work
  version: "5.2"
  category: creative-writing
---

# Novel Review（定稿）

**Stage 5/5.** 短路径：读本次 pack → 只改标明缺口 → 三条 `exec_shell` 同发 → `seal-commit`。字数 / 去 AI 味 / Commit 都在本技能。

钉死 `G="$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py"`。

## When to load

定稿 / 重新定稿 / 审阅定稿 / 扩→审→润→Commit / 审稿 / 去 AI 味 / Continuity Commit.

**不要**用本技能写首稿或细纲。

## Do

| 步 | 命令 / 动作 |
|----|-------------|
| 1 | `python3 "$G" pack-finalize --workdir . --book-id <slug> --unit vNN-U#` |
| 2 | `read_file` stdout 的 `file:`；若输出带截断提示，**同一步**对同一路径按工具说明用 `offset`/`limit` 并行续读。只改 pack **标明仍缺**的段（HITS 行 / LENGTH 短板 / COMMIT 粘贴块） |
| 3 | 无缺口，或改完后：同一助手消息发 **三条** `exec_shell`（`check-length` / `check-deslop` / `check-commit`，同 book/unit）。不要拼进一条 shell |
| 4 | 三个都 exit 0 → `python3 "$G" seal-commit --workdir . --book-id <slug> --unit vNN-U#` |

只消费本次 pack 与 pack 列出的缺口文件；不扫书树补上下文。COMMIT / SUMMARY 骨架在 pack 的 `### COMMIT_SKELETON` / `### SUMMARY_SKELETONS`——**直接 `write`/`edit` 到 `paths`**。不 `search_kb`、不读旧 `continuity/commits/*.md`、不 `read_skill` 猜 `assets/commit-log.md` / `novel-setup/...`（模板已在 pack）。check-* 的 **ADVISORY 不挡 seal**，也不单开改 `facts.md`（除非 pack `paths` 含 `continuity/facts.md`）。审稿 FAIL / 深审才 `search_kb`。

**`pack-finalize` exit ≠ 0 且 stdout 有 `### PACK` / `file:` = 有缺口，正常**：继续第 2 步读 pack 改缺口，不是工具坏了。

第 3 步是三个独立 tool call，例如：

- `exec_shell`: `python3 "$G" check-length --workdir . --book-id <slug> --unit vNN-U#`
- `exec_shell`: `python3 "$G" check-deslop --workdir . --book-id <slug> --unit vNN-U#`
- `exec_shell`: `python3 "$G" check-commit --workdir . --book-id <slug> --unit vNN-U#`

HITS / LENGTH 无缺口且 COMMIT 写「账本已齐」→ 第 2 步无文件可改，直接第 3 步。单章配额（约 2000–3500）以 pack LENGTH 为准。

## Fail

| 情况 | 做法 |
|------|------|
| pack-finalize exit ≠ 0 **且有** `file:` PACK | **继续**第 2 步（缺口包）；勿当工具失败去猜路径 / `search_kb` |
| pack-finalize 无 PACK / 无 `file:` / 脚本崩 | **停** |
| 任务缺 `unit` / `book` | **停** |
| 某 check exit 1 | 只改对应文件，**只重跑该项**（改正文 → length+deslop；改账本 → commit） |
| 三检未齐就 seal | `seal-commit` 拒绝；状态不动 |
| 脚本无 `### VERDICT` | 停，写报告 |
| 同一检查连续 3 次 FAIL | 停 |

## Stop

`seal-commit` 给出 `### SEAL`（reviewed + cursor）→ 停。一轮一个单元。卷末 → 提示卷收束；下一卷 → `novel-plan`。

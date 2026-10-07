---
name: novel-write
source: builtin
description: Triggers on 写细纲 / 重写细纲 / 一批细纲 / 写单元正文 / 重写正文 / 正文写作. Prefer hook PACK ready → read_file → write. Else prompt-pack --stage outline|write. Not for 定稿/扩写/审稿 — novel-review.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.1"
  category: creative-writing
---

# Novel Write（一批细纲 · 写单元）

**Stage 3–4/5.** 生产路径（工作台新会话）：**若 hook 已 PACK ready → 直接 `read_file` 包文件 → write → 停。** 否则钉死 `G="$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py"` 跑 prompt-pack。禁止 `find`/`glob`、`read_skill` 长文、扫树、二读 YAML、`search_kb`。

- **一批细纲 / 写细纲 / 重写细纲**：只读 pack，填或覆盖 YAML，`lint-units` 一次；FAIL 再补一次然后停。不读人物卡。
- **写单元 / 重写正文**：只读 pack CONTEXT，`write` 一份正文（不先读旧稿），不定稿，不跑 `preflight`/`qc-pack`。

卡文 / 续写例外才 `read_skill` `continuation.md`。

## When to load

写细纲 / 重写细纲 / 写一批细纲 / 写单元正文 / 重写正文 / 正文写作 / 续写 / 接手 / 卡文救援.

**不要**在本技能回合做：字数扩写、去 AI 味、审稿、Commit。不要写 `chapters/chNNN.md` 或章纲。

## Do

| Intent | Script | 读什么 | 写什么 |
|--------|--------|--------|--------|
| 一批细纲 / 单写一个细纲 | `prompt-pack --stage outline --volume vNN` 然后 `lint-units --volume vNN` | 仅 pack `file:` | `outline/units/*.yaml`（本批） |
| 写单元正文 | `prompt-pack --stage write --unit vNN-U#` | 仅 pack | `units/vNN-U#.md` + 细纲 `drafted` |
| 续写 / 卡文 | 同上；仍失败才 `read_skill` continuation | 仅 pack | 同上 |

**细纲必填：** `on_stage` ⊆ 卷纲「本卷人物」、`pov` ∈ `on_stage`。不改卷纲已定的 `function` / `next_hook.type`。

## Hard stops

- Pack / preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose.
- **One unit per turn.** 不同单元正文不批量；细纲可批。
- **Draft turn ends at the unit file.**
- **单元规模**：3–8 章默认，硬上限 10 章。

## Stop

一批细纲：`lint-units` ≤2 次。全 PASS → 报 `### UNITS` 并停。
写单元：`units/vNN-U#.md` on disk → `drafted` → 停。定稿另开**新会话**。

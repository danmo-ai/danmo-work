---
name: novel-write
source: builtin
description: Two intents — 一批细纲 (prompt-pack --stage outline, fill ≤4 proposed unit YAML, lint-units) and 写单元 (prompt-pack --stage write, read pack file only, draft one units/vNN-U#.md). Not for 扩写, deslop, review, or Commit — those are novel-review (new session).
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.1"
  category: creative-writing
---

# Novel Write（一批细纲 · 写单元）

**Stage 3–4/5.** 生产路径（工作台新会话）：**exec prompt-pack → read_file 包文件 → write 目标 → 停。** 禁止 `read_skill` 长文、扫树、二读 YAML、`search_kb`。

- **一批细纲**：`--action prompt-pack --stage outline --volume vNN`，填 ≤4 个 `proposed`，再 `lint-units`。
- **写单元**：`--action prompt-pack --stage write --unit vNN-U#`，只读 pack，一份 `units/vNN-U#.md`，不定稿。

卡文 / 续写例外才 `read_skill` `continuation.md`。

## When to load

写细纲 / 写一批细纲 / 写单元正文 / 续写 / 接手 / 卡文救援.

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

一批细纲：`lint-units` 全 PASS → 报 `### UNITS`。
写单元：`units/vNN-U#.md` on disk → `drafted` → 定稿另开**新会话**（`prompt-pack --stage finalize`）。

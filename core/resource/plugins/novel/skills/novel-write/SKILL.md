---
name: novel-write
source: builtin
description: Two intents — 一批细纲 (fill ≤4 proposed unit YAML heads into accepted contracts, then gate --action lint-units) and 写单元 (gate --action preflight, consume only its CONTEXT, draft one units/vNN-U#.md). Unit YAML is the single source of unit-level truth. Not for 扩写, deslop, review, or Commit — those are novel-review (separate turn / model).
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; Core table_*, memory_*, search_kb; ask_user
metadata:
  author: danmo-work
  version: "4.0"
  category: creative-writing
---

# Novel Write（一批细纲 · 写单元）

**Stage 3–4/5.** 两个意图，两种轮次：

- **一批细纲**：把 `accept-volume` 种出的 `proposed` 头填成 `accepted` 合同，一批 ≤4 个，写完跑 `lint-units --volume vNN`；本卷还有 `proposed` 再发下一批。
- **写单元**：一轮一个单元，`preflight --unit` → 只读 `### CONTEXT` → 一份 `units/vNN-U#.md` → 停。定稿另开一轮（`novel-review`，可换模）。

## When to load

写细纲 / 写一批细纲 / 写单元正文 / 续写 / 接手 / 卡文救援.

**不要**在本技能回合做：字数扩写、去 AI 味、审稿、Commit。不要写 `chapters/chNNN.md` 或章纲。

## Do

| Intent | Load | search_kb（≤1） | Script |
|--------|------|-----------------|--------|
| 一批细纲 | `unit-outline.md` + `unit-scale.md` | 节奏与结构（新名用文内取名短清单） | 写完 `--action lint-units --volume vNN`；FAIL 只补失败的那几个 |
| 单写一个细纲 | 同上 | 同上 | 同上 |
| 写单元正文 | `unit-write.md` | **默认不查**；单元含 ch1–3 → 唯一一次查「节奏与结构」+ `read_skill` `opening-chapters.md` | `--action preflight --unit vNN-U#` exit 0 |
| 续写 / 卡文 | `continuation.md` | 同「写单元正文」 | 同上 |
| 场景/对白质感 | `unit-write.md` + 按需 `scene-routing.md` | 情绪与场景（占掉本轮那一次） | 同上 |

**细纲必填新字段：** `on_stage`（本单元开口或被写到的 canon stem，⊆ 卷纲「本卷人物」）、`pov`（默认 POV stem，∈ `on_stage`）；场面可选 `who` / `pov`。卷纲已定的 `unit_id` / 章范围 / `function` / `next_hook.type` 不改（`function` 改了只 warning，以卷纲为准）。

**写正文只消费 CONTEXT。** preflight 已注入：风格指纹、题材专有文全文（`crime-human` 接人味篇）、卷纲索引行、渲染后的单元卡、上一钩、`on_stage` 人物（snapshot + 三锚点 + 1 条台词；`pov` 加「不知」）、开放债务、锁词。不再通读 YAML，不读人物卡，不查题材篇 / 人味篇。

## Hard stops

- No prose without `accepted` 细纲 + `on_stage` 全 `canon` + 存在 `canon` protagonist（preflight asset 门）。
- Gate preflight FAIL → no prose.
- Frozen_Canon unconfirmed → no prose.
- **One unit per turn.** 不同单元正文不批量；细纲可批。
- **Draft turn ends at the unit file.** 不扩写 / 不去 AI 味 / 不审 / 不 Commit（用户明示单轮例外除外）。
- **单元规模**：3–8 章默认，硬上限 10 章（`unit-scale.md`）。

## Stop

一批细纲：`lint-units` 全 PASS → 报 `### UNITS`，提示下一批或开始写单元。
写单元：`units/vNN-U#.md` on disk → 细纲 `status=drafted` → hand off `novel-review`，建议换模。

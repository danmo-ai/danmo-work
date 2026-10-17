---
name: novel-outline
source: builtin
description: Triggers on 写细纲 / 重写细纲 / 一批细纲 / 单元细纲. pack-outline → fill YAML → lint-outline. Not for 写正文 (novel-write) or 定稿 (novel-review).
license: MIT
compatibility: Requires write, edit, read_file, exec_shell; ask_user
metadata:
  author: danmo-work
  version: "1.1"
  category: creative-writing
---

# Novel Outline（一批细纲）

**Stage 3/5.** 只写 `outline/units/*.yaml`。不定稿、不写正文。短路径：读本次 pack → 填 YAML → lint → 停。

钉死 `G="$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py"`。

**本轮边界**：只读 `pack-outline` 落盘的 `file:`（含细纲字段速查）；只写 `outline/units/*.yaml`；只跑 `pack-outline` 与 `lint-outline`（≤3 次），FAIL 只补失败单元重跑该项。此三条之外（源码 / 目录扫描 / 其它技能 / KB / 正文）均属越界。

## When to load

写细纲 / 重写细纲 / 一批细纲 / 单元细纲.

**不要**用本技能写 `units/*.md` 或跑定稿检查。

## Do

| 步 | 命令 | 读 | 写 |
|----|------|----|----|
| 1 | `python3 "$G" pack-outline --workdir . --book-id <slug> --volume vNN` | 仅 pack `file:`；若截断则同文件并行 `offset` 续读 | — |
| 2 | 按 pack 填本批 ≤4 个 `proposed` YAML（重写只改任务指定的那份） | **只**本次 pack | `outline/units/*.yaml` |
| 3 | `python3 "$G" lint-outline --workdir . --book-id <slug> --volume vNN` | — | — |

只消费本次 pack：不读人物卡、不扫书目录、不 `search_kb`。`on_stage` ⊆ 卷纲「本卷人物」、`pov` ∈ `on_stage`。不改卷纲已定的 `function` / `next_hook.type`。

## Fail

| 情况 | 做法 |
|------|------|
| pack-outline exit ≠ 0 | **停**。不写 YAML |
| 任务缺 `volume` / `book` | **停**，不猜 |
| lint-outline FAIL | 只补失败单元，再 lint；最多 **3 次**。仍 FAIL → 停并报告 |
| 想写正文 | 换 `novel-write` 新会话 |

## Stop

`lint-outline` ≤3 次后停。全 PASS → 报 `### UNITS`。不写正文。

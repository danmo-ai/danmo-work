---
name: novel-write
source: builtin
description: Triggers on 写单元正文 / 重写正文 / 正文写作 / 续写. pack-write → write units/*.md → seal-write. Not for 细纲 (novel-outline) or 定稿 (novel-review).
license: MIT
compatibility: Requires write, edit, read_file, exec_shell; ask_user
metadata:
  author: danmo-work
  version: "5.2"
  category: creative-writing
---

# Novel Write（写单元正文）

**Stage 4/5.** 短路径：`pack-write` → 读 pack（题材写时 bullet + 指纹）→ **一次** `write` 全文 → `seal-write` → 停。**0 次 `search_kb`。** 不定稿、不跑三检；字数留给 `novel-review`（pack `LENGTH`）。

钉死 `G="$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py"`。

卡文 / 续写例外才 `read_skill` `continuation.md`。开篇卡文才 `opening-chapters`；场景卡文才 `scene-routing`——二者都会教 `search_kb`，**不要**在短路径默认加载。

## When to load

写单元正文 / 重写正文 / 正文写作 / 续写 / 接手 / 卡文救援.

**不要**写细纲、定稿、Commit。

## Do

| 步 | 命令 | 读 | 写 |
|----|------|----|----|
| 1 | `python3 "$G" pack-write --workdir . --book-id <slug> --unit vNN-U#` | stdout `file:`（题材 bullet 已在 CONTEXT）；若截断则同文件并行 `offset` 续读 | — |
| 2 | **一次** `write` 一份正文（全部章；重写可覆盖） | **只**本次 pack | `units/vNN-U#.md` |
| 3 | `python3 "$G" seal-write --workdir . --book-id <slug> --unit vNN-U#` | — | 细纲 `drafted` |

只消费本次 pack：**0 次 `search_kb`**、不扫书目录 / canon / 旧单元、不自跑字数脚本或对标章配额。题材与风格以 pack 为准。

**One unit per turn.** 单元 3–8 章默认，硬上限 10 章。

## Fail

| 情况 | 做法 |
|------|------|
| pack-write exit ≠ 0 | **停**。不写正文 |
| 任务缺 `unit` / `book` | **停** |
| seal-write 报缺正文 | 先写文件再 seal |
| 想定稿 | 另开 `novel-review` 新会话 |

## Stop

正文落盘 + `seal-write` → 停。定稿另开新会话。

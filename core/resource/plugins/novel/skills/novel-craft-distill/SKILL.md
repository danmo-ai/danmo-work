---
name: novel-craft-distill
source: builtin
description: Distill novel writing craft (style / narrative / dialogue / pacing) from a local book or excerpt into one Markdown file. Decoupled from the active novel/<book-id>/ tree — does not write canon, style-fingerprint, or preflight CONTEXT. Use later via KB import or by prompting “参考 <path> 写作”.
license: MIT
compatibility: Requires write, edit, read_file, grep, glob, exec_shell; ask_user
metadata:
  author: danmo-work
  version: "1.0"
  category: creative-writing
  adapted_from: "https://github.com/Shiaoming123/works-dna-extractor (methods layer); https://github.com/qyh9527/writing-style-distiller-skill (control_scope); https://github.com/mol632991-png/distill-novels (style/narrative/dialogue/pacing dims); https://github.com/woanderingboy/novel-writing-pipeline (fingerprint stats method); https://github.com/Lyrainit/NovelDistiller (long-text sampling flow)"
  upstream_license: MIT
---

# Novel Craft Distill（独立技法蒸馏）

**旁路工具。** 从用户指定的本地源文蒸馏「怎么写」，落盘为**一份** Markdown。与当前创作书解耦：不写 `novel/<book-id>/canon/*`，不改 `style-fingerprint`，不进 preflight。

Adapted from (selective, rewritten): Works DNA Extractor methods layer; writing-style-distiller `control_scope`; distill-novels craft dimensions; novel-writing-pipeline fingerprint stats method; NovelDistiller long-text sampling flow. Not verbatim.

## When to load

蒸馏文风 / 写作技法 / craft DNA / 手法指纹 / 「学这本书怎么写」（只要技法，不要剧情设定）。

**不要**用本技能：立项建树、写正文、审稿、去 AI gate、抽人物/世界观/情节大纲。

## Inputs

| 参数 | 必填 | 说明 |
|------|------|------|
| 源路径 | 是 | 本地 `.txt` / `.md` 单文件，或多章目录 |
| 输出路径 | 否 | 默认 `craft/<source-slug>-craft.md`（工作区根下，**不在** `novel/<book-id>/`） |

可选：先跑 `scripts/fingerprint_stats.py <源>`（stdout JSON，**零正文落盘**），把计量填进 dials。

## Do

1. `read_skill` 本技能后加载 `references/anti-content.md` + `craft-dimensions.md` + `craft-schema.md`；长文再加 `sampling.md`。
2. 确认源路径可读；缺输出路径则用默认 `craft/<slug>-craft.md`（`slug` = 源文件/目录名的 ascii/拼音短名）。
3. 按 `sampling.md` 取样（短摘录标 `confidence: low`）。
4. 可选：`exec_shell` `python3 …/fingerprint_stats.py <源>`，仅用 JSON 数字，不把正文写进产物。
5. 按四维提取技法 → 填满模板 `assets/templates/craft-techniques.md`。
6. **`write` 唯一输出文件**（UTF-8）。聊天罗列技法 = 未完成。
7. 跑 `references/quality-gate.md` 自检；失败则 `edit` 产物直至通过。

## Hard boundary

- 禁止写入：情节摘要、人名/地名专有设定、金手指规则、伏笔清单、时间线、角色关系网。
- 示例句 ≤80 字，只作句式/节奏形态示意；可匿名化为「他/她/那人」。
- 产物顶部必须有 `control_scope: craft-only` 声明（见 schema）。
- 一份文件；不要再写第二份 fingerprint / canon 副本。

## How to use the output later（本技能不自动接线）

1. **导入知识库**：用户把该 md 拷入/导入用户 KB（或自建篇）→ 写作轮 `search_kb` / `get_kb_doc`。
2. **提示参考**：用户说「按 / 参考 `<输出路径>` 写作」→ 当轮 `read_file` 该文档再写。

不要把产物塞进本书 `canon/` 或指望 preflight 自动注入。

## Stop

磁盘上存在唯一输出路径，且 `quality-gate.md` 清单全过 → 向用户回报**绝对或工作区相对路径** + 上述两种用法提示。

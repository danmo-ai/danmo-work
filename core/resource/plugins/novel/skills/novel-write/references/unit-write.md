# 单元正文

## Pipeline system

1. `python3 "$G" pack-write --workdir . --book-id <slug> --unit vNN-U#`。exit ≠ 0 → 停。
2. `read_file` stdout 的 `file:`（含题材写时 bullet + 风格指纹 + 结构约束 + 开篇规则）；若输出带截断提示，同一步对同一路径按工具说明用 `offset`/`limit` 并行续读。
3. **一次** `write` 一份 `units/vNN-U#.md`（全部章，章间 `---`；重写可覆盖）。**重写已存在文件 → 先 `read_file` 同路径再 `write`**。
4. `python3 "$G" seal-write --workdir . --book-id <slug> --unit vNN-U#` → 细纲 `drafted` → 停。

本轮边界以 `novel-write/SKILL.md` 顶部「本轮边界」为准。

## Canonical path

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/units/vNN-U#.md` |
| Format | Markdown prose; chapters separated by `---` |

卡文 / 续写例外才 `read_skill` `continuation.md`（仍优先吃 pack；确需技法再按该文 ≤1 次 `search_kb`）。

# 单元正文

## Pipeline system

1. `python3 "$G" pack-write --workdir . --book-id <slug> --unit vNN-U#`。exit ≠ 0 → 停。
2. `read_file` stdout 的 `file:`（含题材写时 bullet + 风格指纹）；若输出带截断提示，同一步对同一路径按工具说明用 `offset`/`limit` 并行续读。
3. **一次** `write` 一份 `units/vNN-U#.md`（全部章，章间 `---`；重写可覆盖）。
4. `python3 "$G" seal-write --workdir . --book-id <slug> --unit vNN-U#` → 细纲 `drafted` → 停。

**短路径硬轨：** 只读本次 pack。**0 次 `search_kb`**（题材/共性已在包或不属于本轮）。不扫书树、不自计字数。不定稿、不跑 check-* / seal-commit。定稿另开 `novel-review`。

不要加载 `opening-chapters.md` / `scene-routing.md`——那两条是**卡文 / 开篇救援例外**，会教 `search_kb`，与本短路径冲突。

## Canonical path

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/units/vNN-U#.md` |
| Format | Markdown prose; chapters separated by `---` |

卡文 / 续写例外才 `read_skill` `continuation.md`（仍优先吃 pack；确需技法再按该文 ≤1 次 `search_kb`）。

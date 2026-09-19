# Project layout

Canonical tree under the active project workdir. **Directory names are English and fixed.** Do not invent parallel Chinese trees (`设定集/`, `人物/`, `卷纲/`, `台账/`, …) for the same roles.

```text
novel/<book-id>/
  novel-state.yaml              # stage / artifacts / gates / active_unit / last_preflight / craft_lane
  book-bible.md                 # 读者承诺 + 唯一终局储备 unlock 表
  canon/
    world.md                    # 世界观四层；稀疏术语可写在本节（不必另建 glossary）
    author-lore.md              # 作者轨（写正文禁止加载）
    cast/                       # 人物卡；金手指默认写在主角卡
    writing-rules.md            # optional
    style-fingerprint.md        # optional（续写）
    proposals.md                # optional what-if
  outline/
    book_outline.md             # 总纲
    volumes/                    # v01.md … 剧情单元卡（粗）
    units/                      # v01-U1.yaml 单元细纲
  units/
    v01-U1.md                   # 单元正文：一份文件，章间单独一行 ---
  continuity/
    ledger.md                   # Public facts + Tracking + Open loops + ## chNNN 摘要
  reviews/
    v01-U1-review.md            # 仅 FAIL / 深审；PASS 不落盘
  extras/                       # optional non-Canon
  _archive/                     # optional migrated / replaced materials
```

`chapters/` is not canonical. Gate does not read it. If `chapters/` exists and `units/` has no unit prose, doctor blocks and tells the agent to migrate. This change does not ship a migrator.

## Rules

- `<book-id>`: short slug (ascii or pinyin), stable for the book.
- **Prose truth:** `units/vNN-U#.md` only. Chapters are headings inside that file (`## 第N章`), separated by a line that is exactly `---`. Scene breaks are blank lines, never a lone `---`.
- **单元细纲 truth:** `outline/units/vNN-U#.yaml` only (YAML).
- **Canon truth:** `canon/*`. Files are authoritative; table upserts are optional mirrors.
- **Outline truth:** book/volume plans under `outline/`. Volume outlines stop at **剧情单元卡**. Scenes and chapter cuts belong in the 单元细纲.
- **Continuity truth:** `continuity/ledger.md`. Commit still writes one `## chNNN` block per chapter, extracted from the unit file, in one patch.
- **Lore tracks:** `canon/author-lore.md` vs `continuity/ledger.md`. Do not merge author-lore into the ledger.
- **终局储备:** unlock table only in `book-bible.md`; details only in `author-lore.md`.
- **Active unit:** `novel-state.yaml` → `active_unit: vNN-U#`. No `frozen_batch`, no `batch-freeze.yaml`.

## Role map

| Role | Path |
|------|------|
| 设定（圣经 / 世界 / 人物） | `book-bible.md` + `canon/`（含 `cast/`） |
| 作者侧底牌 | `canon/author-lore.md`（写正文不加载） |
| Book & volume outlines | `outline/book_outline.md`, `outline/volumes/` |
| 单元细纲 | `outline/units/vNN-U#.yaml` |
| 单元正文 | `units/vNN-U#.md` |
| 读者已知 / 连载状态 / 伏笔 / 章摘要 | `continuity/ledger.md` |
| Review reports | `reviews/vNN-U#-review.md` |
| Non-Canon extras | `extras/` |
| Replaced migrations | `_archive/` |

Copy blanks from `novel-setup/assets/templates/` (bible, state, `world.md`, `cast-card.md`, `author-lore.md`, `ledger.md`) and `novel-write/assets/templates/unit-outline.yaml` via `read_skill` then `write`.

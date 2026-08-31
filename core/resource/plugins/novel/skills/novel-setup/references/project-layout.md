# Project layout

Canonical tree under the active project workdir. **Directory names are English and fixed.** Do not invent parallel Chinese trees (`设定集/`, `人物/`, `卷纲/`, `台账/`, …) for the same roles.

```text
novel/<book-id>/
  novel-state.yaml              # stage / artifacts / gates / frozen_batch / last_preflight
  book-bible.md                 # 读者承诺 + 唯一终局储备 unlock 表
  canon/
    world.md                    # 世界观四层；稀疏术语可写在本节（不必另建 glossary）
    author-lore.md              # 作者轨（写正文禁止加载）
    cast/                       # 人物卡；金手指默认写在主角卡
    writing-rules.md            # optional
    style-fingerprint.md        # optional（续写）
    proposals.md                # optional what-if
  outline/
    book_outline.md             # 结构/卷地图；不复制终局储备（见 bible）
    volumes/                    # v01.md … 剧情单元卡
  chapters/
    ch001-contract.yaml         # 章合同 (YAML only; required before draft)
    ch001.md                    # prose after draft / Commit
  continuity/
    ledger.md                   # Public facts + Tracking + Open loops + ## chNNN 摘要
  reviews/
    ch001-review.md             # PASS 短 stub；FAIL 全文
  extras/                       # optional non-Canon
  _archive/                     # optional migrated / replaced materials
```

## Legacy paths (read-only migrate)

Older books may still have `continuity/public-lore.md`, `tracking.md`, `chapter_summaries.md`, `foreshadow-tracker.md`, `batch-freeze.yaml`, `canon/glossary.md`. Prefer merging into `ledger.md` + `novel-state.frozen_batch` on cold-start (`novel-setup` doctor / skill note). Gate accepts either layout.

## Project-root source briefs (optional)

Author-imported briefs may live at the **project files root** (sibling of `novel/`), e.g. `提纲.md`, `分卷.md`. Treat them as source material: promote into `canon/` / `outline/`; do not treat project-root Chinese filenames as the book layout.

## Rules

- `<book-id>`: short slug (ascii or pinyin), stable for the book.
- **Prose truth:** `chapters/*.md` after Commit (never `*-contract.yaml`).
- **Chapter contract truth:** `chapters/chNNN-contract.yaml` only (YAML).
- **Canon truth:** `canon/*` (+ optional `table_*` index). Files are authoritative; table upserts are optional mirrors.
- **Outline truth:** book/volume plans under `outline/` only. Volume outlines stop at **剧情单元卡**. Per-chapter planning is **章合同** only (`unit_id` 回指单元).
- **Continuity truth:** `continuity/ledger.md` (reader facts + tracking + open loops + chapter summaries).
- **Lore tracks:** `canon/author-lore.md` (author-only) vs `continuity/ledger.md` (reader-known + serial cursor). Do not merge author-lore into the ledger.
- **终局储备:** unlock table only in `book-bible.md`; details only in `author-lore.md`. Do not duplicate the unlock table in `book_outline.md`.
- **Proposals / what-if:** stay in `outline/` or `canon/proposals.md` until user confirms → then promote to Canon.

## Role map (do not fork)

| Role | Path |
|------|------|
| 设定（圣经 / 世界 / 人物） | `book-bible.md` + `canon/`（含 `cast/`） |
| 作者侧底牌 | `canon/author-lore.md`（写正文不加载） |
| Book & volume outlines | `outline/` |
| 章合同 + prose | `chapters/` |
| 读者已知 / 连载状态 / 伏笔 / 章摘要 | `continuity/ledger.md` |
| Review reports | `reviews/` |
| Batch freeze | `novel-state.yaml` → `frozen_batch` only |
| Non-Canon extras | `extras/` |
| Replaced migrations | `_archive/` |

Copy blanks from `novel-setup/assets/templates/` (bible, state, `world.md`, `cast-card.md`, `author-lore.md`, `ledger.md`) and sibling skill templates via `read_skill` then `write`.

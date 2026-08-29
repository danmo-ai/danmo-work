# Project layout

Canonical tree under the active project workdir. **Directory names are English and fixed.** Do not invent parallel Chinese trees (`设定集/`, `人物/`, `卷纲/`, `台账/`, …) for the same roles.

```text
novel/<book-id>/
  novel-state.yaml              # stage machine / next action
  book-bible.md                 # 设定入口：读者承诺 + 终局储备索引（不是第二套世界观）
  canon/                        # 设定正文（与 table_* 同行权威）
    world.md                    # 世界观四层：规则 / 势力 / 日常 / 禁忌
    glossary.md                 # 名词表（一行释义）
    author-lore.md              # 作者轨：终局细节 / 隐藏规则 / 角色私密（写正文禁止加载）
    writing-rules.md            # optional 本书创作规范
    reveal-schedule.md          # optional 揭设时刻表
    platform-positioning.md     # optional 平台/书名/标签
    goldfinger.md               # optional 金手指独立卡（也可写在主角卡里）
    cast/                       # 人物卡
    research/                   # optional 考据
  outline/                      # book / volume planning (not chapter contracts)
    book_outline.md
    volumes/                    # v01.md … or equivalent volume briefs
  chapters/
    ch001-contract.yaml         # 章合同 (YAML only; required before draft)
    ch001.md                    # prose after draft / Commit
  continuity/                   # ledgers after / between chapters
    public-lore.md              # 读者轨：仅已 Commit 章节的公开事实
    tracking.md                 # 追踪轨：当前状态 / 开放钩 / 不可回改
    chapter_summaries.md
    foreshadow-tracker.md
    decision-log.md             # continuity issues / author rulings
  reviews/
    ch001-review.md
  extras/                       # optional non-Canon (author notes, travelogue, …)
  _archive/                     # optional migrated / replaced materials
```

## Project-root source briefs (optional)

Author-imported briefs may live at the **project files root** (sibling of `novel/`), e.g. `提纲.md`, `分卷.md`. Treat them as source material: promote into `canon/` / `outline/` / tables; do not treat project-root Chinese filenames as the book layout.

## Rules

- `<book-id>`: short slug (ascii or pinyin), stable for the book.
- **Prose truth:** `chapters/*.md` after Commit (never `*-contract.yaml`).
- **Chapter contract truth:** `chapters/chNNN-contract.yaml` only (YAML). See `chapter-contract.md`.
- **Canon truth:** `canon/*` + `table_*` rows with matching `book_id`. Prefer tables for queryable fields; Markdown for long lore.
- **Outline truth:** book/volume plans under `outline/` only. Volume outlines stop at **剧情单元卡**（一段章：节拍/功能/主角目标/因果/阻碍/关键选择/形态/禁提前/下钩）+ 锚点/弧/反转/终局边界. Per-chapter planning is **章合同** only (`unit_id` 回指单元).
- **Continuity truth:** open loops / rulings / rolling summaries under `continuity/` (+ matching `table_*` when queryable).
- **Lore tracks:** `canon/author-lore.md` (author-only) vs `continuity/public-lore.md` (reader-known, from Commit) vs `continuity/tracking.md` (current serial state). Do not fork a second Chinese 设定 tree.
- **Proposals / what-if:** stay in `outline/` or `canon/proposals.md` until user confirms → then promote to Canon + tables.
- **context packages** (if written) are assemblies with source paths — not a second Canon.

## Role map (do not fork)

| Role | Directory |
|------|-----------|
| 设定（圣经 / 世界 / 人物） | `book-bible.md` + `canon/`（含 `cast/`） |
| 作者侧底牌 | `canon/author-lore.md`（写正文不加载） |
| Book & volume outlines | `outline/` |
| 章合同 + prose | `chapters/` |
| 读者已知 / 连载状态 / 伏笔 | `continuity/public-lore.md` + `tracking.md` + 其余 continuity |
| Review reports | `reviews/` |
| Non-Canon extras | `extras/` |
| Replaced migrations | `_archive/` |

Copy blanks from `novel-setup/assets/templates/` (bible, state, `world.md`, `glossary.md`, `cast-card.md`, `goldfinger-card.md`, `author-lore.md`, `public-lore.md`, `tracking.md`) and sibling skill templates via `read_skill` then `write`.

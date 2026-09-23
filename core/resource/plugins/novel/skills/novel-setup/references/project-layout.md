# Project layout

Canonical tree under the active project workdir. **Directory names are English and fixed.**

```text
novel/<book-id>/
  novel-state.yaml              # stage / artifacts / gates / active_unit / last_preflight / genre / subgenre
  book-bible.md                 # 读者承诺 + 唯一终局储备 unlock 表（只写卷号，不写真相）
  canon/
    world.md                    # 世界观四层；金手指规则
    author-lore.md              # 作者轨（写正文/评审禁止加载；真相细节只在此）
    locked-terms.yaml           # 新增：锁词清单（按卷解锁；gate precommit 自动扫描）
    cast/                       # 人物卡；金手指默认写在主角卡
    writing-rules.md            # optional
    style-fingerprint.md        # optional
  outline/
    book_outline.md             # 总纲 + 伏笔单一状态表（FS-id / 状态 / 埋点 / 计划回收 / 实际回收）
    volumes/                    # v01.md … **瘦身 U 卡索引**（unit_id + 章范围 + 一句话功能 + 本单元禁碰）
    units/                      # v01-U1.yaml **单元级唯一事实源**（desire/obstacle/choice/payoff/scenes/chapters）
  units/
    v01-U1.md                   # 单元正文：一份文件，章间单独一行 ---
  continuity/
    facts.md                    # 读者已知事实 + cursor + cast snapshot + open loops + ## chNNN 摘要（当前卷）
    commits/                    # 每单元一个执行日志文件（v01-U1.md），不进 facts
    summaries/                  # 卷收束归档：v01.md …（旧卷 ## chNNN 明细）
  reviews/
    v01-U1-review.md            # 仅 FAIL / 深审；PASS 不落盘
  extras/                       # optional non-Canon
  _archive/                     # optional migrated / replaced materials
```

`chapters/` is not canonical. Gate does not read it.

## 分层原则（单一事实源）

| 层 | 文件 | 职责 | 禁止 |
|----|------|------|------|
| 卷级判断 | `outline/volumes/vNN.md` | 卷目标、起终状态、节奏锚点、终局边界、**单元索引** | 不写 desire/obstacle/choice/payoff/pleasure/forbidden/scenes/章切口 |
| 单元级合同 | `outline/units/vNN-U#.yaml` | **唯一**单元级事实源 | 不写卷级判断 |
| 正文 | `units/vNN-U#.md` | 纯 prose | 不写规划/分析/自检 |
| 读者事实 | `continuity/facts.md` | 读者已知 + cursor + loops + 当前卷章摘要 | 不写执行日志/门禁结果/gate 计数 |
| 执行日志 | `continuity/commits/vNN-U#.md` | 本单元做了什么、gate 结果、四计数、锁词扫描、扩写技术 | 不重复 facts 里的事实 |
| 作者底牌 | `canon/author-lore.md` | 真相细节 | 写正文/评审禁止加载 |

## Rules

- `<book-id>`: short slug (ascii or pinyin), stable.
- **Prose truth:** `units/vNN-U#.md` only. Chapters are headings inside that file (`## 第N章`), separated by a line that is exactly `---`. Scene breaks are blank lines.
- **单元细纲 truth:** `outline/units/vNN-U#.yaml` only (YAML).
- **Canon truth:** `canon/*`.
- **Outline truth:** book/volume plans under `outline/`. Volume outlines stop at **单元索引**（unit_id + 章范围 + 一句话功能 + 本单元禁碰）。Scenes and chapter cuts belong in the 单元细纲.
- **Continuity facts truth:** `continuity/facts.md`. Commit writes one `## chNNN` block per chapter + updates loops/cast snapshot in one patch.
- **Commit log truth:** `continuity/commits/vNN-U#.md`. Per-unit execution log, not appended to facts.
- **Lore tracks:** `canon/author-lore.md` vs `continuity/facts.md`. Do not merge author-lore into facts.
- **终局储备:** unlock table only in `book-bible.md`; details only in `author-lore.md`; **锁词清单 only in `canon/locked-terms.yaml`**.
- **Active unit:** `novel-state.yaml` → `active_unit: vNN-U#`.

## Role map

| Role | Path |
|------|------|
| 设定（圣经 / 世界 / 人物） | `book-bible.md` + `canon/`（含 `cast/`） |
| 作者侧底牌 | `canon/author-lore.md`（写正文不加载） |
| 锁词与解锁节奏 | `canon/locked-terms.yaml`（gate 自动扫描） |
| Book & volume outlines | `outline/book_outline.md`, `outline/volumes/` |
| 单元细纲（唯一单元级合同） | `outline/units/vNN-U#.yaml` |
| 单元正文 | `units/vNN-U#.md` |
| 读者已知 / 连载状态 / 伏笔 / 章摘要 | `continuity/facts.md` |
| 本单元执行日志 | `continuity/commits/vNN-U#.md` |
| Review reports | `reviews/vNN-U#-review.md` |
| Non-Canon extras | `extras/` |
| Replaced migrations | `_archive/` |

Copy blanks from `novel-setup/assets/templates/` (bible, state, `world.md`, `cast-card.md`, `author-lore.md`, `facts.md`, `locked-terms.yaml`) and `novel-write/assets/templates/unit-outline.yaml` via `read_skill` then `write`.

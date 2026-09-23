"""migrate: bring an older book onto the current layout (plan §10).

- facts.md `## chNNN` blocks → continuity/summaries/vNN.md (facts keeps one index row per volume)
- novel-state.yaml without `genre` → guessed once + blocker「确认 genre」
- `craft_lane` removed; `crime-human` on 悬疑 (or unset genre) becomes `subgenre: 刑侦探案`
- unit outlines without on_stage / pov → on_stage prefilled from state_deltas 谁; pov left empty (warning)
- cast cards without `role` → protagonist when the card says so, else recurring (warning)
- volume outlines without 「本卷人物」 → union of the volume's unit on_stage
Writes continuity/commits/migrate-<date>.md. Requires doctor PASS (layout + UTF-8) first.
"""
from __future__ import annotations

import datetime as _dt
import re
from pathlib import Path

from .cast import CAST_KV_RE, cast_paths, parse_cast_card
from .common import (
    Report,
    load_yaml_map,
    nonempty_list,
    read_book_text,
    set_state_scalar,
    write_book_text,
)
from .context import GENRES
from .ledger import ledger_path, split_chapter_summary_blocks, summaries_rel
from .outline import (
    chapter_range_of,
    list_unit_ids,
    load_unit,
    on_stage_of,
    parse_volume_cast,
    state_delta_who,
    unit_outline_rel,
    volume_outline_rel,
)


def _chapter_volume_map(book_root: Path) -> dict[int, str]:
    out: dict[int, str] = {}
    for uid in list_unit_ids(book_root, "outline"):
        try:
            u, _ = load_unit(book_root, uid)
        except (OSError, UnicodeDecodeError):
            continue
        a, b = chapter_range_of(u)
        vol = uid.split("-")[0]
        for ch in range(a, b + 1):
            out[ch] = vol
    return out


def migrate_summaries(book_root: Path, r: Report, changes: list[str]) -> None:
    lp = ledger_path(book_root)
    if not lp.is_file():
        return
    text = read_book_text(lp)
    head, blocks = split_chapter_summary_blocks(text)
    if not blocks:
        return
    ch_vol = _chapter_volume_map(book_root)
    last_vol = "v01"
    per_vol: dict[str, list[tuple[int, str]]] = {}
    for ch, block in blocks:
        vol = ch_vol.get(ch)
        if vol is None:
            vol = last_vol
            r.advisory("migrate", f"## ch{ch:03d} not covered by any unit outline — filed under {vol}")
        last_vol = vol
        per_vol.setdefault(vol, []).append((ch, block))
    for vol, items in sorted(per_vol.items()):
        path = book_root / summaries_rel(vol)
        existing = read_book_text(path) if path.is_file() else f"# Chapter summaries — {vol}\n\n"
        appended = 0
        for ch, block in sorted(items):
            if f"## ch{ch:03d}" in existing:
                continue
            existing = existing.rstrip("\n") + "\n\n" + block.rstrip("\n") + "\n"
            appended += 1
        write_book_text(path, existing)
        changes.append(f"{summaries_rel(vol)}: +{appended} chapter blocks")
    index_lines = ["## Chapter summaries", "", "章摘要按卷存放（Commit 直接写卷文件；本节只留索引）：", ""]
    for vol in sorted(per_vol):
        index_lines.append(f"- {vol} → {summaries_rel(vol)}")
    new_head = head.rstrip("\n")
    if re.search(r"^## Chapter summaries\s*$", new_head, re.M):
        new_head = re.sub(
            r"^## Chapter summaries\s*$",
            "\n".join(index_lines),
            new_head,
            count=1,
            flags=re.M,
        )
    else:
        new_head += "\n\n" + "\n".join(index_lines)
    write_book_text(lp, new_head + "\n")
    changes.append(f"{lp.name}: removed {len(blocks)} ## chNNN blocks, kept volume index")


def guess_genre(book_root: Path, st: dict) -> str:
    bible = book_root / "book-bible.md"
    text = read_book_text(bible) if bible.is_file() else ""
    for g in GENRES:
        if re.search(rf"(题材|类型|genre)[^\n]{{0,12}}{re.escape(g)}", text):
            return g
    if str(st.get("craft_lane") or "").strip().lower().replace("_", "-") == "crime-human":
        return "悬疑"
    qc = str(st.get("qc_profile") or "").strip()
    if qc == "mystery":
        return "悬疑"
    if qc == "female_emotion":
        return "古代言情" if re.search(r"古代|王朝|皇|侯|宫", text) else "现代言情"
    if re.search(r"系统|穿越|重生", text):
        return "系统穿越"
    if re.search(r"修仙|仙侠|道友|飞升", text):
        return "仙侠"
    if re.search(r"修炼|境界|灵气|玄幻", text):
        return "玄幻"
    if re.search(r"仕途|官场|扫黑|纪委", text):
        return "仕途扫黑"
    return "都市"


def migrate_state(book_root: Path, st: dict, r: Report, changes: list[str]) -> None:
    sp = book_root / "novel-state.yaml"
    if not sp.is_file():
        return
    if str(st.get("genre") or "").strip():
        return
    genre = guess_genre(book_root, st)
    text = read_book_text(sp)
    text = set_state_scalar(text, "genre", genre)
    blockers = st.get("blockers")
    note = f"确认 genre（migrate 猜为 {genre}）"
    if isinstance(blockers, list) and not blockers:
        text = re.sub(r"^blockers:\s*\[\]\s*$", f"blockers:\n  - \"{note}\"", text, count=1, flags=re.M)
    elif re.search(r"^blockers:\s*$", text, re.M):
        text = re.sub(r"^blockers:\s*$", f"blockers:\n  - \"{note}\"", text, count=1, flags=re.M)
    else:
        text = text.rstrip("\n") + f"\nblockers:\n  - \"{note}\"\n"
    write_book_text(sp, text)
    st["genre"] = genre
    changes.append(f"novel-state.yaml: genre={genre} (guessed; blocker added)")
    r.advisory("migrate", f"genre guessed as {genre} — confirm and clear the blocker")


def migrate_units(book_root: Path, r: Report, changes: list[str]) -> dict[str, list[str]]:
    """Returns volume → union of on_stage after migration."""
    per_vol: dict[str, list[str]] = {}
    for uid in list_unit_ids(book_root, "outline"):
        path = book_root / unit_outline_rel(uid)
        try:
            text = read_book_text(path)
        except (OSError, UnicodeDecodeError):
            continue
        u = load_yaml_map(text)
        vol = uid.split("-")[0]
        bucket = per_vol.setdefault(vol, [])
        touched = False
        if "on_stage" not in u:
            who = state_delta_who(nonempty_list(u.get("state_deltas")))
            stems = [w for w in who if re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]*", w)]
            flow = "[" + ", ".join(stems) + "]"
            insert = f"on_stage: {flow}   # migrate 从 state_deltas 预填；核对后删本注释\n"
            if "pov" not in u:
                insert += 'pov: ""           # migrate 留空；写作前填 POV stem\n'
                r.advisory("migrate", f"{unit_outline_rel(uid)}: pov left empty")
            text = _insert_after_key(text, "status", insert)
            touched = True
            for s in stems:
                if s not in bucket:
                    bucket.append(s)
        else:
            for s in on_stage_of(u):
                if s not in bucket:
                    bucket.append(s)
            if "pov" not in u:
                text = _insert_after_key(text, "on_stage", 'pov: ""           # migrate 留空；写作前填 POV stem\n')
                touched = True
                r.advisory("migrate", f"{unit_outline_rel(uid)}: pov left empty")
        if touched:
            write_book_text(path, text)
            changes.append(f"{unit_outline_rel(uid)}: on_stage/pov added")
    return per_vol


def _insert_after_key(text: str, key: str, insert: str) -> str:
    m = re.search(rf"^{re.escape(key)}\s*:.*$", text, re.M)
    if not m:
        return text.rstrip("\n") + "\n" + insert
    end = m.end()
    return text[:end] + "\n" + insert.rstrip("\n") + text[end:]


def migrate_cast(book_root: Path, r: Report, changes: list[str]) -> None:
    for path in cast_paths(book_root):
        text = read_book_text(path)
        card = parse_cast_card(text, path.stem, path)
        if card.role:
            continue
        role = "protagonist" if re.search(r"角色\s*[：:]\s*protagonist|主角卡|role\s*[：:]\s*protagonist", text) else "recurring"
        lines = text.splitlines()
        idx = None
        for i, line in enumerate(lines[:12]):
            if CAST_KV_RE.match(line):
                idx = i
        line = f"`role`: {role}  # protagonist | volume_antagonist | recurring（migrate 推断，请核对）"
        if idx is not None:
            lines.insert(idx + 1, line)
        else:
            lines.insert(1, "")
            lines.insert(2, "`status`: canon" if card.status == "" else f"`status`: {card.status}")
            lines.insert(3, line)
        write_book_text(path, "\n".join(lines) + "\n")
        changes.append(f"canon/cast/{path.name}: role={role}")
        if role == "recurring":
            r.advisory("migrate", f"canon/cast/{path.name}: role guessed recurring — confirm")


def migrate_volumes(book_root: Path, per_vol: dict[str, list[str]], changes: list[str]) -> None:
    vol_dir = book_root / "outline" / "volumes"
    if not vol_dir.is_dir():
        return
    for path in sorted(vol_dir.glob("v*.md")):
        text = read_book_text(path)
        if "本卷人物" in text and parse_volume_cast(text):
            continue
        stems = per_vol.get(path.stem, [])
        section = ["", "## 本卷人物（stem；批准即 canon）", ""]
        section += [f"- {s}" for s in stems] if stems else ["- "]
        text = text.rstrip("\n") + "\n" + "\n".join(section) + "\n"
        write_book_text(path, text)
        changes.append(f"{volume_outline_rel(path.stem)}: 本卷人物 added ({len(stems)} stems)")


def migrate_lane(book_root: Path, st: dict, r: Report, changes: list[str]) -> None:
    """Drop legacy craft_lane. crime-human on a 悬疑 (or unset) book becomes subgenre 刑侦探案."""
    sp = book_root / "novel-state.yaml"
    if not sp.is_file() or "craft_lane" not in st:
        return
    text = read_book_text(sp)
    lane = str(st.get("craft_lane") or "").strip().lower().replace("_", "-")
    if lane == "crime-human":
        genre = str(st.get("genre") or "").strip()
        if not genre or genre == "悬疑":
            if not genre:
                text = set_state_scalar(text, "genre", "悬疑")
                st["genre"] = "悬疑"
                changes.append("novel-state.yaml: genre=悬疑 (from craft_lane)")
            if not str(st.get("subgenre") or "").strip():
                text = set_state_scalar(text, "subgenre", "刑侦探案")
                st["subgenre"] = "刑侦探案"
                changes.append("novel-state.yaml: subgenre=刑侦探案 (from craft_lane)")
        else:
            r.advisory("migrate", f"craft_lane dropped; genre={genre} left unchanged")
    text2 = re.sub(r"^craft_lane\s*:.*\n", "", text, count=1, flags=re.M)
    if text2 == text:
        return
    write_book_text(sp, text2)
    st.pop("craft_lane", None)
    changes.append("novel-state.yaml: removed craft_lane")


def run_migrate(book_root: Path, st: dict, r: Report) -> list[str]:
    changes: list[str] = []
    migrate_summaries(book_root, r, changes)
    migrate_state(book_root, st, r, changes)
    migrate_lane(book_root, st, r, changes)
    per_vol = migrate_units(book_root, r, changes)
    migrate_cast(book_root, r, changes)
    migrate_volumes(book_root, per_vol, changes)
    if changes:
        stamp = _dt.date.today().isoformat()
        log = book_root / "continuity" / "commits" / f"migrate-{stamp}.md"
        body = [f"# migrate {stamp}", "", "结构迁移（novel_gate --action migrate）改动清单：", ""]
        body += [f"- {c}" for c in changes]
        body += ["", "警告见 gate 输出 ADVISORY。人工核对：genre、role、on_stage、pov、本卷人物。", ""]
        existing = read_book_text(log) if log.is_file() else ""
        write_book_text(log, (existing.rstrip("\n") + "\n\n" if existing else "") + "\n".join(body))
        changes.append(f"continuity/commits/{log.name}: written")
    return changes

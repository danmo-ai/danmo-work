"""doctor: layout, UTF-8, state fields, orphan outlines/prose, legacy structure hints."""
from __future__ import annotations

from pathlib import Path

from .common import (
    Report,
    _as_int,
    file_exists,
    is_utf8_file,
    iter_book_text_files,
    read_book_text,
    writing_stage,
)
from .context import validate_state_fields
from .ledger import has_reader_continuity, ledger_path, split_chapter_summary_blocks
from .outline import (
    legacy_chapters_message,
    list_unit_ids,
    load_unit,
    unit_outline_rel,
    unit_prose_rel,
)


def legacy_structure_notes(book_root: Path, st: dict) -> list[str]:
    """Reasons the book needs `--action migrate` (empty when current)."""
    notes: list[str] = []
    lp = ledger_path(book_root)
    if lp.is_file():
        _, blocks = split_chapter_summary_blocks(read_book_text(lp))
        if blocks:
            notes.append(f"{lp.name} still holds {len(blocks)} ## chNNN blocks — move to continuity/summaries/vNN.md")
    if not str(st.get("genre") or "").strip() and writing_stage(str(st.get("stage") or "")):
        notes.append("novel-state.yaml has no genre")
    for uid in list_unit_ids(book_root, "outline"):
        try:
            u, _ = load_unit(book_root, uid)
        except (OSError, UnicodeDecodeError):
            continue
        if u.get("scenes") and "on_stage" not in u:
            notes.append(f"{unit_outline_rel(uid)} has scenes but no on_stage")
            break
    cast_dir = book_root / "canon" / "cast"
    if cast_dir.is_dir():
        for path in sorted(cast_dir.glob("*.md")):
            text = read_book_text(path)
            if "`role`" not in text and "role:" not in text.split("\n## ", 1)[0]:
                notes.append(f"canon/cast/{path.name} has no `role` line")
                break
    return notes


def check_doctor(book_root: Path, st: dict, r: Report) -> None:
    for rel in ("novel-state.yaml", "book-bible.md", "canon/world.md"):
        if not file_exists(book_root, rel):
            r.blocking("layout", "missing " + rel)
    for d in ("canon", "canon/cast", "outline", "outline/volumes", "outline/units", "units", "continuity", "reviews"):
        if not file_exists(book_root, d):
            r.blocking("layout", "missing directory " + d + "/")
    validate_state_fields(st, r)
    legacy = legacy_chapters_message(book_root)
    if legacy:
        r.blocking("migrate", legacy)
    for path in iter_book_text_files(book_root, skip_archive=True):
        if not is_utf8_file(path):
            try:
                rel = str(path.relative_to(book_root))
            except ValueError:
                rel = str(path)
            r.blocking(
                "encoding",
                "non-UTF-8 text: " + rel + " — run python3 scripts/migrate_novel_encoding.py",
            )
            break
    if not file_exists(book_root, "canon/author-lore.md"):
        if writing_stage(str(st.get("stage") or "")):
            r.blocking("lore-tracks", "missing canon/author-lore.md (required from outline/writing onward)")
        else:
            r.advisory("lore-tracks", "missing canon/author-lore.md — seed at setup")
    if not has_reader_continuity(book_root):
        if writing_stage(str(st.get("stage") or "")):
            r.blocking(
                "lore-tracks",
                "missing continuity/facts.md (or legacy ledger.md / public-lore+tracking)",
            )
        else:
            r.advisory(
                "lore-tracks",
                "missing continuity/facts.md — seed at setup (legacy ledger.md also accepted)",
            )
    outlines = set(list_unit_ids(book_root, "outline"))
    proses = set(list_unit_ids(book_root, "prose"))
    for uid in sorted(outlines):
        try:
            u, rel = load_unit(book_root, uid)
        except (OSError, UnicodeDecodeError) as e:
            r.blocking("orphan-outline", f"{unit_outline_rel(uid)}: {e}")
            continue
        if str(u.get("status") or "").strip() in ("drafted", "reviewed") and uid not in proses:
            r.blocking("orphan-outline", f"{rel} status={u.get('status')} but {unit_prose_rel(uid)} missing")
    for uid in sorted(proses - outlines):
        r.blocking("orphan-prose", f"{unit_prose_rel(uid)} has no {unit_outline_rel(uid)}")
    last = _as_int(st.get("last_committed_ch"))
    if last > 0:
        found = False
        units_dir = book_root / "units"
        if units_dir.is_dir():
            for path in units_dir.glob("*.md"):
                try:
                    body = read_book_text(path)
                except OSError:
                    continue
                if f"## 第{last}章" in body:
                    found = True
                    break
        if not found:
            r.blocking("state", f"last_committed_ch={last} but no units/*.md contains ## 第{last}章")
    for note in legacy_structure_notes(book_root, st):
        r.advisory("migrate", note + " — run --action migrate")

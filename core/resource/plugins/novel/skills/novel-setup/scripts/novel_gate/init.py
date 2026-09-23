"""init: build the book tree once and copy templates, so the model only fills the bible,
`genre`, `qc_profile`, `craft_lane` (plan §7)."""
from __future__ import annotations

import re
from pathlib import Path

from .common import Report, read_book_text, set_state_scalar, write_book_text
from .context import GENRES, plugin_root

BOOK_DIRS = (
    "canon",
    "canon/cast",
    "outline",
    "outline/volumes",
    "outline/units",
    "units",
    "continuity",
    "continuity/summaries",
    "continuity/commits",
    "reviews",
)

# (template rel under plugin skills/, destination rel under the book root)
TEMPLATE_COPIES = (
    ("novel-setup/assets/templates/novel-state.yaml", "novel-state.yaml"),
    ("novel-setup/assets/templates/book-bible.md", "book-bible.md"),
    ("novel-setup/assets/templates/world.md", "canon/world.md"),
    ("novel-setup/assets/templates/author-lore.md", "canon/author-lore.md"),
    ("novel-setup/assets/templates/locked-terms.yaml", "canon/locked-terms.yaml"),
    ("novel-setup/assets/templates/facts.md", "continuity/facts.md"),
    ("novel-write/assets/templates/style-fingerprint.md", "canon/style-fingerprint.md"),
    ("novel-plan/assets/templates/book-outline.md", "outline/book-outline.md"),
)


def _fill(text: str, book_id: str, title: str) -> str:
    text = text.replace("{{title}}", title or book_id).replace("{{book_id}}", book_id)
    return text


def init_book(workdir: str, book_id: str, r: Report, title: str = "", genre: str = "") -> dict:
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]*", book_id or ""):
        r.blocking("init", f"--book-id {book_id!r} must be a lowercase slug")
        return {}
    if genre and genre not in GENRES:
        r.blocking("init", f"--genre {genre!r} not in {list(GENRES)}")
        return {}
    root = Path(workdir).resolve() / "novel" / book_id
    created_dirs: list[str] = []
    for d in BOOK_DIRS:
        p = root / d
        if not p.exists():
            p.mkdir(parents=True, exist_ok=True)
            created_dirs.append(d)
    skills = plugin_root() / "skills"
    copied: list[str] = []
    kept: list[str] = []
    for src_rel, dst_rel in TEMPLATE_COPIES:
        src = skills / src_rel
        dst = root / dst_rel
        if dst.exists():
            kept.append(dst_rel)
            continue
        if not src.is_file():
            r.advisory("init", f"template missing in plugin: skills/{src_rel}")
            continue
        text = _fill(read_book_text(src), book_id, title)
        if dst_rel == "novel-state.yaml":
            text = set_state_scalar(text, "book_id", f'"{book_id}"')
            text = set_state_scalar(text, "title", f'"{title}"' if title else '""')
            text = set_state_scalar(text, "stage", "setup")
            text = set_state_scalar(text, "next_action", '"填 book-bible.md 读者承诺；定 genre / qc_profile / craft_lane；跑 novel-plan"')
            if genre:
                text = set_state_scalar(text, "genre", genre)
        write_book_text(dst, text)
        copied.append(dst_rel)
    (root / "continuity/commits/.keep").touch(exist_ok=True)
    (root / "reviews/.keep").touch(exist_ok=True)
    return {"root": str(root), "dirs": created_dirs, "copied": copied, "kept": kept}

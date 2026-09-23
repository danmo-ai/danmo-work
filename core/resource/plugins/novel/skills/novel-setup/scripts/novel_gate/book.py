"""Per-invocation read cache shared by the checks."""
from __future__ import annotations

from pathlib import Path

from .cast import CastCard, cast_paths, parse_cast_card
from .common import read_book_text
from .context import style_fingerprint_brief
from .ledger import ledger_path, summary_source_text


class BookCache:
    """Per-process book reads shared across one gate invocation."""

    def __init__(self, book_root: Path):
        self.book_root = book_root
        self._ledger: str | None = None
        self._open_loops: str | None = None
        self._outline_texts: list[tuple[Path, str]] | None = None
        self._cast_files: list[Path] | None = None
        self._cast_texts: dict[str, str] | None = None
        self._cast_cards: dict[str, CastCard] | None = None
        self._style: str | None = None
        self._style_loaded = False
        self._summary: tuple[str, str] | None = None

    def ledger_text(self) -> str:
        if self._ledger is None:
            lp = ledger_path(self.book_root)
            self._ledger = read_book_text(lp) if lp.is_file() else ""
        return self._ledger

    def open_loops_text(self) -> str:
        if self._open_loops is None:
            if self.ledger_text():
                self._open_loops = self.ledger_text()
            else:
                tracker = self.book_root / "continuity/foreshadow-tracker.md"
                if tracker.is_file():
                    self._open_loops = read_book_text(tracker)
                else:
                    tracking = self.book_root / "continuity/tracking.md"
                    self._open_loops = read_book_text(tracking) if tracking.is_file() else ""
        return self._open_loops

    def outline_texts(self) -> list[tuple[Path, str]]:
        if self._outline_texts is None:
            outline = self.book_root / "outline"
            out: list[tuple[Path, str]] = []
            if outline.is_dir():
                for path in sorted(outline.rglob("*.md")):
                    out.append((path, read_book_text(path)))
            self._outline_texts = out
        return self._outline_texts

    def cast_paths(self) -> list[Path]:
        if self._cast_files is None:
            self._cast_files = cast_paths(self.book_root)
        return self._cast_files

    def cast_text_map(self) -> dict[str, str]:
        """path.as_posix() → text; load once for anchors + relations."""
        if self._cast_texts is None:
            self._cast_texts = {}
            for path in self.cast_paths():
                self._cast_texts[path.as_posix()] = read_book_text(path)
        return self._cast_texts

    def cast_cards(self) -> dict[str, CastCard]:
        if self._cast_cards is None:
            texts = self.cast_text_map()
            self._cast_cards = {
                path.stem: parse_cast_card(texts[path.as_posix()], path.stem, path) for path in self.cast_paths()
            }
        return self._cast_cards

    def style_brief(self) -> str:
        if not self._style_loaded:
            self._style = style_fingerprint_brief(self.book_root)
            self._style_loaded = True
        return self._style or ""

    def summary_source(self) -> tuple[str, str]:
        if self._summary is None:
            self._summary = summary_source_text(self.book_root)
        return self._summary

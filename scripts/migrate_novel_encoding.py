#!/usr/bin/env python3
"""One-shot: convert novel book text files under WORK_DATA_DIR from GB18030 (or UTF-8 BOM) to UTF-8.

Usage:
  python3 scripts/migrate_novel_encoding.py --dry-run
  python3 scripts/migrate_novel_encoding.py
  python3 scripts/migrate_novel_encoding.py --root /path/to/data --backup
  python3 scripts/migrate_novel_encoding.py --skip-archive

Root defaults to $WORK_DATA_DIR or ~/.danmo-work/data.
Scans paths containing /novel/ with suffixes .md .yaml .yml .txt .json.
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

TEXT_SUFFIXES = {".md", ".yaml", ".yml", ".txt", ".json"}


def default_root() -> Path:
    return Path(os.environ.get("WORK_DATA_DIR") or Path.home() / ".danmo-work" / "data")


def is_novel_text(path: Path, root: Path) -> bool:
    try:
        rel = path.relative_to(root)
    except ValueError:
        return False
    parts = rel.parts
    if "novel" not in parts:
        return False
    return path.suffix.lower() in TEXT_SUFFIXES


def decode_legacy(raw: bytes) -> tuple[str, str] | None:
    """Return (text, source_encoding) or None if undecodable."""
    try:
        return raw.decode("utf-8"), "utf-8"
    except UnicodeDecodeError:
        pass
    for enc in ("utf-8-sig", "gb18030"):
        try:
            return raw.decode(enc), enc
        except UnicodeDecodeError:
            continue
    return None


def convert_file(path: Path, *, dry_run: bool, backup: bool) -> str:
    """Return status: skipped | converted | failed."""
    try:
        raw = path.read_bytes()
    except OSError as e:
        print(f"failed (read): {path}: {e}", file=sys.stderr)
        return "failed"
    if not raw:
        return "skipped"
    decoded = decode_legacy(raw)
    if decoded is None:
        print(f"failed (decode): {path}", file=sys.stderr)
        return "failed"
    text, enc = decoded
    if enc == "utf-8":
        return "skipped"
    # Normalize newlines to LF when rewriting (charset fix is the goal).
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    if dry_run:
        print(f"would convert ({enc} → utf-8): {path}")
        return "converted"
    try:
        if backup:
            bak = path.with_suffix(path.suffix + f".{enc}.bak")
            if not bak.exists():
                bak.write_bytes(raw)
        path.write_text(text, encoding="utf-8")
    except OSError as e:
        print(f"failed (write): {path}: {e}", file=sys.stderr)
        return "failed"
    print(f"converted ({enc} → utf-8): {path}")
    return "converted"


def iter_targets(root: Path, skip_archive: bool) -> list[Path]:
    out: list[Path] = []
    for p in root.rglob("*"):
        if not p.is_file():
            continue
        if not is_novel_text(p, root):
            continue
        if skip_archive and "_archive" in p.parts:
            continue
        out.append(p)
    return sorted(out)


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="Convert novel text files to UTF-8")
    ap.add_argument("--root", type=Path, default=None, help="data root (default WORK_DATA_DIR)")
    ap.add_argument("--dry-run", action="store_true", help="report only; do not write")
    ap.add_argument("--backup", action="store_true", help="write .<enc>.bak beside originals")
    ap.add_argument("--skip-archive", action="store_true", help="skip paths under _archive/")
    args = ap.parse_args(argv)

    root = (args.root or default_root()).expanduser().resolve()
    if not root.is_dir():
        print(f"no data dir: {root}", file=sys.stderr)
        return 1

    counts = {"converted": 0, "skipped": 0, "failed": 0}
    for path in iter_targets(root, args.skip_archive):
        status = convert_file(path, dry_run=args.dry_run, backup=args.backup)
        counts[status] += 1

    print(
        f"done. converted={counts['converted']} skipped={counts['skipped']} failed={counts['failed']}"
        + (" (dry-run)" if args.dry_run else "")
    )
    return 1 if counts["failed"] else 0


if __name__ == "__main__":
    raise SystemExit(main())

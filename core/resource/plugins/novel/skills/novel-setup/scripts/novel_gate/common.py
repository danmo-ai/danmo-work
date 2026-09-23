"""Shared plumbing: minimal YAML reader, strict UTF-8 IO, state resolution, Report."""
from __future__ import annotations

import re
import sys
from pathlib import Path

UNIT_ID_RE = re.compile(r"^v\d+-U\d+$")
VOLUME_ID_RE = re.compile(r"^v\d+$")

MIGRATE_HINT = (
    "non-UTF-8 text file — run: python3 scripts/migrate_novel_encoding.py "
    "(from DanQing-Teams repo; or set WORK_DATA_DIR)"
)
BOOK_TEXT_SUFFIXES = {".md", ".yaml", ".yml", ".txt", ".json"}


# --- YAML (PyYAML optional; indentation fallback covers the shapes we emit) ---


def _unquote(s: str) -> str:
    s = s.strip()
    if len(s) >= 2 and s[0] == s[-1] and s[0] in "\"'":
        return s[1:-1]
    return s


def _parse_flow_list(raw: str) -> list[str]:
    raw = raw.strip()
    if raw in ("[]", ""):
        return []
    if raw.startswith("[") and raw.endswith("]"):
        inner = raw[1:-1].strip()
        if not inner:
            return []
        parts, buf, q = [], "", None
        for ch in inner:
            if q:
                buf += ch
                if ch == q:
                    q = None
                continue
            if ch in "\"'":
                q = ch
                buf += ch
                continue
            if ch == ",":
                parts.append(_unquote(buf))
                buf = ""
                continue
            buf += ch
        if buf.strip():
            parts.append(_unquote(buf))
        return [p for p in parts if p]
    return [_unquote(raw)] if raw else []


def _strip_inline_comment(raw: str) -> str:
    """Drop a # comment that is not inside quotes. Block scalars are not passed here."""
    quote = None
    for idx, ch in enumerate(raw):
        if quote:
            if ch == quote:
                quote = None
            continue
        if ch in "\"'":
            quote = ch
            continue
        if ch == "#" and (idx == 0 or raw[idx - 1].isspace()):
            return raw[:idx]
    return raw


def _peek_yaml_line(lines: list[str], start: int) -> tuple[int, str] | tuple[None, None]:
    j = start
    while j < len(lines):
        raw = _strip_inline_comment(lines[j])
        if raw.strip():
            return len(raw) - len(raw.lstrip(" ")), raw.strip()
        j += 1
    return None, None


def _read_block_scalar(lines: list[str], start: int, parent_indent: int) -> tuple[str, int]:
    buf: list[str] = []
    i = start
    while i < len(lines):
        raw = lines[i]
        if raw.strip():
            ind = len(raw) - len(raw.lstrip(" "))
            if ind <= parent_indent:
                break
        buf.append(raw)
        i += 1
    filled = [row for row in buf if row.strip()]
    cut = min((len(row) - len(row.lstrip(" ")) for row in filled), default=0)
    text = "\n".join(row[cut:] if len(row) >= cut else row.lstrip(" ") for row in buf)
    return text.strip("\n"), i


def _is_yaml_map_item(item: str) -> bool:
    if not item or item[0] in "\"'[{":
        return False
    key, sep, _rest = item.partition(":")
    if not sep:
        return False
    key = key.strip()
    return bool(key) and " " not in key


def _load_yaml_map_fallback(text: str) -> dict:
    """Indentation parser for unit outlines when PyYAML is absent.

    Covers the shape we emit: nested maps, lists of maps, nested lists,
    flow lists (including quoted colons), and | / > block scalars.
    Not a general YAML parser.
    """
    root: dict = {}
    # (indent of keys that belong in this map, map)
    stack: list[tuple[int, dict]] = [(-1, root)]
    # (indent of the "-" lines, parent map, key)
    lists: list[tuple[int, dict, str]] = []
    lines = text.splitlines()
    i = 0

    def pop_to(indent: int) -> None:
        while len(stack) > 1 and indent < stack[-1][0]:
            stack.pop()
        while lists and indent < lists[-1][0]:
            lists.pop()

    while i < len(lines):
        raw = _strip_inline_comment(lines[i])
        i += 1
        if not raw.strip():
            continue
        indent = len(raw) - len(raw.lstrip(" "))
        line = raw.strip()
        pop_to(indent)
        cur = stack[-1][1]

        if line.startswith("- "):
            if not lists or indent != lists[-1][0]:
                continue
            _item_indent, parent, key = lists[-1]
            bucket = parent.get(key)
            if not isinstance(bucket, list):
                bucket = []
                parent[key] = bucket
            item = line[2:].strip()
            if _is_yaml_map_item(item):
                k, _, v = item.partition(":")
                k, v = k.strip(), v.strip()
                entry: dict = {}
                bucket.append(entry)
                if v in ("|", ">", "|-", ">-", "|+", ">+"):
                    body, i = _read_block_scalar(lines, i, indent)
                    entry[k] = body
                elif v == "":
                    entry[k] = {}
                else:
                    entry[k] = _yaml_scalar(v)
                stack.append((indent + 2, entry))
            else:
                bucket.append(_yaml_scalar(item))
            continue

        if ":" not in line:
            continue
        key, _, rest = line.partition(":")
        key, rest = key.strip(), rest.strip()
        if not key:
            continue
        if rest == "":
            nxt_indent, nxt_line = _peek_yaml_line(lines, i)
            if nxt_line and nxt_line.startswith("- ") and nxt_indent is not None and nxt_indent > indent:
                cur[key] = []
                lists.append((nxt_indent, cur, key))
                continue
            child = {}
            cur[key] = child
            stack.append((indent + 2, child))
            continue
        if rest.startswith("[") and rest.endswith("]"):
            cur[key] = _parse_flow_list(rest)
        elif rest in ("|", ">", "|-", ">-", "|+", ">+"):
            body, i = _read_block_scalar(lines, i, indent)
            cur[key] = body
        else:
            cur[key] = _yaml_scalar(rest)
    return root


def load_yaml_map(text: str) -> dict:
    try:
        import yaml  # type: ignore

        data = yaml.safe_load(text)
        return data if isinstance(data, dict) else {}
    except Exception:
        return _load_yaml_map_fallback(text)


def _yaml_scalar(v: str):
    v = _unquote(v)
    if re.fullmatch(r"-?\d+", v):
        return int(v)
    return v


# --- small helpers ---


def _as_int(v, default: int = 0) -> int:
    try:
        return int(v)
    except (TypeError, ValueError):
        return default


def is_blank(s) -> bool:
    return s is None or str(s).strip() == ""


def nonempty_list(v) -> list[str]:
    if not isinstance(v, list):
        return []
    return [str(x).strip() for x in v if str(x).strip()]


def writing_stage(stage: str) -> bool:
    return str(stage).strip() in {"outline", "writing", "review"}


def tomato_profile(profile: str) -> bool:
    return str(profile).strip() in {"male_power", "female_emotion"}


def last_runes(s: str, n: int) -> str:
    if n <= 0:
        return ""
    return s if len(s) <= n else s[-n:]


def rune_count(s: str) -> int:
    return sum(1 for r in s if r not in " \n\t\r")


def volume_of(unit_id: str) -> str:
    """'v01-U2' → 'v01'."""
    uid = (unit_id or "").strip()
    return uid.split("-")[0] if "-" in uid else ""


def volume_number(tag: str) -> int:
    try:
        return int(re.sub(r"\D", "", str(tag or "")))
    except ValueError:
        return 0


# --- IO ---


def _force_stdio_utf8() -> None:
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8")  # type: ignore[attr-defined]
        except Exception:
            pass


def read_book_text(path: Path) -> str:
    """Strict UTF-8 read. Raises UnicodeDecodeError with migrate hint."""
    try:
        return path.read_text(encoding="utf-8")
    except UnicodeDecodeError as e:
        raise UnicodeDecodeError(
            e.encoding,
            e.object,
            e.start,
            e.end,
            f"{path}: {MIGRATE_HINT}",
        ) from None


def write_book_text(path: Path, text: str) -> None:
    """Always persist UTF-8 (no BOM)."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def read_text(root: Path, rel: str) -> str:
    return read_book_text(root / rel)


def file_exists(root: Path, rel: str) -> bool:
    return (root / rel).exists()


def is_utf8_file(path: Path) -> bool:
    try:
        path.read_bytes().decode("utf-8")
        return True
    except UnicodeDecodeError:
        return False


def iter_book_text_files(book_root: Path, *, skip_archive: bool = True) -> list[Path]:
    out: list[Path] = []
    if not book_root.is_dir():
        return out
    for path in book_root.rglob("*"):
        if not path.is_file():
            continue
        if path.suffix.lower() not in BOOK_TEXT_SUFFIXES:
            continue
        if skip_archive and "_archive" in path.parts:
            continue
        out.append(path)
    return sorted(out)


# --- state / book resolution ---


def load_state(path: Path) -> dict:
    st = load_yaml_map(read_book_text(path))
    if not st.get("book_id"):
        st["book_id"] = path.parent.name
    return st


def resolve_book(workdir: str, book_id: str) -> tuple[Path, dict]:
    work = Path(workdir).resolve()
    if not workdir:
        raise SystemExit("workdir is required")
    state_path = work / "novel-state.yaml"
    if state_path.is_file():
        st = load_state(state_path)
        if not book_id or not st.get("book_id") or st.get("book_id") == book_id:
            return work, st
    novel_dir = work / "novel"
    if book_id:
        root = novel_dir / book_id
        return root, load_state(root / "novel-state.yaml")
    if not novel_dir.is_dir():
        raise FileNotFoundError(f"no novel-state.yaml in workdir and cannot list novel/: {work}")
    found: list[tuple[Path, dict]] = []
    for child in novel_dir.iterdir():
        if not child.is_dir():
            continue
        sp = child / "novel-state.yaml"
        if not sp.is_file():
            continue
        found.append((child, load_state(sp)))
    if not found:
        raise FileNotFoundError(f"no novel/<book-id>/novel-state.yaml under {work}")
    if len(found) > 1:
        ids = [st.get("book_id") or p.name for p, st in found]
        raise FileNotFoundError(f"multiple books {ids}; pass --book-id")
    return found[0]


def set_state_scalar(text: str, key: str, value: str) -> str:
    """Replace a top-level `key: value` line in novel-state.yaml text (keeps trailing comment).
    Appends the line when the key is absent."""
    pat = re.compile(rf"^({re.escape(key)}\s*:\s*)([^#\n]*?)(\s*#.*)?$", re.M)
    m = pat.search(text)
    if not m:
        return text.rstrip("\n") + f"\n{key}: {value}\n"
    comment = m.group(3) or ""
    return text[: m.start()] + f"{m.group(1)}{value}{comment}" + text[m.end():]


def set_state_nested_scalar(text: str, parent: str, key: str, value: str) -> str:
    """Replace `parent:\\n  key: value` (2-space nested) in novel-state.yaml text."""
    pat = re.compile(
        rf"^({re.escape(parent)}\s*:[^\n]*\n(?:[ \t]+[^\n]*\n)*?)([ \t]+{re.escape(key)}\s*:\s*)([^#\n]*?)(\s*#.*)?$",
        re.M,
    )
    m = pat.search(text)
    if not m:
        return text
    comment = m.group(4) or ""
    return text[: m.start(3)] + value + comment + text[m.end():]


# --- report ---


class Report:
    def __init__(self, action: str, book_id: str, book_root: Path, chapter: int = 0, unit: str = ""):
        self.action = action
        self.book_id = book_id
        self.book_root = str(book_root)
        self.chapter = chapter
        self.unit = unit
        self.verdict = "PASS"
        self.findings: list[dict] = []
        self.context_lines: list[str] = []
        self.counts: dict[str, int] = {}
        self.chapter_reports: list[Report] = []
        self.range_from = 0
        self.range_to = 0
        self.extra_sections: list[tuple[str, list[str]]] = []

    def add_counts(self, **kw: int) -> None:
        for k, v in kw.items():
            self.counts[k] = self.counts.get(k, 0) + int(v)

    def blocking(self, check: str, msg: str) -> None:
        self.findings.append({"severity": "blocking", "check": check, "message": msg})

    def advisory(self, check: str, msg: str) -> None:
        self.findings.append({"severity": "advisory", "check": check, "message": msg})

    def section(self, title: str, lines: list[str]) -> None:
        """Extra printable block placed after CONTEXT (e.g. HITS, UNITS, CHANGES)."""
        self.extra_sections.append((title, list(lines)))

    def finalize(self) -> None:
        if self.chapter_reports:
            for cr in self.chapter_reports:
                cr.finalize()
            failed = [cr.chapter for cr in self.chapter_reports if cr.verdict == "FAIL"]
            if failed:
                self.verdict = "FAIL"
                self.blocking(
                    "batch",
                    "failed chapters: " + ", ".join(str(c) for c in failed),
                )
            else:
                self.verdict = "PASS"
            return
        self.verdict = "FAIL" if any(f["severity"] == "blocking" for f in self.findings) else "PASS"

    def _format_body(self) -> list[str]:
        lines: list[str] = []
        if self.context_lines:
            lines += ["", "### CONTEXT"]
            lines.extend(self.context_lines)
        for title, body in self.extra_sections:
            lines += ["", f"### {title}"]
            lines.extend(body if body else ["None."])
        if self.counts:
            lines += ["", "### COUNTS"]
            for k in ("em_dash_count", "ai_vocab_count", "english_leak_count", "simile_count"):
                if k in self.counts:
                    lines.append(f"{k}: {self.counts[k]}")
            for k in sorted(self.counts):
                if k not in ("em_dash_count", "ai_vocab_count", "english_leak_count", "simile_count"):
                    lines.append(f"{k}: {self.counts[k]}")
        lines += ["", "### BLOCKING"]
        blocks = [f for f in self.findings if f["severity"] == "blocking"]
        if not blocks:
            lines.append("None.")
        else:
            for f in blocks:
                lines.append(f"- [{f['check']}] {f['message']}")
        lines += ["", "### ADVISORY"]
        adv = [f for f in self.findings if f["severity"] == "advisory"]
        if not adv:
            lines.append("None.")
        else:
            for f in adv:
                lines.append(f"- [{f['check']}] {f['message']}")
        return lines

    def format(self) -> str:
        lines = ["### VERDICT", self.verdict, "", "### ACTION", self.action]
        if self.book_id:
            lines += ["", "### BOOK", self.book_id]
        if self.unit:
            lines += ["", "### UNIT", self.unit]
        if self.chapter_reports:
            lines += ["", "### RANGE", f"{self.range_from}-{self.range_to}"]
            fail_note = [f for f in self.findings if f["check"] == "batch"]
            if fail_note:
                lines += ["", "### BLOCKING"]
                for f in fail_note:
                    lines.append(f"- [{f['check']}] {f['message']}")
            for cr in self.chapter_reports:
                lines += ["", f"### CHAPTER {cr.chapter}", "### VERDICT", cr.verdict]
                lines.extend(cr._format_body())
            return "\n".join(lines) + "\n"
        if self.chapter:
            lines += ["", "### CHAPTER", str(self.chapter)]
        lines.extend(self._format_body())
        return "\n".join(lines) + "\n"

    def as_dict(self) -> dict:
        d: dict = {
            "action": self.action,
            "book_id": self.book_id,
            "book_root": self.book_root,
            "chapter": self.chapter,
            "verdict": self.verdict,
            "findings": self.findings,
            "context": self.context_lines,
            "counts": self.counts,
        }
        if self.unit:
            d["unit"] = self.unit
        if self.extra_sections:
            d["sections"] = {title: body for title, body in self.extra_sections}
        if self.chapter_reports:
            d["range"] = {"from": self.range_from, "to": self.range_to}
            d["chapters"] = [cr.as_dict() for cr in self.chapter_reports]
        return d

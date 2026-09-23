"""Cast cards: parse canon/cast/*.md (stem = character id), anchors, dialogue sample,
knowledge boundary, relation table; cast-lint; candidate → canon promotion."""
from __future__ import annotations

import re
from dataclasses import dataclass, field
from pathlib import Path

from .common import Report, read_book_text, write_book_text

CAST_ANCHOR_RE = re.compile(r"^\s*[-*]\s*\*?\*?(视觉|语言|行为)\*?\*?[：:]\s*(.*)")
CAST_KV_RE = re.compile(r"^\s*`?(status|role)`?\s*[：:]\s*([A-Za-z_-]+)", re.I)
CAST_STATUSES = {"candidate", "canon"}
CAST_ROLES = {"protagonist", "volume_antagonist", "recurring"}
_UNKNOWN_RE = re.compile(r"^\s*[-*]\s*\*?\*?不知\*?\*?[：:]\s*(.*)")
_DIALOGUE_RE = re.compile(r"^\s*[-*]\s*\*?\*?(压力下|日常|掩饰(?:\s*/\s*说谎)?)\*?\*?[：:]\s*(.*)")
_CATCHPHRASE_RE = re.compile(r"^\s*[-*]\s*\*?\*?口头禅[^：:]*[：:]\s*(.*)")
_INTERSECT_RE = re.compile(r"^\s*[-*]\s*\*?\*?与主角相交点[^：:]*[：:]\s*(.*)")
_DESIRE_RE = re.compile(r"^\s*[-*]\s*\*?\*?欲望[^：:]*[：:]\s*(.*)")
_EXIT_RE = re.compile(r"^\s*[-*]\s*\*?\*?退场[^：:]*[：:]\s*(.*)")
_TEMPLATE_PLACEHOLDER = re.compile(r"^\{\{.*\}\}$")


@dataclass
class CastCard:
    stem: str
    path: Path
    title: str
    status: str
    role: str
    anchors: dict[str, str] = field(default_factory=dict)  # 视觉/语言/行为
    dialogue: dict[str, str] = field(default_factory=dict)  # 压力下/日常/掩饰
    unknown: str = ""
    catchphrase: str = ""
    desire: str = ""
    intersect: str = ""
    exit: str = ""
    relations: list[dict] = field(default_factory=list)
    text: str = ""

    @property
    def label(self) -> str:
        return self.title or self.stem

    def dialogue_sample(self) -> str:
        for key in ("压力下", "日常", "掩饰"):
            v = self.dialogue.get(key, "").strip()
            if v:
                return v
        return ""

    def missing_fields(self) -> list[str]:
        """Minimum fill per role (template 完整度 table); labels are human-facing."""
        miss: list[str] = []
        if self.role == "recurring":
            if not self.desire:
                miss.append("欲望")
            if not self.intersect:
                miss.append("与主角相交点")
            if not (self.anchors.get("视觉") or self.anchors.get("行为")):
                miss.append("视觉锚或行为锚")
            if not self.catchphrase:
                miss.append("口头禅")
            if not self.dialogue_sample():
                miss.append("台词样例")
        else:
            for k in ("视觉", "语言", "行为"):
                if not self.anchors.get(k):
                    miss.append(f"{k}锚")
            if not self.unknown:
                miss.append("知识边界·不知")
            for k in ("压力下", "日常", "掩饰"):
                if not self.dialogue.get(k):
                    miss.append(f"台词·{k}")
        if not self.exit:
            miss.append("退场")
        return miss


def _clean(v: str) -> str:
    v = v.strip()
    if _TEMPLATE_PLACEHOLDER.match(v):
        return ""
    return v


def parse_cast_card(text: str, stem: str = "", path: Path | None = None) -> CastCard:
    lines = text.splitlines()
    title = lines[0].lstrip("# ").strip() if lines else ""
    if _TEMPLATE_PLACEHOLDER.match(title):
        title = ""
    card = CastCard(stem=stem, path=path or Path(f"{stem}.md"), title=title, status="", role="", text=text)
    section = ""
    for line in lines[:12]:
        m = CAST_KV_RE.match(line)
        if m:
            key, val = m.group(1).lower(), m.group(2).strip().lower()
            if key == "status" and not card.status:
                card.status = val
            elif key == "role" and not card.role:
                card.role = val
    for line in lines:
        s = line.strip()
        if s.startswith("#"):
            section = s.lstrip("#").strip()
            continue
        m = CAST_ANCHOR_RE.match(line)
        if m and "三锚点" in section:
            card.anchors[m.group(1)] = _clean(m.group(2))
            continue
        if "台词" in section:
            m = _DIALOGUE_RE.match(line)
            if m:
                key = m.group(1)
                key = "掩饰" if key.startswith("掩饰") else key
                card.dialogue[key] = _clean(m.group(2))
                continue
        if "知识边界" in section:
            m = _UNKNOWN_RE.match(line)
            if m:
                card.unknown = _clean(m.group(1))
                continue
        if "语言习惯" in section:
            m = _CATCHPHRASE_RE.match(line)
            if m:
                card.catchphrase = _clean(m.group(1))
                continue
        if "功能" in section:
            m = _INTERSECT_RE.match(line)
            if m:
                card.intersect = _clean(m.group(1))
                continue
            m = _EXIT_RE.match(line)
            if m:
                card.exit = _clean(m.group(1))
                continue
        if "四件套" in section and not card.desire:
            m = _DESIRE_RE.match(line)
            if m:
                card.desire = _clean(m.group(1))
                continue
        if "关系" in section and s.startswith("|"):
            cells = [c.strip() for c in s.strip("|").split("|")]
            if not cells or cells[0] in ("对方",) or set(cells[0]) <= {"-", ":"}:
                continue
            other = cells[0].strip("`").strip()
            if not other:
                continue
            card.relations.append(
                {
                    "other": other,
                    "type": cells[1] if len(cells) > 1 else "",
                    "state": cells[2] if len(cells) > 2 else "",
                    "last_change": cells[3] if len(cells) > 3 else "",
                    "next": cells[4] if len(cells) > 4 else "",
                }
            )
    # fallback: template-less legacy cards may carry anchors anywhere
    if not card.anchors:
        for line in lines:
            m = CAST_ANCHOR_RE.match(line)
            if m and m.group(2).strip():
                card.anchors.setdefault(m.group(1), _clean(m.group(2)))
    return card


def cast_dir(book_root: Path) -> Path:
    return book_root / "canon" / "cast"


def cast_paths(book_root: Path) -> list[Path]:
    d = cast_dir(book_root)
    return sorted(d.glob("*.md")) if d.is_dir() else []


def load_cast_cards(book_root: Path, cache=None) -> dict[str, CastCard]:
    if cache is not None:
        return cache.cast_cards()
    out: dict[str, CastCard] = {}
    for path in cast_paths(book_root):
        out[path.stem] = parse_cast_card(read_book_text(path), path.stem, path)
    return out


def cast_status_map(book_root: Path, cache=None) -> dict[str, str]:
    return {stem: (c.status or "candidate") for stem, c in load_cast_cards(book_root, cache).items()}


def _match_cast_file(
    files: list[Path],
    name: str,
    text_map: dict[str, str] | None = None,
) -> tuple[Path, str] | tuple[None, None]:
    """Find the cast file for a character. Exact filename (stem) match only.
    Content matching is unsafe: another character's card may mention this name
    in its 关系 table, so a fuzzy fallback can inject the wrong card."""
    for path in files:
        if path.stem == name:
            text = text_map.get(path.as_posix()) if text_map is not None else read_book_text(path)
            assert text is not None
            return path, text
    return None, None


def load_cast_anchors(book_root: Path, names: list[str], cache=None) -> list[str]:
    if not names:
        return []
    cards = load_cast_cards(book_root, cache)
    out: list[str] = []
    for name in names:
        card = cards.get(name)
        if card is None:
            continue
        anchors = [f"{k}={v}" for k, v in card.anchors.items() if v]
        if anchors:
            out.append(f"{card.label}: " + "; ".join(anchors[:3]))
        else:
            out.append(f"{card.label}: (无三锚点行)")
    return out


def load_cast_relations(book_root: Path, names: list[str], cache=None) -> list[str]:
    """Relationship rows between on-scene characters only: from each named
    character's cast card 关系 table, keep rows whose 对方 is also on scene."""
    if len(names) < 2:
        return []
    cards = load_cast_cards(book_root, cache)
    out: list[str] = []
    for name in names:
        card = cards.get(name)
        if card is None:
            continue
        for row in card.relations:
            other = row["other"]
            if other == name or other == card.label:
                continue
            if other not in names and not any(other == c.label for w in names if (c := cards.get(w))):
                continue
            detail = " / ".join(c for c in (row["type"], row["state"]) if c)
            out.append(f"{card.label} → {other}: {detail}" if detail else f"{card.label} → {other}")
    return out


def cast_context_lines(
    book_root: Path, names: list[str], pov: str, snapshot_rows: list[str], cache=None
) -> list[str]:
    """CONTEXT block for on_stage characters: snapshot row + 三锚点 + 1 台词 (+ POV 不知)."""
    lines: list[str] = []
    cards = load_cast_cards(book_root, cache)
    for name in names:
        card = cards.get(name)
        if card is None:
            lines.append(f"  - {name}: （无人物卡 canon/cast/{name}.md）")
            continue
        lines.append(f"  - {card.label}（{name}，{card.role or 'recurring'}）")
        snap = [row for row in snapshot_rows if name in row or (card.label and card.label in row)]
        if snap:
            lines.append(f"    snapshot: {snap[0]}")
        else:
            lines.append("    snapshot: （不在 Cast snapshot — Commit 后须补）")
        anchors = [f"{k}={v}" for k, v in card.anchors.items() if v]
        lines.append("    三锚点: " + ("; ".join(anchors[:3]) if anchors else "（无三锚点行）"))
        sample = card.dialogue_sample()
        if sample:
            lines.append(f"    台词: {sample}")
        if pov and name == pov:
            lines.append(f"    不知（POV 不得写出）: {card.unknown or '（知识边界未填）'}")
    rels = load_cast_relations(book_root, names, cache)
    if rels:
        lines.append("  关系（在场角色间）:")
        for rel in rels:
            lines.append(f"    - {rel}")
    return lines


# --- cast-lint ---


def check_cast_lint(book_root: Path, r: Report, cache=None) -> dict[str, CastCard]:
    cards = load_cast_cards(book_root, cache)
    if not cards:
        r.blocking("cast", "canon/cast/ has no cards")
        return cards
    protagonists = [c for c in cards.values() if c.role == "protagonist" and c.status == "canon"]
    for stem, card in cards.items():
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]*", stem):
            r.advisory("cast", f"{stem}.md: stem should be ascii slug (used as id in on_stage / 本卷人物 / 关系表)")
        if card.status not in CAST_STATUSES:
            r.blocking("cast", f"{stem}.md: `status` must be candidate|canon (got {card.status or 'missing'})")
        if card.role not in CAST_ROLES:
            r.blocking("cast", f"{stem}.md: `role` must be protagonist|volume_antagonist|recurring (got {card.role or 'missing'})")
        seen_other: set[str] = set()
        for row in card.relations:
            other = row["other"]
            if other in seen_other:
                r.advisory("relation", f"{stem}.md: duplicate 关系 row for {other}")
            seen_other.add(other)
            if other == stem:
                r.blocking("relation", f"{stem}.md: 关系 row points at itself")
                continue
            target = cards.get(other)
            if target is None:
                r.blocking("relation", f"{stem}.md: 关系 对方={other} has no canon/cast/{other}.md (write the stem)")
                continue
            if not any(back["other"] == stem for back in target.relations):
                r.blocking("relation", f"{stem}.md → {other} has no back row in {other}.md (质态可不同，但两边都要有行)")
            if not row["state"]:
                r.advisory("relation", f"{stem}.md → {other}: 当前质态 empty")
        if card.status == "canon":
            miss = card.missing_fields()
            if miss:
                r.advisory("cast", f"{stem}.md ({card.role}): missing {', '.join(miss)}")
    if not protagonists:
        r.advisory("cast", "no canon card with `role`: protagonist — asset gate blocks prose until one exists")
    return cards


def promote_cards(book_root: Path, stems: list[str], r: Report) -> list[str]:
    """candidate → canon by rewriting the `status` line. Returns promoted stems."""
    promoted: list[str] = []
    for stem in stems:
        path = cast_dir(book_root) / f"{stem}.md"
        if not path.is_file():
            r.blocking("cast", f"本卷人物 {stem}: no canon/cast/{stem}.md")
            continue
        text = read_book_text(path)
        card = parse_cast_card(text, stem, path)
        if card.status == "canon":
            continue
        new, n = re.subn(
            r"^(\s*`?status`?\s*[：:]\s*)candidate\b",
            r"\1canon",
            text,
            count=1,
            flags=re.M | re.I,
        )
        if n == 0:
            head, sep, rest = text.partition("\n")
            new = f"{head}\n\n`status`: canon\n{rest}" if sep else f"{text}\n\n`status`: canon\n"
        write_book_text(path, new)
        promoted.append(stem)
    return promoted

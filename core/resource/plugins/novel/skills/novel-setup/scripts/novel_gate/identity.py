"""Story clock + structured state_deltas + identity-at-unit (QueryAt opening).

Baseline lives on cast cards; live identity is Cast snapshot after Commit.
Preflight injects Identity@unit = baseline ∪ snapshot (NOT this unit's delta `to`).
"""
from __future__ import annotations

import re
from pathlib import Path
from typing import Any

from .common import nonempty_list, read_book_text, rune_count

IDENTITY_FIELDS = ("title", "age", "location", "power", "relation", "status", "other")
IDENTITY_VALUE_MAX_RUNES = 80  # longer "to" is narrative, not a snapshot cell
AGE_VALUE_MAX_RUNES = 24
FROM_TO_RE = re.compile(
    r"^(?:从\s*)?(.+?)\s*(?:→|->|⇒|=>|到|至)\s*(.+)$"
)
LEGACY_JOB_RE = re.compile(r"^\s*[-*]\s*\*?\*?本职[^：:]*[：:]\s*(.*)")
AGE_START_RE = re.compile(
    r"^\s*[-*]\s*\*?\*?(?:开卷年龄|age_at_story_start)[^：:]*[：:]\s*(.*)",
    re.I,
)
TITLE_BASE_RE = re.compile(
    r"^\s*[-*]\s*\*?\*?(?:本职\s*/\s*开卷身份|本职|开卷身份)[^：:]*[：:]\s*(.*)"
)
# Short age cell: "34" / "34岁" / "34岁（1972年生）"
AGE_VALUE_RE = re.compile(
    r"^\d{1,3}\s*岁?(?:\s*[（(][^）)]{0,20}[）)])?\s*$"
)


def looks_like_age_value(text: str) -> bool:
    """True only for short age-shaped values — not narrative that mentions 岁."""
    t = (text or "").strip()
    if not t or rune_count(t) > AGE_VALUE_MAX_RUNES:
        return False
    if re.fullmatch(r"\d{1,3}", t):
        return True
    return bool(AGE_VALUE_RE.match(t))


def looks_like_identity_value(text: str, *, max_runes: int = IDENTITY_VALUE_MAX_RUNES) -> bool:
    """Title/location snapshot cells are short; long prose is not comparable."""
    t = (text or "").strip()
    return bool(t) and rune_count(t) <= max_runes


def normalize_state_deltas(items: list[Any] | None) -> list[dict[str, str]]:
    """Normalize free-text or structured state_deltas into dict rows.

    Structured: {stem, field, from, to, note?}
    Legacy string: "stem: from→to" or "stem: note"

    Age is only assigned when from/to look like ages (not any string containing 岁).
    """
    out: list[dict[str, str]] = []
    for item in items or []:
        if isinstance(item, dict):
            stem = str(item.get("stem") or item.get("who") or "").strip()
            if not stem:
                continue
            field = str(item.get("field") or "other").strip().lower() or "other"
            if field not in IDENTITY_FIELDS:
                field = "other"
            frm = str(item.get("from") or item.get("old") or "").strip()
            to = str(item.get("to") or item.get("new") or "").strip()
            note = str(item.get("note") or "").strip()
            if field == "age" and not (looks_like_age_value(frm) or looks_like_age_value(to)):
                field = "other"
            out.append(
                {
                    "stem": stem,
                    "field": field,
                    "from": frm,
                    "to": to,
                    "note": note,
                }
            )
            continue
        s = str(item or "").strip()
        if not s:
            continue
        if "：" in s and ":" not in s.split("：", 1)[0]:
            who, rest = s.split("：", 1)
        elif ":" in s:
            who, rest = s.split(":", 1)
        else:
            who, rest = s, ""
        who, rest = who.strip(), rest.strip()
        if not who:
            continue
        frm, to, note, field = "", "", rest, "other"
        m = FROM_TO_RE.match(rest)
        if m:
            frm, to = m.group(1).strip(), m.group(2).strip()
            note = ""
        # Heuristic field from Chinese keywords (age only if values look like ages)
        low = rest
        if re.search(r"职位|官职|本职|头衔|称号|升职|贬职|捕头|捕快|官升|官贬", low):
            field = "title"
        elif looks_like_age_value(frm) or looks_like_age_value(to):
            field = "age"
        elif re.search(r"位置|抵达|离开|赶往|身在", low):
            field = "location"
        elif re.search(r"境界|修为|实力|突破", low):
            field = "power"
        elif re.search(r"信任|站队|债务|秘密|结盟|背叛|决裂|反目|投靠|欠|关系", low):
            field = "relation"
        out.append({"stem": who, "field": field, "from": frm, "to": to, "note": note})
    return out


def state_delta_who(items: list[Any] | None) -> list[str]:
    seen: list[str] = []
    for row in normalize_state_deltas(items):
        w = row["stem"]
        if w and w not in seen:
            seen.append(w)
    return seen


def format_delta_line(row: dict[str, str]) -> str:
    stem = row.get("stem") or "?"
    field = row.get("field") or "other"
    frm, to, note = row.get("from") or "", row.get("to") or "", row.get("note") or ""
    if frm and to:
        body = f"{frm}→{to}"
    elif to:
        body = to
    else:
        body = note or ""
    extra = f" ({note})" if note and (frm or to) else ""
    return f"{stem}.{field}: {body}{extra}".strip()


def relation_delta_rows(items: list[Any] | None) -> list[dict[str, str]]:
    rows = normalize_state_deltas(items)
    return [
        r
        for r in rows
        if r["field"] == "relation"
        or re.search(r"信任|站队|债务|秘密|结盟|背叛|决裂|反目|投靠|欠", r.get("note") or "")
        or re.search(r"信任|站队|债务|秘密|结盟|背叛|决裂|反目|投靠|欠", r.get("to") or "")
        or re.search(r"信任|站队|债务|秘密|结盟|背叛|决裂|反目|投靠|欠", r.get("from") or "")
    ]


def parse_cast_baseline(text: str) -> dict[str, str]:
    """age_at_story_start + baseline title from cast card markdown."""
    age, title = "", ""
    section = ""
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("#"):
            section = s.lstrip("#").strip()
            continue
        if "身份基线" in section or "功能" in section:
            m = AGE_START_RE.match(line)
            if m and not age:
                age = m.group(1).strip()
                continue
            m = TITLE_BASE_RE.match(line)
            if m and not title:
                title = m.group(1).strip()
                continue
            if not title:
                m = LEGACY_JOB_RE.match(line)
                if m:
                    title = m.group(1).strip()
    return {"age": age, "title": title}


def parse_cast_snapshot_table(ledger_text: str) -> dict[str, dict[str, str]]:
    """Parse Cast snapshot markdown table → {name_or_stem: {col: value}}.

    Supports both legacy columns (角色|位置|目标|…) and identity columns
    (角色|年龄|职位|位置|…).
    """
    headers: list[str] = []
    rows: dict[str, dict[str, str]] = {}
    in_cast = False
    for line in ledger_text.splitlines():
        if re.match(r"^###\s*Cast snapshot", line, re.I) or (
            "Cast snapshot" in line and line.startswith("#")
        ):
            in_cast = True
            headers = []
            continue
        if in_cast and line.startswith("#"):
            break
        if not in_cast or not line.strip().startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if not cells:
            continue
        if cells[0] in ("角色",) or set(cells[0]) <= {"-", ":"}:
            if cells[0] == "角色":
                headers = cells
            continue
        if not headers:
            # Legacy without explicit header capture yet — assume old layout
            headers = ["角色", "位置", "目标", "伤势/资源", "知情范围", "关系质态（一行）"]
            if len(cells) >= 7:
                headers = [
                    "角色",
                    "年龄",
                    "职位",
                    "位置",
                    "目标",
                    "伤势/资源",
                    "知情范围",
                    "关系质态（一行）",
                ]
        name = cells[0]
        if not name:
            continue
        entry: dict[str, str] = {}
        for i, h in enumerate(headers):
            if i < len(cells):
                entry[h] = cells[i]
        rows[name] = entry
    return rows


def _snapshot_get(row: dict[str, str], *keys: str) -> str:
    for k in keys:
        if k in row and str(row[k]).strip():
            return str(row[k]).strip()
    # fuzzy: header contains key fragment
    for k in keys:
        for hk, hv in row.items():
            if k in hk and str(hv).strip() and hk != "角色":
                return str(hv).strip()
    return ""


def identity_from_snapshot_and_baseline(
    name: str,
    label: str,
    snapshot_table: dict[str, dict[str, str]],
    baseline: dict[str, str],
) -> dict[str, str]:
    """Opening identity for a character: snapshot overrides baseline; no unit `to`."""
    row = snapshot_table.get(name) or snapshot_table.get(label) or {}
    age = _snapshot_get(row, "年龄", "age") or baseline.get("age") or ""
    title = _snapshot_get(row, "职位", "本职", "title") or baseline.get("title") or ""
    location = _snapshot_get(row, "位置", "location")
    goal = _snapshot_get(row, "目标", "goal")
    injury = _snapshot_get(row, "伤势/资源", "伤势", "资源")
    knowledge = _snapshot_get(row, "知情范围", "知情")
    relation = _snapshot_get(row, "关系质态（一行）", "关系质态", "关系")
    return {
        "age": age,
        "title": title,
        "location": location,
        "goal": goal,
        "injury": injury,
        "knowledge": knowledge,
        "relation": relation,
    }


def format_identity_line(ident: dict[str, str]) -> str:
    parts = []
    if ident.get("age"):
        parts.append(f"年龄={ident['age']}")
    if ident.get("title"):
        parts.append(f"职位={ident['title']}")
    if ident.get("location"):
        parts.append(f"位置={ident['location']}")
    if ident.get("goal"):
        parts.append(f"目标={ident['goal']}")
    if ident.get("injury"):
        parts.append(f"伤势/资源={ident['injury']}")
    if ident.get("knowledge"):
        parts.append(f"知情={ident['knowledge']}")
    if ident.get("relation"):
        parts.append(f"关系={ident['relation']}")
    return "；".join(parts) if parts else "（无年龄/职位/位置 — 补人物卡基线或 Cast snapshot）"


def story_day_of(unit: dict) -> int | None:
    raw = unit.get("story_day")
    if raw is None or raw == "":
        return None
    if isinstance(raw, bool):
        return None
    if isinstance(raw, (int, float)):
        return int(raw)
    s = str(raw).strip()
    if not s or s.lower() in ("null", "none", "~"):
        return None
    m = re.search(r"-?\d+", s)
    return int(m.group()) if m else None


def is_flashback(unit: dict) -> bool:
    v = unit.get("flashback")
    if isinstance(v, bool):
        return v
    return str(v or "").strip().lower() in ("true", "yes", "1")


def validate_timeline_fields(unit: dict, r, rel: str, *, require_clock: bool = False) -> None:
    """accepted+ : gap_from_prev blocking; story_day|time_label at least one blocking."""
    from .common import Report, is_blank

    assert isinstance(r, Report)
    status = str(unit.get("status") or "").strip()
    if status not in ("accepted", "drafted", "reviewed") and not require_clock:
        return
    if is_blank(unit.get("gap_from_prev")):
        r.blocking("timeline", f"{rel} gap_from_prev empty — 与上单元时间差必填（accepted+）")
    day = story_day_of(unit)
    if day is None and is_blank(unit.get("time_label")):
        r.blocking(
            "timeline",
            f"{rel} story_day/time_label both empty — 本单元时钟锚点必填（accepted+）",
        )
    checks = unit.get("constraint_checks") or {}
    if isinstance(checks, dict) and checks.get("timeline_monotonic") is False and not is_flashback(unit):
        r.advisory(
            "timeline",
            f"{rel} constraint_checks.timeline_monotonic=false without flashback:true",
        )


def format_unit_clock(unit: dict) -> str:
    bits = []
    day = story_day_of(unit)
    if day is not None:
        bits.append(f"story_day={day}")
    gap = str(unit.get("gap_from_prev") or "").strip()
    if gap:
        bits.append(f"gap={gap}")
    tlabel = str(unit.get("time_label") or "").strip()
    if tlabel:
        bits.append(f"label={tlabel}")
    if is_flashback(unit):
        bits.append("flashback")
    return "；".join(bits) if bits else "（无时钟字段）"


def previous_unit_dict(book_root: Path, unit: dict) -> tuple[str, dict] | tuple[str, None]:
    """Nearest prior unit in the same volume by chapter_range end < current start."""
    from .common import load_yaml_map
    from .outline import chapter_range_of

    uid = str(unit.get("unit_id") or "").strip()
    if "-" not in uid:
        return "", None
    vol = uid.split("-")[0]
    cur_a, _ = chapter_range_of(unit)
    units_dir = book_root / "outline" / "units"
    if not units_dir.is_dir() or cur_a <= 0:
        return "", None
    best_id, best, best_end = "", None, -1
    for path in sorted(units_dir.glob(f"{vol}-U*.yaml")):
        if path.stem == uid:
            continue
        try:
            other = load_yaml_map(read_book_text(path))
        except OSError:
            continue
        _oa, ob = chapter_range_of(other)
        if ob <= 0 or ob >= cur_a:
            continue
        if ob >= best_end:
            best_id, best, best_end = path.stem, other, ob
    return best_id, best


def check_timeline_monotonic(book_root: Path, unit: dict, r, rel: str) -> None:
    """Blocking when this unit's story_day goes backward vs previous unit (same volume)."""
    from .common import Report, load_yaml_map
    from .outline import chapter_range_of

    assert isinstance(r, Report)
    checks = unit.get("constraint_checks") or {}
    if isinstance(checks, dict) and checks.get("timeline_monotonic") is False:
        return
    if is_flashback(unit):
        return
    day = story_day_of(unit)
    if day is None:
        return
    uid = str(unit.get("unit_id") or "").strip()
    if "-" not in uid:
        return
    vol = uid.split("-")[0]
    units_dir = book_root / "outline" / "units"
    if not units_dir.is_dir():
        return
    cur_a, _ = chapter_range_of(unit)
    prev_day: int | None = None
    prev_id = ""
    prev_end = -1
    for path in sorted(units_dir.glob(f"{vol}-U*.yaml")):
        if path.stem == uid:
            continue
        try:
            other = load_yaml_map(read_book_text(path))
        except OSError:
            continue
        _oa, ob = chapter_range_of(other)
        if ob <= 0:
            continue
        if cur_a > 0 and ob >= cur_a:
            continue
        if is_flashback(other):
            continue
        od = story_day_of(other)
        if od is None:
            continue
        if ob >= prev_end:
            prev_day, prev_id, prev_end = od, path.stem, ob
    if prev_day is not None and day < prev_day:
        r.blocking(
            "timeline",
            f"{rel} story_day={day} < previous unit {prev_id} story_day={prev_day} "
            f"(set flashback:true or fix clock)",
        )


def deltas_end_goals_lines(items: list[Any] | None) -> list[str]:
    """Human lines for unit-end identity goals (not opening identity)."""
    lines: list[str] = []
    for row in normalize_state_deltas(items):
        if not (row.get("to") or row.get("note")):
            continue
        lines.append(format_delta_line(row))
    return lines


def apply_deltas_to_identity(
    ident: dict[str, str], deltas: list[dict[str, str]], stem: str
) -> dict[str, str]:
    """Replay deltas for stem onto identity dict (field→to). Used for Commit guidance tests."""
    out = dict(ident)
    for row in deltas:
        if row["stem"] != stem:
            continue
        field = row["field"]
        val = row.get("to") or ""
        if not val:
            continue
        if field == "title":
            out["title"] = val
        elif field == "age":
            out["age"] = val
        elif field == "location":
            out["location"] = val
        elif field == "relation":
            out["relation"] = val
        elif field == "power":
            out["injury"] = (out.get("injury") or "") + (f" / 能力:{val}" if out.get("injury") else f"能力:{val}")
    return out

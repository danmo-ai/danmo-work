"""Unit outline (YAML) and volume outline (Markdown) access + shape validation."""
from __future__ import annotations

import re
from pathlib import Path

from .common import (
    UNIT_ID_RE,
    Report,
    _as_int,
    is_blank,
    load_yaml_map,
    nonempty_list,
    read_book_text,
    write_book_text,
)

HOOK_TYPES = {
    "信息缺口", "未兑现承诺", "高代价选择",
    "身份反转", "关系临界", "倒计时",
    "未完成动作", "突然揭示", "回声/意象",
}
BEAT_NAMES = {"建立期待", "尝试", "加压", "决断", "兑现", "余波"}
CHAPTER_HEAD_RE = re.compile(r"^## 第(\d+)章(?:\s+(.*?))?\s*$")
VOLUME_UNIT_ROW = re.compile(r"\|\s*U(\d+)\s*\|")
UNIT_BEAT_RE = re.compile(
    r"ch\s*(\d+)\s*(?:[-–—]\s*ch\s*(\d+))?\s*(建立期待|尝试|加压|决断|兑现|余波|切断)[：:]\s*(.*)",
    re.I,
)


def unit_outline_rel(unit_id: str) -> str:
    return f"outline/units/{unit_id}.yaml"


def unit_prose_rel(unit_id: str) -> str:
    return f"units/{unit_id}.md"


def volume_outline_rel(volume: str) -> str:
    return f"outline/volumes/{volume}.md"


def chapter_range_of(unit: dict) -> tuple[int, int]:
    raw = unit.get("chapter_range")
    if isinstance(raw, list) and len(raw) >= 2:
        a, b = _as_int(raw[0]), _as_int(raw[1])
        if a > 0 and b >= a:
            return a, b
    if isinstance(raw, str):
        m = re.search(r"(\d+)\s*,\s*(\d+)", raw)
        if m:
            a, b = int(m.group(1)), int(m.group(2))
            if a > 0 and b >= a:
                return a, b
    return 0, 0


def scene_rows(unit: dict) -> list[dict]:
    raw = unit.get("scenes") or []
    if not isinstance(raw, list):
        return []
    return [x for x in raw if isinstance(x, dict)]


def chapter_rows(unit: dict) -> list[dict]:
    raw = unit.get("chapters") or []
    if not isinstance(raw, list):
        return []
    return [x for x in raw if isinstance(x, dict)]


def next_hook_of(unit: dict) -> tuple[str, str]:
    hook = unit.get("next_hook") or {}
    if not isinstance(hook, dict):
        return "", ""
    return str(hook.get("type") or "").strip(), str(hook.get("out") or "").strip()


def _unit_sort_key(s: str) -> tuple[int, int]:
    return int(re.search(r"\d+", s).group()), int(s.rsplit("U", 1)[-1])


def list_unit_ids(book_root: Path, kind: str) -> list[str]:
    folder = book_root / ("outline/units" if kind == "outline" else "units")
    if not folder.is_dir():
        return []
    out: list[str] = []
    for e in folder.iterdir():
        if e.is_dir():
            continue
        m = UNIT_ID_RE.match(e.stem)
        if not m:
            continue
        if kind == "outline" and e.suffix.lower() in {".yaml", ".yml"}:
            out.append(e.stem)
        elif kind == "prose" and e.suffix.lower() == ".md":
            out.append(e.stem)
    return sorted(out, key=_unit_sort_key)


def load_unit(book_root: Path, unit_id: str) -> tuple[dict, str]:
    rel = unit_outline_rel(unit_id)
    path = book_root / rel
    if not path.is_file():
        raise FileNotFoundError(rel)
    return load_yaml_map(read_book_text(path)), rel


def split_unit_prose(text: str) -> tuple[list[dict], list[str]]:
    """Split a unit file on ## 第N章. Returns slices and blocking messages."""
    lines = text.splitlines()
    heads: list[tuple[int, int, str]] = []
    for i, line in enumerate(lines):
        m = CHAPTER_HEAD_RE.match(line.strip())
        if m:
            heads.append((i, int(m.group(1)), (m.group(2) or "").strip()))
    errors: list[str] = []
    if not heads:
        return [], ["no ## 第N章 headings"]
    slices: list[dict] = []
    for idx, (line_i, num, title) in enumerate(heads):
        end = heads[idx + 1][0] if idx + 1 < len(heads) else len(lines)
        if idx == 0:
            prev = line_i - 1
            while prev >= 0 and not lines[prev].strip():
                prev -= 1
            if prev >= 0 and lines[prev].strip() == "---":
                errors.append(f"chapter {num} must not have a --- divider before the first heading")
        else:
            prev = line_i - 1
            while prev >= 0 and not lines[prev].strip():
                prev -= 1
            if prev < 0 or lines[prev].strip() != "---":
                errors.append(f"chapter {num} missing --- divider before heading")
        body_lines = lines[line_i + 1 : end]
        while body_lines and not body_lines[-1].strip():
            body_lines.pop()
        if body_lines and body_lines[-1].strip() == "---":
            body_lines.pop()
            while body_lines and not body_lines[-1].strip():
                body_lines.pop()
        slices.append(
            {
                "chapter": num,
                "title": title,
                "body": "\n".join(body_lines).strip(),
                "line": line_i + 1,
            }
        )
    return slices, errors


def legacy_chapters_message(book_root: Path) -> str | None:
    ch = book_root / "chapters"
    if not ch.is_dir():
        return None
    files = [p for p in ch.iterdir() if p.is_file() and not p.name.startswith(".")]
    if not files:
        return None
    units = book_root / "units"
    prose = list(units.glob("v*-U*.md")) if units.is_dir() else []
    if prose:
        return None
    return (
        "chapters/ is obsolete and units/ has no prose — migrate into "
        "units/<unit-id>.md (one file, --- between ## 第N章). Gate does not read chapters/."
    )


def state_delta_who(items: list[str]) -> list[str]:
    out = []
    for item in items:
        s = item.strip()
        if ":" in s:
            out.append(s.split(":", 1)[0].strip())
        elif "：" in s:
            out.append(s.split("：", 1)[0].strip())
        elif s:
            out.append(s)
    return [w for w in out if w]


def on_stage_of(unit: dict) -> list[str]:
    """Canonical stems listed in the unit's `on_stage` field (deduplicated, order kept)."""
    seen: list[str] = []
    for w in nonempty_list(unit.get("on_stage")):
        if w not in seen:
            seen.append(w)
    return seen


def pov_of(unit: dict) -> str:
    return str(unit.get("pov") or "").strip()


def _outline_texts(book_root: Path, cache=None) -> list[tuple[Path, str]]:
    if cache is not None:
        return cache.outline_texts()
    outline = book_root / "outline"
    if not outline.is_dir():
        return []
    return [(path, read_book_text(path)) for path in sorted(outline.rglob("*.md"))]


def unit_listed(outline_root: Path, unit_id: str, cache=None) -> bool:
    """True only when a volume outline has a structured row for this unit.
    Plain text mention of the id is not enough."""
    unit_id = unit_id.strip()
    if not unit_id:
        return False
    u_short = ""
    vol = ""
    i = unit_id.rfind("-U")
    if i >= 0:
        vol = unit_id[:i]
        u_short = unit_id[i + 1 :]
    if cache is not None:
        texts = cache.outline_texts()
    elif outline_root.is_dir():
        texts = [(path, read_book_text(path)) for path in outline_root.rglob("*.md")]
    else:
        return False
    for path, text in texts:
        # Structured row: | U1 | ... | v01-U1 | ... or - 单元ID：`v01-U1`
        if re.search(r"^\s*[-*]?\s*(?:\|[^|\n]*){0,3}[^\n]*" + re.escape(unit_id) + r"(?![\w-])", text, re.M):
            return True
        if u_short and VOLUME_UNIT_ROW.search(text) and f"| {u_short} |" in text:
            base = path.stem
            if base.lower() == vol.lower() or vol in text:
                return True
    return False


def unit_beat_line(book_root: Path, unit_id: str, ch: int, cache=None) -> str:
    unit_id = (unit_id or "").strip()
    if not unit_id:
        return ""
    texts = _outline_texts(book_root, cache)
    for path, text in texts:
        if unit_id not in text and f"`{unit_id}`" not in text:
            continue
        # Prefer beat lines covering this chapter
        for line in text.splitlines():
            m = UNIT_BEAT_RE.search(line)
            if not m:
                continue
            a, b, role, detail = m.group(1), m.group(2), m.group(3), m.group(4)
            start, end = int(a), int(b) if b else int(a)
            if start <= ch <= end:
                return f"{role}: {detail.strip()}" if detail.strip() else role
        # Fallback: unit function line near unit id
        if f"`{unit_id}`" in text or unit_id in text:
            for line in text.splitlines():
                if "单元功能" in line and "：" in line:
                    return line.split("：", 1)[-1].strip() or line.strip()
    return ""


# --- volume outline (Markdown): unit index + 本卷人物 ---


def _md_table_rows(text: str, heading_key: str) -> list[list[str]]:
    """Cells of table rows under the first heading containing heading_key."""
    rows: list[list[str]] = []
    grab = False
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("#"):
            grab = heading_key in s
            continue
        if not grab or not s.startswith("|"):
            continue
        cells = [c.strip() for c in s.strip("|").split("|")]
        if not cells or set(cells[0]) <= {"-", ":"}:
            continue
        rows.append(cells)
    return rows


def _strip_code(s: str) -> str:
    return s.strip().strip("`").strip()


def _parse_range_cell(s: str) -> tuple[int, int]:
    m = re.search(r"(\d+)\s*[-–—~]\s*(?:ch)?\s*(\d+)", s, re.I)
    if m:
        a, b = int(m.group(1)), int(m.group(2))
        if a > 0 and b >= a:
            return a, b
    m = re.search(r"(\d+)", s)
    if m:
        n = int(m.group(1))
        return n, n
    return 0, 0


def parse_volume_index(text: str, volume: str) -> list[dict]:
    """Rows of the 单元索引 table:
    | unit_id | 章范围 | 一句话功能 | 终局边界短语 | 下一钩类型 |.
    Accepts `U1` or `v01-U1` in the first column. Header row is skipped."""
    rows = _md_table_rows(text, "单元索引")
    out: list[dict] = []
    for cells in rows:
        first = _strip_code(cells[0])
        if not first or first in ("unit_id", "单元", "单元 id", "单元ID", "单元id"):
            continue
        uid = first
        m = re.fullmatch(r"U(\d+)", first)
        if m:
            uid = f"{volume}-U{m.group(1)}"
        if not UNIT_ID_RE.match(uid):
            continue
        a, b = _parse_range_cell(cells[1]) if len(cells) > 1 else (0, 0)
        out.append(
            {
                "unit_id": uid,
                "chapter_range": [a, b],
                "function": cells[2] if len(cells) > 2 else "",
                "endgame_boundary": cells[3] if len(cells) > 3 else "",
                "next_hook_type": _strip_code(cells[4]) if len(cells) > 4 else "",
            }
        )
    return out


def parse_volume_cast(text: str) -> list[str]:
    """Stems listed under `## 本卷人物` (one `- stem` per line, backticks/notes tolerated)."""
    out: list[str] = []
    grab = False
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("#"):
            grab = "本卷人物" in s
            continue
        if not grab:
            continue
        m = re.match(r"^[-*]\s*`?([A-Za-z0-9][A-Za-z0-9_-]*)`?", s)
        if m:
            stem = m.group(1)
            if stem not in out:
                out.append(stem)
    return out


def load_volume(book_root: Path, volume: str) -> tuple[str, str]:
    rel = volume_outline_rel(volume)
    path = book_root / rel
    if not path.is_file():
        raise FileNotFoundError(rel)
    return read_book_text(path), rel


def volume_index_row(book_root: Path, unit_id: str, cache=None) -> dict | None:
    vol = unit_id.split("-")[0] if "-" in unit_id else ""
    if not vol:
        return None
    path = book_root / volume_outline_rel(vol)
    if not path.is_file():
        return None
    text = None
    if cache is not None:
        for p, t in cache.outline_texts():
            if p == path:
                text = t
                break
    if text is None:
        text = read_book_text(path)
    for row in parse_volume_index(text, vol):
        if row["unit_id"] == unit_id:
            return row
    return None


def _volume_text(book_root: Path, volume: str, cache=None) -> str | None:
    path = book_root / volume_outline_rel(volume)
    if not path.is_file():
        return None
    if cache is not None:
        for p, t in cache.outline_texts():
            if p == path:
                return t
    return read_book_text(path)


def volume_cast_for(book_root: Path, unit_id: str, cache=None) -> list[str] | None:
    """Stems under 「本卷人物」 of the unit's volume; None when the section/file is absent."""
    vol = unit_id.split("-")[0] if "-" in unit_id else ""
    if not vol:
        return None
    text = _volume_text(book_root, vol, cache)
    if text is None or "本卷人物" not in text:
        return None
    return parse_volume_cast(text)


def _norm_sentence(s: str) -> str:
    return re.sub(r"[\s\W_]+", "", str(s or ""), flags=re.UNICODE)


# --- shape validation ---


def validate_unit_shape(unit: dict, r: Report, rel: str) -> None:
    uid = str(unit.get("unit_id") or "").strip()
    if not uid:
        r.blocking("unit_id", f"{rel} unit_id empty — return to novel-plan")
    elif not UNIT_ID_RE.match(uid):
        r.blocking("unit_id", f"{rel} unit_id={uid} must match vNN-U#")
    a, b = chapter_range_of(unit)
    if a <= 0:
        r.blocking("chapter_range", f"{rel} chapter_range invalid")
    scenes = scene_rows(unit)
    chs = chapter_rows(unit)
    nums = [_as_int(c.get("chapter")) for c in chs]
    if a > 0 and nums != list(range(a, b + 1)):
        r.blocking("chapters", f"{rel} chapters {nums} != range {a}-{b}")
    counts: dict[int, int] = {}
    for s in scenes:
        n = _as_int(s.get("chapter"))
        counts[n] = counts.get(n, 0) + 1
        beat = str(s.get("beat") or "").strip()
        if beat not in BEAT_NAMES:
            r.blocking("scenes", f"{rel} scene {s.get('id') or '?'} beat={beat!r} invalid")
        landed = s.get("must_land") or []
        if not isinstance(landed, list) or not any(str(x).strip() for x in landed):
            r.blocking("scenes", f"{rel} scene {s.get('id') or '?'} must_land empty")
    if a > 0:
        for n in range(a, b + 1):
            if counts.get(n, 0) < 2:
                r.blocking("scenes", f"{rel} chapter {n} has {counts.get(n, 0)} scenes, need ≥2")
    shares = [_as_int(c.get("word_share")) for c in chs]
    wt = _as_int(unit.get("word_target"))
    if chs and sum(shares) != wt:
        r.blocking("word_target", f"{rel} word_share sum {sum(shares)} != word_target {wt}")
    if is_blank(unit.get("function")):
        r.blocking("contract", f"{rel} function empty")
    htype, hout = next_hook_of(unit)
    if htype not in HOOK_TYPES:
        r.blocking("hook", f"next_hook.type must be one of KB 爽点与追读 types, got {htype}")
    if not hout:
        r.blocking("hook", "next_hook.out empty (need a concrete event)")


def validate_unit_against_volume(unit: dict, row: dict | None, r: Report, rel: str) -> None:
    """卷纲定调、细纲对准: function (warning), next_hook.type (blocking), 终局边界 (blocking)."""
    if row is None:
        return
    a, b = chapter_range_of(unit)
    ra, rb = row.get("chapter_range") or [0, 0]
    if ra > 0 and (a, b) != (ra, rb):
        r.blocking("chapter_range", f"{rel} chapter_range {a}-{b} != volume index {ra}-{rb}")
    vf = str(row.get("function") or "").strip()
    uf = str(unit.get("function") or "").strip()
    if vf and uf and _norm_sentence(vf) != _norm_sentence(uf):
        r.advisory("function", f"{rel} function differs from volume index — 以卷纲为准: {vf}")
    vt = str(row.get("next_hook_type") or "").strip()
    ut, _ = next_hook_of(unit)
    if vt and vt in HOOK_TYPES and ut and vt != ut:
        r.blocking("hook", f"{rel} next_hook.type={ut} != volume index {vt}")
    boundary = str(row.get("endgame_boundary") or "").strip()
    if boundary and boundary not in ("-", "—", "无"):
        if not nonempty_list(unit.get("forbidden")) and is_blank(unit.get("endgame_boundary")):
            r.blocking(
                "endgame_boundary",
                f"{rel} volume index sets 终局边界「{boundary}」 but forbidden and endgame_boundary are both empty",
            )


def validate_on_stage(unit: dict, volume_cast: list[str] | None, cast_status: dict[str, str], r: Report, rel: str) -> None:
    """on_stage ⊆ 本卷人物; every on_stage stem must be a canon card."""
    stage = on_stage_of(unit)
    if not stage:
        r.advisory("on_stage", f"{rel} on_stage empty — 未列上场人物，CONTEXT 不带人物")
        return
    if volume_cast is not None:
        extra = [s for s in stage if s not in volume_cast]
        if extra:
            r.blocking("on_stage", f"{rel} on_stage {extra} not in 卷纲「本卷人物」")
    for stem in stage:
        status = cast_status.get(stem)
        if status is None:
            r.blocking("on_stage", f"{rel} on_stage {stem}: no canon/cast/{stem}.md")
        elif status != "canon":
            r.blocking("cast", f"{rel} on_stage {stem} is {status} — promote to canon first (accept-volume)")
    pov = pov_of(unit)
    if pov and pov not in stage:
        r.blocking("pov", f"{rel} pov={pov} not in on_stage")
    for s in scene_rows(unit):
        who = nonempty_list(s.get("who"))
        bad = [w for w in who if w not in stage]
        if bad:
            r.blocking("on_stage", f"{rel} scene {s.get('id') or '?'} who {bad} not in on_stage")
        spov = str(s.get("pov") or "").strip()
        if spov and spov not in stage:
            r.blocking("pov", f"{rel} scene {s.get('id') or '?'} pov={spov} not in on_stage")


# --- accept-volume / lint-units ---


def _yaml_str(v: str) -> str:
    v = str(v or "")
    if v == "" or re.search(r"[:#\"'\[\]{}|>&*!%@`,]", v) or v.strip() != v:
        return '"' + v.replace("\\", "\\\\").replace('"', '\\"') + '"'
    return v


def seed_unit_yaml(row: dict, unit_scale: dict, volume_cast: list[str]) -> str:
    """Head of outline/units/vNN-U#.yaml from a volume index row (status: proposed).
    Model fills the contract + scenes + chapters; gate keeps checking shape."""
    a, b = row.get("chapter_range") or [0, 0]
    n_ch = (b - a + 1) if (a > 0 and b >= a) else 0
    floor_per = _as_int(unit_scale.get("word_floor_per_ch"), 2000)
    ceil_per = _as_int(unit_scale.get("word_ceiling_per_ch"), 3500)
    target_per = 2500
    hook_type = row.get("next_hook_type") or ""
    lines = [
        f"# 单元细纲 {row['unit_id']}：由 accept-volume 按卷纲索引行种下头部；合同、场面、章切口由 novel-write 填。",
        "# 场面是写作骨架，章只是切口。每章 ≥2 场；word_share 之和 = word_target。",
        f"unit_id: {row['unit_id']}",
        f"chapter_range: [{a}, {b}]",
        'title_working: ""',
        f"word_target: {n_ch * target_per}",
        f"word_floor: {n_ch * floor_per}",
        f"word_ceiling: {n_ch * ceil_per}",
        "status: proposed             # proposed|accepted|drafted|reviewed",
        "",
        f"function: {_yaml_str(row.get('function') or '')}   # 来自卷纲索引；改动只 warning，以卷纲为准",
        'entry: ""',
        'desire: ""',
        'obstacle: ""',
        'choice: ""',
        'payoff: ""',
        'pleasure: ""',
        "forbidden: []",
        f"endgame_boundary: {_yaml_str(row.get('endgame_boundary') or '')}",
        "next_hook:",
        f"  type: {_yaml_str(hook_type)}   # 来自卷纲索引；不一致 blocking",
        '  out: ""',
        "",
        f"on_stage: []      # ⊆ 卷纲「本卷人物」: {', '.join(volume_cast) if volume_cast else '（卷纲未列）'}",
        'pov: ""           # 默认 POV stem；场面可用 pov 覆盖',
        "",
        "scenes: []        # - id: S1 / beat / chapter / where / pov / who / want / turn / must_land",
        "chapters: []      # - chapter / title_working / opens_on / ends_on / cut_hook / word_share",
        "",
        "state_deltas: []",
        "info_control:",
        "  reveals: []",
        "  foreshadowing: []",
        "constraint_checks:",
        "  goldfinger_ratio_ok: true",
        "  timeline_monotonic: true",
        "  active_plot_lines_le_3: true",
        "  endgame_reserve_ok: true",
        "  agency_settled: true",
        "  unit_size_within_bounds: true",
        "continuity_risks: []",
        "",
    ]
    return "\n".join(lines)


def validate_volume_index(rows: list[dict], volume: str, r: Report, rel: str) -> None:
    if not rows:
        r.blocking("volume", f"{rel} has no 单元索引 rows")
        return
    prev_end = 0
    for row in rows:
        a, b = row["chapter_range"]
        if not row["unit_id"].startswith(volume + "-"):
            r.blocking("volume", f"{rel} row {row['unit_id']} does not belong to {volume}")
        if a <= 0:
            r.blocking("volume", f"{rel} row {row['unit_id']} 章范围 unreadable")
            continue
        if prev_end and a != prev_end + 1:
            r.blocking("volume", f"{rel} 章范围 gap/overlap before {row['unit_id']}: ch{prev_end} → ch{a}")
        prev_end = b
        if b - a + 1 > 10:
            r.blocking("volume", f"{rel} row {row['unit_id']} has {b - a + 1} chapters (hard cap 10)")
        ht = row.get("next_hook_type") or ""
        if ht and ht not in HOOK_TYPES:
            r.blocking("volume", f"{rel} row {row['unit_id']} 钩子类型 {ht!r} not in nine types")
        if not str(row.get("function") or "").strip():
            r.blocking("volume", f"{rel} row {row['unit_id']} 一句话功能 empty")


def accept_volume(book_root: Path, st: dict, volume: str, r: Report) -> dict:
    """Promote 「本卷人物」 to canon and seed proposed unit YAML heads from the index."""
    from .cast import promote_cards

    try:
        text, rel = load_volume(book_root, volume)
    except OSError as e:
        r.blocking("volume", f"{volume_outline_rel(volume)}: {e}")
        return {}
    rows = parse_volume_index(text, volume)
    validate_volume_index(rows, volume, r, rel)
    cast = parse_volume_cast(text)
    if "本卷人物" not in text:
        r.blocking("volume", f"{rel} missing `## 本卷人物` section (stems; 批准即 canon)")
    elif not cast:
        r.blocking("volume", f"{rel} 「本卷人物」 lists no stems")
    if any(f["severity"] == "blocking" for f in r.findings):
        return {"rows": rows, "cast": cast}
    promoted = promote_cards(book_root, cast, r)
    seeded: list[str] = []
    kept: list[str] = []
    scale = st.get("unit_scale") if isinstance(st.get("unit_scale"), dict) else {}
    for row in rows:
        path = book_root / unit_outline_rel(row["unit_id"])
        if path.is_file():
            kept.append(row["unit_id"])
            continue
        write_book_text(path, seed_unit_yaml(row, scale, cast))
        seeded.append(row["unit_id"])
    return {"rows": rows, "cast": cast, "promoted": promoted, "seeded": seeded, "kept": kept}


def lint_units(book_root: Path, st: dict, volume: str, r: Report, cache=None) -> list[tuple[str, str, list[str]]]:
    """Validate every outline/units/<volume>-U#.yaml against shape + volume rules.
    Returns [(unit_id, PASS|FAIL, messages)]; findings roll up into r."""
    from .cast import cast_status_map

    text = _volume_text(book_root, volume, cache)
    rows: list[dict] = []
    volume_cast: list[str] | None = None
    if text is None:
        r.blocking("volume", f"{volume_outline_rel(volume)} missing")
    else:
        rows = parse_volume_index(text, volume)
        validate_volume_index(rows, volume, r, volume_outline_rel(volume))
        volume_cast = parse_volume_cast(text) if "本卷人物" in text else None
        if volume_cast is None:
            r.advisory("volume", f"{volume_outline_rel(volume)} has no 「本卷人物」 — on_stage subset check skipped")
    by_id = {row["unit_id"]: row for row in rows}
    status_map = cast_status_map(book_root, cache)
    unit_ids = [u for u in list_unit_ids(book_root, "outline") if u.startswith(volume + "-")]
    for uid in by_id:
        if uid not in unit_ids:
            r.advisory("volume", f"{uid} indexed in 卷纲 but {unit_outline_rel(uid)} missing (run accept-volume)")
    results: list[tuple[str, str, list[str]]] = []
    for uid in unit_ids:
        sub = Report("lint-units", r.book_id, book_root, 0, uid)
        try:
            u, rel = load_unit(book_root, uid)
        except (OSError, UnicodeDecodeError) as e:
            sub.blocking("contract", f"{unit_outline_rel(uid)}: {e}")
            sub.finalize()
            results.append((uid, sub.verdict, [f"[{f['check']}] {f['message']}" for f in sub.findings]))
            r.findings.extend(sub.findings)
            continue
        filed = str(u.get("unit_id") or "").strip()
        if filed and filed != uid:
            sub.blocking("unit_id", f"{rel} unit_id={filed} want {uid}")
        if uid not in by_id:
            sub.blocking("unit_id", f"{uid} not in 卷纲 单元索引")
        status = str(u.get("status") or "").strip()
        if status == "proposed" and not scene_rows(u):
            sub.advisory("contract", f"{rel} status=proposed, no scenes yet — 待细纲")
        else:
            validate_unit_shape(u, sub, rel)
        validate_unit_against_volume(u, by_id.get(uid), sub, rel)
        if scene_rows(u) or on_stage_of(u):
            validate_on_stage(u, volume_cast, status_map, sub, rel)
        sub.finalize()
        results.append((uid, sub.verdict, [f"[{f['check']}] {f['message']}" for f in sub.findings]))
        r.findings.extend(sub.findings)
    return results

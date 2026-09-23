"""Preflight: state validation, knowledge-base article lookup, and the `### CONTEXT`
block — the only pre-writing injection (plan §6)."""
from __future__ import annotations

import os
import re
from pathlib import Path

from .cast import cast_context_lines, cast_status_map
from .common import (
    UNIT_ID_RE,
    Report,
    file_exists,
    is_blank,
    load_yaml_map,
    nonempty_list,
    read_book_text,
    rune_count,
    tomato_profile,
    volume_number,
    writing_stage,
)
from .ledger import (
    MAX_OPEN_DEBTS,
    cast_snapshot_rows,
    extract_chapter_summary_block,
    has_reader_continuity,
    ledger_path,
    load_locked_terms,
    open_debt_count,
    open_loops_rows,
    summary_source_text,
    volume_summary_text,
)
from .outline import (
    chapter_range_of,
    chapter_rows,
    legacy_chapters_message,
    load_unit,
    next_hook_of,
    on_stage_of,
    pov_of,
    scene_rows,
    unit_listed,
    unit_outline_rel,
    validate_on_stage,
    validate_unit_against_volume,
    validate_unit_shape,
    volume_cast_for,
    volume_index_row,
    volume_outline_rel,
)

STYLE_MAX_RUNES = 480  # context hook: keep the injected style brief tiny
QC_PROFILES = {"male_power", "female_emotion", "mystery", "general"}
CRAFT_LANES = {"default", "crime-human"}
GENRES = ("玄幻", "仙侠", "都市", "悬疑", "现代言情", "古代言情", "仕途扫黑", "系统穿越")
CRIME_LANE_ARTICLE = "刑侦人味文风"
KB_REL = "ai.danmo.work/knowledge"


# --- state ---


def craft_lane_of(st: dict | None) -> str:
    if not st:
        return "default"
    raw = str(st.get("craft_lane") or "default").strip().lower().replace("_", "-")
    return "crime-human" if raw == "crime-human" else "default"


def genre_of(st: dict | None) -> str:
    if not st:
        return ""
    return str(st.get("genre") or "").strip()


def validate_state_fields(st: dict, r: Report) -> None:
    qc = str(st.get("qc_profile") or "").strip()
    if qc and qc not in QC_PROFILES:
        r.blocking("state", f"qc_profile={qc} not in {sorted(QC_PROFILES)}")
    lane = str(st.get("craft_lane") or "").strip()
    if lane and lane not in CRAFT_LANES:
        r.blocking("state", f"craft_lane={lane} not in {sorted(CRAFT_LANES)}")
    genre = genre_of(st)
    if genre and genre not in GENRES:
        r.blocking("state", f"genre={genre} not in {list(GENRES)}")
    if craft_lane_of(st) == "crime-human" and genre and genre != "悬疑":
        r.blocking("state", f"craft_lane=crime-human requires genre=悬疑 (got {genre})")


# --- knowledge base ---


def plugin_root() -> Path:
    raw = os.environ.get("NOVEL_PLUGIN_ROOT", "").strip()
    if raw:
        return Path(raw)
    # scripts/novel_gate/context.py → scripts → novel-setup → skills → novel
    return Path(__file__).resolve().parents[4]


def knowledge_dir() -> Path:
    return plugin_root() / KB_REL


def _article_title(text: str) -> str:
    for line in text.splitlines():
        if line.startswith("# "):
            return line[2:].strip()
    return ""


def knowledge_article(title: str) -> str:
    """Full text of the knowledge article whose H1 equals `title` ('' when absent)."""
    d = knowledge_dir()
    if not title or not d.is_dir():
        return ""
    for path in sorted(d.glob("*.md")):
        text = path.read_text(encoding="utf-8")
        if _article_title(text) == title:
            return text.strip()
    return ""


def genre_articles(st: dict | None) -> list[tuple[str, str]]:
    """[(title, text)] to inject: the genre article, plus 刑侦人味文风 on crime-human."""
    out: list[tuple[str, str]] = []
    genre = genre_of(st)
    if genre in GENRES:
        text = knowledge_article(genre)
        if text:
            out.append((genre, text))
    if genre == "悬疑" and craft_lane_of(st) == "crime-human":
        text = knowledge_article(CRIME_LANE_ARTICLE)
        if text:
            out.append((CRIME_LANE_ARTICLE, text))
    return out


# --- style brief ---


def _style_card_lines(bible_text: str) -> list[str]:
    """Extract the book-bible `## Style card` block (POV / Voice notes / Anti-patterns)."""
    out: list[str] = []
    grab = False
    for ln in bible_text.splitlines():
        s = ln.strip()
        if s.startswith("## Style card"):
            grab = True
            continue
        if grab:
            if s.startswith("#"):
                break
            if s:
                out.append(ln)
    return out


def style_fingerprint_brief(book_root: Path) -> str:
    """Style brief injected at the top of preflight CONTEXT: canon/style-fingerprint.md,
    falling back to the book-bible Style card. Capped at STYLE_MAX_RUNES."""
    fp = book_root / "canon" / "style-fingerprint.md"
    if fp.is_file():
        lines = read_book_text(fp).strip().splitlines()
    else:
        bible = book_root / "book-bible.md"
        if not bible.is_file():
            return ""
        lines = _style_card_lines(read_book_text(bible))
    kept: list[str] = []
    total = 0
    for ln in lines:
        s = ln.strip()
        if not s:
            continue
        if s.startswith("## 参考章") or s.startswith("## 指纹摘要"):
            break
        total += rune_count(s)
        if total > STYLE_MAX_RUNES:
            break
        kept.append(ln)
    return "\n".join(kept).strip()


# --- previous hook ---


def previous_unit_hook(book_root: Path, unit: dict, cache=None) -> str:
    start, _end = chapter_range_of(unit)
    if start <= 1:
        return ""
    prev_ch = start - 1
    units_dir = book_root / "outline" / "units"
    if units_dir.is_dir():
        for path in sorted(units_dir.glob("*.yaml")):
            if path.stem == str(unit.get("unit_id") or "").strip():
                continue
            try:
                other = load_yaml_map(read_book_text(path))
            except OSError:
                continue
            a, b = chapter_range_of(other)
            if b == prev_ch:
                _, hout = next_hook_of(other)
                if hout:
                    return hout
    uid = str(unit.get("unit_id") or "").strip()
    vol = uid.split("-")[0] if "-" in uid else ""
    text = volume_summary_text(book_root, vol) if vol else ""
    block = extract_chapter_summary_block(text, prev_ch) if text else ""
    if not block:
        if cache is not None:
            legacy, _ = cache.summary_source()
        else:
            legacy, _ = summary_source_text(book_root)
        block = extract_chapter_summary_block(legacy, prev_ch)
    m = re.search(r"[-*]\s*钩子\s*[：:]\s*(.+)", block)
    return m.group(1).strip() if m else ""


# --- unit card (rendered from YAML) ---


def render_unit_card(unit: dict) -> list[str]:
    uid = str(unit.get("unit_id") or "").strip()
    a, b = chapter_range_of(unit)
    lines = [f"- 单元卡 {uid or '?'}（ch{a}–ch{b}）:"]
    for key, label in (
        ("function", "function"),
        ("entry", "entry"),
        ("desire", "desire"),
        ("obstacle", "obstacle"),
        ("choice", "choice"),
        ("payoff", "payoff"),
        ("pleasure", "pleasure"),
        ("endgame_boundary", "endgame_boundary"),
    ):
        v = str(unit.get(key) or "").strip()
        if v:
            lines.append(f"  {label}: {v}")
    forbidden = nonempty_list(unit.get("forbidden"))
    lines.append(f"  forbidden: {forbidden if forbidden else '[]'}")
    info = unit.get("info_control") or {}
    if isinstance(info, dict):
        reveals = nonempty_list(info.get("reveals"))
        fs = nonempty_list(info.get("foreshadowing"))
        if reveals:
            lines.append(f"  reveals: {'；'.join(reveals)}")
        if fs:
            lines.append(f"  foreshadowing: {'；'.join(fs)}")
    deltas = nonempty_list(unit.get("state_deltas"))
    if deltas:
        lines.append(f"  state_deltas: {'；'.join(deltas)}")
    stage = on_stage_of(unit)
    pov = pov_of(unit)
    lines.append(f"  on_stage: {stage if stage else '（未列上场人物）'}" + (f"  pov: {pov}" if pov else ""))
    lines.append("  场面序:")
    for s in scene_rows(unit):
        landed = s.get("must_land") or []
        facts = "；".join(str(x) for x in landed) if isinstance(landed, list) else str(landed)
        who = nonempty_list(s.get("who"))
        spov = str(s.get("pov") or "").strip()
        tag = ""
        if who:
            tag += f" who={','.join(who)}"
        if spov:
            tag += f" pov={spov}"
        lines.append(
            f"    {s.get('id') or '?'} ch{s.get('chapter')} {s.get('beat')}: {s.get('want') or ''} → {s.get('turn') or ''} | {facts}{tag}"
        )
    lines.append("  章切口:")
    for c in chapter_rows(unit):
        lines.append(
            f"    第{c.get('chapter')}章 {c.get('title_working') or ''} cut={c.get('cut_hook') or ''} share={c.get('word_share') or ''}"
        )
    _, hout = next_hook_of(unit)
    lines.append(f"  next_hook.out: {hout}")
    return lines


def _volume_index_line(row: dict | None, unit: dict) -> str:
    uid = str(unit.get("unit_id") or "").strip()
    if row is None:
        return f"- 卷纲索引行: （{volume_outline_rel(uid.split('-')[0]) if '-' in uid else '卷纲'} 无 {uid} 行）"
    a, b = row.get("chapter_range") or [0, 0]
    return (
        f"- 卷纲索引行: {row['unit_id']} | ch{a}–ch{b} | {row.get('function') or ''} | "
        f"钩子={row.get('next_hook_type') or ''} | 终局边界={row.get('endgame_boundary') or '—'}"
    )


def build_preflight_context(
    book_root: Path, unit: dict, r: Report, cache=None, st: dict | None = None,
    volume_row: dict | None = None,
) -> list[str]:
    lines: list[str] = []
    # 1. style fingerprint
    style = cache.style_brief() if cache is not None else style_fingerprint_brief(book_root)
    if style:
        lines.append("- 风格指纹（本书固定，写入时对齐 POV/语域/句式/禁语/章末钩）:")
        for ln in style.splitlines():
            lines.append(f"  {ln}")
    # 2. genre article(s), whole text
    articles = genre_articles(st)
    genre = genre_of(st)
    lane = craft_lane_of(st)
    if articles:
        for title, text in articles:
            lines.append(f"- 题材专有文「{title}」（整篇；写作只对照本篇 + 下方单元卡）:")
            for ln in text.splitlines():
                lines.append(f"  {ln}")
    elif genre:
        lines.append(f"- 题材专有文: （知识库无「{genre}」）")
        r.advisory("genre", f"knowledge article 「{genre}」 not found under {KB_REL}")
    else:
        lines.append("- 题材专有文: （novel-state.genre 未填，未注入题材篇）")
        r.advisory("genre", "novel-state.yaml genre empty — set one of " + " / ".join(GENRES))
    lines.append(f"- genre: {genre or '（空）'}  craft_lane: {lane}")
    # 3. volume index row
    lines.append(_volume_index_line(volume_row, unit))
    # 4. unit card
    lines.extend(render_unit_card(unit))
    # 5. previous hook
    prev_hook = previous_unit_hook(book_root, unit, cache)
    lines.append(f"- 接钩（上一单元）: {prev_hook or '（首单元或无上单元钩）'}")
    # 6. cast (on_stage only)
    if cache is not None:
        ledger_text = cache.ledger_text()
    else:
        lp = ledger_path(book_root)
        ledger_text = read_book_text(lp) if lp.is_file() else ""
    snap = cast_snapshot_rows(ledger_text)
    stage = on_stage_of(unit)
    pov = pov_of(unit)
    lines.append("- 人物（仅 on_stage；snapshot + 三锚点 + 1 条台词；POV 加不知）:")
    if stage:
        lines.extend(cast_context_lines(book_root, stage, pov, snap, cache))
    else:
        lines.append("  （未列上场人物 — 细纲 on_stage 为空）")
    if not snap:
        lines.append("  （facts.md 无 Cast snapshot 表）")
    # 7. open debts
    loops = open_loops_rows(ledger_text)
    lines.append("- 开放债务:")
    if loops:
        for row in loops[:8]:
            lines.append(f"  {row}")
    else:
        lines.append("  （无 open loops 行）")
    # 8. locked terms
    uid = str(unit.get("unit_id") or "").strip()
    try:
        cur_vol = uid.split("-")[0] if "-" in uid else ""
        locked_map, compliance, _ = load_locked_terms(book_root)
        locked_terms_list = []
        cv = volume_number(cur_vol)
        for vol_tag, terms in locked_map.items():
            if not re.search(r"\d", str(vol_tag)):
                continue
            if volume_number(vol_tag) > cv:
                locked_terms_list.extend(nonempty_list(terms))
        if locked_terms_list or compliance:
            lines.append("- 本单元锁词（precommit 硬扫描；正文不得出现字面）:")
            if locked_terms_list:
                lines.append("  未解锁: " + " / ".join(locked_terms_list[:20]))
            if compliance:
                lines.append("  永久合规: " + " / ".join(compliance[:15]))
    except Exception:
        pass
    # 9. loading discipline
    lines.append(
        "- 加载纪律: 只消费本 CONTEXT；不再读人物卡 / 卷纲 / 账本；禁止扫树；禁止 author-lore；禁止 chapters/。"
        "至多一次 search_kb（仅含 ch1–3 查「节奏与结构」）。"
    )
    return lines


# --- preflight ---


def asset_gate(book_root: Path, stage: list[str], cache=None) -> list[str]:
    """Blocking messages for the asset gate: on_stage all canon and a canon protagonist exists."""
    from .cast import load_cast_cards

    cards = load_cast_cards(book_root, cache)
    msgs: list[str] = []
    if not any(c.role == "protagonist" and c.status == "canon" for c in cards.values()):
        msgs.append("no canon cast card with `role`: protagonist — run accept-volume or fix the card")
    for stem in stage:
        c = cards.get(stem)
        if c is not None and c.status != "canon":
            msgs.append(f"on_stage {stem} is {c.status}")
    return msgs


def check_preflight(book_root: Path, st: dict, unit_id: str, r: Report, cache=None) -> None:
    if not UNIT_ID_RE.match(unit_id or ""):
        r.blocking("unit", f"--unit {unit_id!r} must match vNN-U#")
        return
    if not file_exists(book_root, "novel-state.yaml"):
        r.blocking("state", "missing novel-state.yaml")
        return
    validate_state_fields(st, r)
    legacy = legacy_chapters_message(book_root)
    if legacy:
        r.blocking("migrate", legacy)
        return
    if not has_reader_continuity(book_root):
        r.blocking(
            "lore-tracks",
            "missing continuity/facts.md (or legacy ledger.md / public-lore+tracking) — draft from facts only",
        )
    if file_exists(book_root, "canon/author-lore.md"):
        r.advisory("lore-tracks", "do not load canon/author-lore.md into the draft context")
    elif writing_stage(str(st.get("stage") or "")):
        r.blocking("lore-tracks", "missing canon/author-lore.md")
    try:
        u, rel = load_unit(book_root, unit_id)
    except OSError as e:
        r.blocking("contract", f"{unit_outline_rel(unit_id)}: {e}")
        return
    filed = str(u.get("unit_id") or "").strip()
    if filed and filed != unit_id:
        r.blocking("unit_id", f"{rel} unit_id={filed} want {unit_id}")
    status = str(u.get("status") or "").strip()
    if status not in ("accepted", "drafted"):
        r.blocking("contract", f"{rel} status={status} (need accepted before prose)")
    if filed and UNIT_ID_RE.match(filed) and not unit_listed(book_root / "outline", filed, cache):
        r.blocking("unit_id", f"{filed} not found in outline/ — return to novel-plan")
    validate_unit_shape(u, r, rel)
    row = volume_index_row(book_root, unit_id, cache)
    validate_unit_against_volume(u, row, r, rel)
    validate_on_stage(u, volume_cast_for(book_root, unit_id, cache), cast_status_map(book_root, cache), r, rel)
    for msg in asset_gate(book_root, on_stage_of(u), cache):
        r.blocking("asset", msg)
    if tomato_profile(str(st.get("qc_profile") or "")) and is_blank(u.get("pleasure")):
        r.blocking("pleasure", f"qc_profile={st.get('qc_profile')} requires pleasure")
    debts = open_debt_count(book_root, u, cache)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")
    r.context_lines = build_preflight_context(book_root, u, r, cache, st, row)


# --- KB citation validator (docs hygiene) ---


_CITE_BLOCK = re.compile(r"(?:见|改查|才查)((?:\s*「[^」]+」)+)")
_CITE_QUOTE = re.compile(r"「([^」]+)」")
_MD_HEADING = re.compile(r"^#{1,3}\s+(.+?)\s*$")


def _strip_heading_note(title: str) -> str:
    title = re.sub(r"（[^）]*）", "", title)
    title = re.sub(r"\([^)]*\)", "", title)
    return re.sub(r"\s+", "", title)


def _md_lines_outside_fences(text: str) -> list[str]:
    out: list[str] = []
    fence = False
    for line in text.splitlines():
        if line.strip().startswith("```"):
            fence = not fence
            continue
        if not fence:
            out.append(line)
    return out


def _is_knowledge_path(path: str) -> bool:
    norm = path.replace("\\", "/")
    return norm.startswith("knowledge/") or "/knowledge/" in norm


def _section_matches(section: str, headings: list[str]) -> bool:
    want = _strip_heading_note(section)
    if len(want) < 2:
        return False
    for heading in headings:
        key = _strip_heading_note(heading)
        if key == want or key.startswith(want):
            return True
    return False


def kb_cite_errors(files: dict[str, str]) -> list[str]:
    """Dangling 见/改查/才查 citations against knowledge H1 titles.

    `「标题」` must be a knowledge article title. `「标题 → 小节」` must
    name a heading in that article. A quote that is not a title may still
    be a heading in the same file. Parenthetical heading notes are ignored.
    """
    articles: dict[str, list[str]] = {}
    for path, text in files.items():
        if not _is_knowledge_path(path):
            continue
        headings: list[str] = []
        title = ""
        for line in _md_lines_outside_fences(text):
            match = _MD_HEADING.match(line)
            if not match:
                continue
            heading = match.group(1).strip()
            headings.append(heading)
            if line.startswith("# ") and not title:
                title = heading
        if title:
            articles[title] = headings

    errors: list[str] = []
    for path in sorted(files):
        lines = _md_lines_outside_fences(files[path])
        own = [m.group(1).strip() for line in lines if (m := _MD_HEADING.match(line))]
        body = "\n".join(lines)
        for block in _CITE_BLOCK.finditer(body):
            for quote in _CITE_QUOTE.findall(block.group(1)):
                message = _cite_error(quote, articles, own)
                if message:
                    errors.append(f"{path}: {message}")
    return errors


def _cite_error(quote: str, articles: dict[str, list[str]], own: list[str]) -> str:
    if "→" in quote:
        title, _, section = quote.partition("→")
    elif "->" in quote:
        title, _, section = quote.partition("->")
    else:
        title, section = quote, ""
    title, section = title.strip(), section.strip()
    if title in articles:
        if section and not _section_matches(section, articles[title]):
            return f"「{quote}」 section not in 「{title}」"
        return ""
    if not section and _section_matches(title, own):
        return ""
    return f"「{quote}」 is not a knowledge title"


def plugin_kb_cite_errors() -> list[str]:
    root = plugin_root()
    files: dict[str, str] = {}
    for rel in (KB_REL, "ai.danmo.work/experts", "skills"):
        folder = root / rel
        if not folder.is_dir():
            continue
        for path in folder.rglob("*.md"):
            files[path.relative_to(root).as_posix()] = path.read_text(encoding="utf-8")
    if not any(_is_knowledge_path(path) for path in files):
        raise FileNotFoundError(f"knowledge dir missing under {root}")
    return kb_cite_errors(files)

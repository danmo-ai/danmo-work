"""Preflight CONTEXT + thin outline-pack. Preflight is the only hard prose injection;
outline-pack is a slim volume pack for filling proposed unit YAML."""
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
GENRE_MAX_RUNES = 1200  # truncate genre KB articles in preflight
QC_PROFILES = {"male_power", "female_emotion", "mystery", "general"}
TIME_SYSTEMS = {"relative_days", "calendar"}
GENRES = ("玄幻", "仙侠", "都市", "悬疑", "现代言情", "古代言情", "仕途扫黑", "系统穿越")
# Closed set from knowledge「题材与平台」. One subgenre per genre.
SUBGENRES: dict[str, tuple[str, ...]] = {
    "玄幻": ("传统玄幻", "玄幻脑洞", "西方奇幻"),
    "仙侠": ("古典仙侠", "凡人修仙", "洪荒神话", "武侠", "现代修真"),
    "都市": ("都市日常", "战神赘婿", "都市高武", "都市种田", "娱乐圈"),
    "悬疑": ("刑侦探案", "规则怪谈", "悬疑灵异", "悬疑脑洞", "女频悬疑"),
    "现代言情": ("青春甜宠", "豪门总裁", "职场婚恋", "年代", "种田", "现言脑洞", "星光璀璨"),
    "古代言情": ("宫斗宅斗", "古风世情", "古言脑洞", "玄幻言情", "民国言情"),
    "仕途扫黑": ("官场", "扫黑"),
    "系统穿越": ("都市脑洞", "玄幻系统", "历史脑洞", "科幻末世", "快穿", "游戏", "诸天无限", "悬疑副本"),
}
CRIME_FLAVOR_ARTICLE = "刑侦人味文风"
KB_REL = "ai.danmo.work/knowledge"


# --- state ---


def genre_of(st: dict | None) -> str:
    if not st:
        return ""
    return str(st.get("genre") or "").strip()


def subgenre_of(st: dict | None) -> str:
    if not st:
        return ""
    return str(st.get("subgenre") or "").strip()


def validate_state_fields(st: dict, r: Report) -> None:
    qc = str(st.get("qc_profile") or "").strip()
    if qc and qc not in QC_PROFILES:
        r.blocking("state", f"qc_profile={qc} not in {sorted(QC_PROFILES)}")
    ts = str(st.get("time_system") or "").strip()
    if ts and ts not in TIME_SYSTEMS:
        r.blocking("state", f"time_system={ts} not in {sorted(TIME_SYSTEMS)}")
    genre = genre_of(st)
    if genre and genre not in GENRES:
        r.blocking("state", f"genre={genre} not in {list(GENRES)}")
    sub = subgenre_of(st)
    if sub:
        allowed = SUBGENRES.get(genre, ())
        if sub not in allowed:
            r.blocking("state", f"subgenre={sub} not in genre={genre or '（空）'} {list(allowed)}")


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
    """[(title, text)] to inject: the genre article, plus 刑侦人味文风 when subgenre is 刑侦探案."""
    out: list[tuple[str, str]] = []
    genre = genre_of(st)
    if genre in GENRES:
        text = knowledge_article(genre)
        if text:
            out.append((genre, text))
    if genre == "悬疑" and subgenre_of(st) == "刑侦探案":
        text = knowledge_article(CRIME_FLAVOR_ARTICLE)
        if text:
            out.append((CRIME_FLAVOR_ARTICLE, text))
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


def truncate_article(text: str, max_runes: int = GENRE_MAX_RUNES) -> tuple[str, bool]:
    """Keep H1 + leading body up to max_runes. Returns (text, truncated)."""
    if not text:
        return "", False
    if rune_count(text) <= max_runes:
        return text.strip(), False
    kept: list[str] = []
    total = 0
    for ln in text.splitlines():
        n = rune_count(ln) + 1
        if total + n > max_runes and kept:
            break
        kept.append(ln)
        total += n
    return "\n".join(kept).strip(), True


def volume_timeline_brief(book_root: Path, unit_id: str, cache=None) -> str:
    """One-line summary of 卷纲「本卷时间线」section."""
    vol = unit_id.split("-")[0] if "-" in unit_id else ""
    if not vol:
        return "（无卷号）"
    path = book_root / volume_outline_rel(vol)
    text = ""
    if cache is not None:
        for p, t in cache.outline_texts():
            if p.name == f"{vol}.md" or p.as_posix().endswith(f"volumes/{vol}.md"):
                text = t
                break
    if not text and path.is_file():
        text = read_book_text(path)
    if not text:
        return "（卷纲缺失）"
    grab = False
    bits: list[str] = []
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("#"):
            if "本卷时间线" in s:
                grab = True
                continue
            if grab:
                break
            continue
        if not grab:
            continue
        if s.startswith("-") and ("：" in s or ":" in s):
            raw = s.lstrip("-* ")
            if "：" in raw:
                label, _, val = raw.partition("：")
            else:
                label, _, val = raw.partition(":")
            val = val.strip()
            if val and val not in ("无",):
                bits.append(f"{label.strip()}={val}")
        if len(bits) >= 4:
            break
    return "；".join(bits) if bits else "（未填）"


def render_unit_card(unit: dict, *, include_clock: bool = False, include_deltas: bool = False) -> list[str]:
    """Unit contract for CONTEXT. Clock/deltas default off (printed as separate blocks)."""
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
    if include_deltas:
        from .identity import format_delta_line, normalize_state_deltas

        norm = normalize_state_deltas(unit.get("state_deltas"))
        if norm:
            lines.append("  单元结束身份目标（勿当开场人设；正文结束须落到 to）:")
            for row in norm:
                lines.append(f"    - {format_delta_line(row)}")
    stage = on_stage_of(unit)
    pov = pov_of(unit)
    if include_clock:
        from .identity import format_unit_clock

        clock = format_unit_clock(unit)
        if clock != "（无时钟字段）":
            lines.append(f"  时钟: {clock}")
    lines.append(f"  on_stage: {stage if stage else '（未列上场人物）'}" + (f"  pov: {pov}" if pov else ""))
    lines.append("  场面序:")
    for s in scene_rows(unit):
        landed = s.get("must_land") or []
        facts = "；".join(str(x) for x in landed) if isinstance(landed, list) else str(landed)
        who = nonempty_list(s.get("who"))
        spov = str(s.get("pov") or "").strip()
        when = str(s.get("when") or "").strip()
        where = str(s.get("where") or "").strip()
        tag = ""
        if when:
            tag += f" when={when}"
        if where:
            tag += f" where={where}"
        if who:
            tag += f" who={','.join(who)}"
        if spov:
            tag += f" pov={spov}"
        emo = str(s.get("emotional_beat") or "").strip()
        effect = str(s.get("reader_effect") or "").strip()
        sub = str(s.get("subtext") or "").strip()
        contract = ""
        if emo or effect or sub:
            contract = f" 情:{emo or '—'} →读:{effect or '—'} | 潜:{sub or '—'}"
        lines.append(
            f"    {s.get('id') or '?'} ch{s.get('chapter')} {s.get('beat')}: "
            f"{s.get('want') or ''} → {s.get('turn') or ''} | {facts}{tag}{contract}"
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


def build_continuity_lines(
    book_root: Path, unit: dict, cache=None, st: dict | None = None,
) -> list[str]:
    """Compact continuity pack for qc-pack ### CONTINUITY (no genre article)."""
    from .cast import load_cast_cards
    from .identity import (
        deltas_end_goals_lines,
        format_identity_line,
        format_unit_clock,
        identity_from_snapshot_and_baseline,
        parse_cast_baseline,
        parse_cast_snapshot_table,
        previous_unit_dict,
    )

    lines: list[str] = []
    uid = str(unit.get("unit_id") or "").strip()
    lines.append(f"unit: {uid or '?'}")
    lines.append(f"时钟: {format_unit_clock(unit)}")
    prev_id, prev_u = previous_unit_dict(book_root, unit)
    if prev_u is not None:
        lines.append(f"上一单元时钟: {prev_id} {format_unit_clock(prev_u)}")
    else:
        lines.append("上一单元时钟: （首单元或无前序）")
    prev_hook = previous_unit_hook(book_root, unit, cache)
    lines.append(f"接钩: {prev_hook or '（无）'}")
    if cache is not None:
        ledger_text = cache.ledger_text()
    else:
        lp = ledger_path(book_root)
        ledger_text = read_book_text(lp) if lp.is_file() else ""
    snap_table = parse_cast_snapshot_table(ledger_text)
    cards = load_cast_cards(book_root, cache)
    stage = on_stage_of(unit)
    lines.append("开场期望身份 (identity@unit):")
    if stage:
        for name in stage:
            card = cards.get(name)
            if card is None:
                lines.append(f"  - {name}: （无人物卡）")
                continue
            baseline = {"age": card.age_at_start, "title": card.title_baseline}
            if not baseline["age"] and not baseline["title"]:
                baseline = parse_cast_baseline(card.text)
            ident = identity_from_snapshot_and_baseline(name, card.label, snap_table, baseline)
            lines.append(f"  - {card.label}（{name}）: {format_identity_line(ident)}")
    else:
        lines.append("  （on_stage 空）")
    goals = deltas_end_goals_lines(unit.get("state_deltas"))
    lines.append("单元结束身份目标 (state_deltas.to；勿开场写穿):")
    if goals:
        for g in goals:
            lines.append(f"  - {g}")
    else:
        lines.append("  （无）")
    info = unit.get("info_control") or {}
    fs_items = nonempty_list(info.get("foreshadowing")) if isinstance(info, dict) else []
    loop_rows = open_loops_rows(ledger_text)
    lines.append("本单元伏笔 ↔ Open loops:")
    if fs_items:
        for item in fs_items:
            fs_ids = re.findall(r"FS-\d+", str(item), flags=re.I)
            matched = []
            for fs in fs_ids:
                hit = next((row for row in loop_rows if fs.upper() in row.upper()), "")
                matched.append(f"{fs.upper()}={'在表' if hit else '缺失'}")
            lines.append(f"  - {item}" + (f" → {', '.join(matched)}" if matched else ""))
    else:
        lines.append("  （细纲无 foreshadowing）")
    return lines


def build_preflight_context(
    book_root: Path, unit: dict, r: Report, cache=None, st: dict | None = None,
    volume_row: dict | None = None,
) -> list[str]:
    """State-first CONTEXT: clock/identity before truncated genre articles."""
    from .identity import (
        deltas_end_goals_lines,
        format_unit_clock,
        previous_unit_dict,
    )

    lines: list[str] = []
    genre = genre_of(st)
    sub = subgenre_of(st)
    ts = str((st or {}).get("time_system") or "relative_days").strip() or "relative_days"
    active = str((st or {}).get("active_unit") or "").strip()
    uid = str(unit.get("unit_id") or "").strip()
    # 1. book one-liner
    lines.append(
        f"- 书级: genre={genre or '（空）'}  subgenre={sub or '（空）'}  "
        f"time_system={ts}  active_unit={active or uid or '（空）'}"
    )
    # 2. volume index + timeline brief
    lines.append(_volume_index_line(volume_row, unit))
    lines.append(f"- 本卷时间线: {volume_timeline_brief(book_root, uid, cache)}")
    # 3. clock (this + previous)
    lines.append(f"- 本单元时钟: {format_unit_clock(unit)}")
    prev_id, prev_u = previous_unit_dict(book_root, unit)
    if prev_u is not None:
        lines.append(f"- 上一单元时钟: {prev_id} {format_unit_clock(prev_u)}")
    else:
        lines.append("- 上一单元时钟: （首单元或无前序）")
    # 4. unit card (no embedded deltas/clock)
    lines.extend(render_unit_card(unit, include_clock=False, include_deltas=False))
    # 5. previous hook
    prev_hook = previous_unit_hook(book_root, unit, cache)
    lines.append(f"- 接钩（上一单元）: {prev_hook or '（首单元或无上单元钩）'}")
    # 6. cast identity only
    if cache is not None:
        ledger_text = cache.ledger_text()
    else:
        lp = ledger_path(book_root)
        ledger_text = read_book_text(lp) if lp.is_file() else ""
    snap = cast_snapshot_rows(ledger_text)
    stage = on_stage_of(unit)
    pov = pov_of(unit)
    lines.append("- 人物（仅 on_stage；identity@unit + 三锚点 + 1 条台词；POV 加不知）:")
    if stage:
        lines.extend(cast_context_lines(book_root, stage, pov, snap, cache, ledger_text=ledger_text))
    else:
        lines.append("  （未列上场人物 — 细纲 on_stage 为空）")
    if not snap:
        lines.append("  （facts.md 无 Cast snapshot 表）")
    # 7. single identity end-goals block
    goals = deltas_end_goals_lines(unit.get("state_deltas"))
    if goals:
        lines.append("- 本单元身份转变目标（写作约束；开场人设已在 identity@unit，勿提前写穿 to）:")
        for g in goals:
            lines.append(f"  - {g}")
    # 8. open debts
    loops = open_loops_rows(ledger_text)
    lines.append("- 开放债务:")
    if loops:
        for row in loops[:8]:
            lines.append(f"  {row}")
    else:
        lines.append("  （无 open loops 行）")
    # 9. locked terms
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
    # 10. truncated genre articles
    articles = genre_articles(st)
    if articles:
        for title, text in articles:
            kind = "子类专有文" if title == CRIME_FLAVOR_ARTICLE else "题材专有文"
            body, truncated = truncate_article(text, GENRE_MAX_RUNES)
            note = f"（截断至约 {GENRE_MAX_RUNES} 字；全文见 KB「{title}」）" if truncated else "（摘要注入）"
            lines.append(f"- {kind}「{title}」{note}:")
            for ln in body.splitlines():
                lines.append(f"  {ln}")
    elif genre:
        lines.append(f"- 题材专有文: （知识库无「{genre}」）")
        r.advisory("genre", f"knowledge article 「{genre}」 not found under {KB_REL}")
    else:
        lines.append("- 题材专有文: （novel-state.genre 未填，未注入题材篇）")
        r.advisory("genre", "novel-state.yaml genre empty — set one of " + " / ".join(GENRES))
    # 11. style fingerprint
    style = cache.style_brief() if cache is not None else style_fingerprint_brief(book_root)
    if style:
        lines.append("- 风格指纹（本书固定，写入时对齐 POV/语域/句式/禁语/章末钩）:")
        for ln in style.splitlines():
            lines.append(f"  {ln}")
    # 12. loading discipline
    lines.append(
        "- 加载纪律: 只消费本 CONTEXT；不再读人物卡 / 卷纲 / 账本；禁止扫树；禁止 author-lore；禁止 chapters/。"
        "至多一次 search_kb（仅含 ch1–3 查「节奏与结构」；题材全文不足时才查 KB 同名篇）。"
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
    from .identity import check_timeline_monotonic

    check_timeline_monotonic(book_root, u, r, unit_outline_rel(unit_id))
    for msg in asset_gate(book_root, on_stage_of(u), cache):
        r.blocking("asset", msg)
    if tomato_profile(str(st.get("qc_profile") or "")) and is_blank(u.get("pleasure")):
        r.blocking("pleasure", f"qc_profile={st.get('qc_profile')} requires pleasure")
    debts = open_debt_count(book_root, u, cache)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")
    r.context_lines = build_preflight_context(book_root, u, r, cache, st, row)


# --- outline-pack (thin; 一批细纲写前) ---


def build_outline_pack(book_root: Path, st: dict, volume: str, cache=None) -> list[str]:
    """Thin pack for filling proposed unit YAML — not full preflight CONTEXT.

    上一钩 / Cast snapshot / open loops / 本卷时间线 / 本卷人物 / proposed 队列.
    No genre article, no style fingerprint, no rendered unit card.
    """
    from .cast import load_cast_cards
    from .identity import (
        format_identity_line,
        format_unit_clock,
        identity_from_snapshot_and_baseline,
        parse_cast_snapshot_table,
        previous_unit_dict,
    )
    from .outline import (
        _volume_text,
        list_unit_ids,
        parse_volume_cast,
        parse_volume_index,
    )

    lines: list[str] = []
    genre = genre_of(st)
    sub = subgenre_of(st)
    ts = str((st or {}).get("time_system") or "relative_days").strip() or "relative_days"
    lines.append(
        f"- 书级: genre={genre or '（空）'}  subgenre={sub or '（空）'}  time_system={ts}  volume={volume}"
    )
    lines.append(f"- 本卷时间线: {volume_timeline_brief(book_root, f'{volume}-U1', cache)}")

    text = _volume_text(book_root, volume, cache)
    cast_stems = parse_volume_cast(text) if text and "本卷人物" in text else []
    lines.append(
        "- 本卷人物: " + (", ".join(cast_stems) if cast_stems else "（卷纲未列 — 先补 ## 本卷人物）")
    )

    if cache is not None:
        ledger_text = cache.ledger_text()
    else:
        lp = ledger_path(book_root)
        ledger_text = read_book_text(lp) if lp.is_file() else ""
    snap_table = parse_cast_snapshot_table(ledger_text)
    cards = load_cast_cards(book_root, cache)
    lines.append("- Cast snapshot（填 state_deltas.from / 开场身份对齐；无表用卡基线）:")
    names = cast_stems or list(snap_table.keys())
    if names:
        for name in names:
            card = cards.get(name)
            label = card.label if card else name
            baseline = {"age": "", "title": ""}
            if card is not None:
                baseline = {"age": card.age_at_start, "title": card.title_baseline}
            ident = identity_from_snapshot_and_baseline(name, label, snap_table, baseline)
            lines.append(f"  - {label}（{name}）: {format_identity_line(ident)}")
    else:
        lines.append("  （无本卷人物且无 Cast snapshot）")

    loops = open_loops_rows(ledger_text)
    lines.append("- 开放债务:")
    if loops:
        for row in loops[:8]:
            lines.append(f"  {row}")
    else:
        lines.append("  （无 open loops 行）")

    try:
        locked_map, compliance, _ = load_locked_terms(book_root)
        locked_terms_list = []
        cv = volume_number(volume)
        for vol_tag, terms in locked_map.items():
            if not re.search(r"\d", str(vol_tag)):
                continue
            if volume_number(vol_tag) > cv:
                locked_terms_list.extend(nonempty_list(terms))
        if locked_terms_list or compliance:
            lines.append("- 本卷锁词（写入 forbidden；正文 precommit 硬扫）:")
            if locked_terms_list:
                lines.append("  未解锁: " + " / ".join(locked_terms_list[:20]))
            if compliance:
                lines.append("  永久合规: " + " / ".join(compliance[:15]))
    except Exception:
        pass

    index_rows = parse_volume_index(text, volume) if text else []
    by_id = {row["unit_id"]: row for row in index_rows}
    unit_ids = [u for u in list_unit_ids(book_root, "outline") if u.startswith(volume + "-")]
    proposed: list[tuple[str, dict]] = []
    for uid in unit_ids:
        try:
            u, _ = load_unit(book_root, uid)
        except OSError:
            continue
        if str(u.get("status") or "").strip() == "proposed":
            proposed.append((uid, u))
    batch = proposed[:4]
    lines.append(f"- 待填细纲 proposed（本批 ≤4，共 {len(proposed)}）:")
    if not batch:
        lines.append("  （无 proposed — 本卷细纲已填完或未 accept-volume）")
    for uid, u in batch:
        row = by_id.get(uid) or {}
        a, b = chapter_range_of(u)
        if a <= 0 and row.get("chapter_range"):
            cr = row["chapter_range"]
            if isinstance(cr, (list, tuple)) and len(cr) >= 2:
                a, b = int(cr[0] or 0), int(cr[1] or 0)
        fn = str(u.get("function") or row.get("function") or "").strip()
        hook = ""
        nh = u.get("next_hook") if isinstance(u.get("next_hook"), dict) else {}
        if isinstance(nh, dict):
            hook = str(nh.get("type") or "").strip()
        if not hook:
            hook = str(row.get("next_hook_type") or "").strip()
        endb = str(u.get("endgame_boundary") or row.get("endgame_boundary") or "").strip()
        lines.append(
            f"  - {uid} | ch{a}–ch{b} | {fn or '（功能空）'} | 钩={hook or '（空）'} | 终局={endb or '（空）'}"
        )
        prev_hook = previous_unit_hook(book_root, u, cache)
        lines.append(f"    接钩→entry: {prev_hook or '（首单元或无上单元钩）'}")
        prev_id, prev_u = previous_unit_dict(book_root, u)
        if prev_u is not None:
            lines.append(f"    上一单元时钟: {prev_id} {format_unit_clock(prev_u)}")
        else:
            lines.append("    上一单元时钟: （首单元或无前序 — gap 写「开卷」）")

    lines.append(
        "- 纪律: 只用本包填时钟 / state_deltas.from / entry / forbidden / on_stage；"
        "不读题材全文、不读人物卡、不 read_skill。"
        "填完 lint-units 一次；FAIL 只补一次再 lint 一次然后停。不写正文。"
    )
    return lines


def check_outline_pack(book_root: Path, st: dict, volume: str, r: Report, cache=None) -> None:
    """Volume-level thin injection before 一批细纲. Does not gate writing."""
    from .common import VOLUME_ID_RE
    from .outline import volume_outline_rel

    vol = (volume or "").strip()
    if not vol or not VOLUME_ID_RE.match(vol):
        r.blocking("volume", f"--volume {volume!r} must match vNN")
        return
    if not file_exists(book_root, volume_outline_rel(vol)):
        r.blocking("volume", f"{volume_outline_rel(vol)} missing — approve volume + accept-volume first")
        return
    validate_state_fields(st, r)
    lines = build_outline_pack(book_root, st, vol, cache)
    r.section("OUTLINE_PACK", lines)


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

"""Anti-AI prose scan: toxic patterns, level-1 words, banned phrases, English leaks,
em-dash density, simile density, chicken-soup endings. Single source of truth (KB 05 mirrors)."""
from __future__ import annotations

import re
from dataclasses import dataclass

from .common import Report, last_runes, rune_count

# P0 毒句式：网文常见 tic + 翻案腔变体（后者借鉴 lieflat 实测：不是/而是 0.70/千字）。
# 不收「不是 A，是 B」（对白误杀高）、「表面/看似」（叙事正当用法多）——见 KB 05 P1。
# 「是……的。」强调框架：把整句裹进是/的，时间状语别扭、句尾拖「的。」→ 还原自然陈述（例：是晚上十点多才到所里的 → 晚上十点多才到所里）。
# 中间至少 3 字，避开「是的。」「是真的。」「是他干的。」等过短对白；(?<!不) 避开「不是…的」。
TOXIC = [
    re.compile(r"不是.{1,20}而是"),
    re.compile(r"并非.{1,20}而是"),
    re.compile(r"不在于.{1,20}而在于"),
    re.compile(r"与其说.{1,20}不如说"),
    re.compile(r"带着一丝|带着一抹"),
    re.compile(r"声音不大[，,]却带着"),
    re.compile(r"(他|她)知道|终于明白"),
    re.compile(r"眼中闪过|嘴角勾起|心中涌起"),
    re.compile(r"他不知道的是"),
    re.compile(r"(?<!不)是.{3,40}的[。！？]"),
]
LEVEL_ONE = [
    "仿佛", "犹如", "宛若", "不禁", "深吸一口气", "目光深邃",
    "不容置疑", "前所未有", "话锋一转", "心中暗道", "指节泛白", "瞳孔微缩",
]
SOUP_ENDINGS = [
    "或许，这只是个开始", "或许这只是个开始",
    "反击才刚刚开始", "这只是个开始",
]
BANNED_PHRASES = [
    "嘴角微微上扬", "勾起一抹弧度", "空气仿佛凝固", "气氛一时之间",
    "不由自主", "目光深邃", "目光锐利", "声音沙哑", "声音低沉",
    "微微", "某种",
]
SIMILE_WORDS = ["像是", "仿佛", "好像"]
MAX_SIMILES_PER_CH = 8
MAX_SIMILES_PER_PARA = 2
EM_DASH_TOKEN = "——"
MAX_EM_DASH_PER_1K = 5
ENGLISH_LEAK_RE = re.compile(r"[A-Za-z]{2,}")
ENGLISH_WHITELIST = {
    "OK", "APP", "WiFi", "WIFI", "VIP", "CEO", "CFO", "CTO", "NPC", "HP", "MP",
    "GPS", "AI", "ID", "TV", "KTV", "DNA", "IQ", "EQ", "UFO", "CBD", "LED",
    "USB", "PDF", "PPT", "VS", "SPA", "KPI", "NBA", "CBA", "SUV", "MV", "BGM",
}


@dataclass(frozen=True)
class DeslopHit:
    kind: str  # toxic | level1 | soup | banned | english | emdash | simile
    line: int
    col: int
    match: str
    rule: str


def offset_to_line_col(text: str, offset: int) -> tuple[int, int]:
    before = text[: max(0, offset)]
    line = before.count("\n") + 1
    last_nl = before.rfind("\n")
    col = offset - last_nl if last_nl >= 0 else offset + 1
    return line, col


def _excerpt(line: str, col: int, width: int = 40) -> str:
    s = line.strip()
    if len(s) <= width:
        return s
    start = max(0, col - 1 - width // 3)
    chunk = s[start : start + width]
    return ("…" if start > 0 else "") + chunk + ("…" if start + width < len(s) else "")


def iter_deslop_hits(prose: str) -> list[DeslopHit]:
    """Locate P0 deslop hits with 1-based line/col (same rules as precommit)."""
    hits: list[DeslopHit] = []
    lines = prose.splitlines()
    for i, line in enumerate(lines, start=1):
        for pat in TOXIC:
            for m in pat.finditer(line):
                hits.append(DeslopHit("toxic", i, m.start() + 1, m.group(0), pat.pattern))
        for w in LEVEL_ONE:
            start = 0
            while True:
                j = line.find(w, start)
                if j < 0:
                    break
                hits.append(DeslopHit("level1", i, j + 1, w, w))
                start = j + len(w)
        for w in BANNED_PHRASES:
            start = 0
            while True:
                j = line.find(w, start)
                if j < 0:
                    break
                hits.append(DeslopHit("banned", i, j + 1, w, w))
                start = j + len(w)
        for m in ENGLISH_LEAK_RE.finditer(line):
            tok = m.group(0)
            if tok in ENGLISH_WHITELIST or tok.upper() in ENGLISH_WHITELIST:
                continue
            hits.append(DeslopHit("english", i, m.start() + 1, tok, "english-leak"))
        for w in SIMILE_WORDS:
            start = 0
            while True:
                j = line.find(w, start)
                if j < 0:
                    break
                hits.append(DeslopHit("simile", i, j + 1, w, w))
                start = j + len(w)
        start = 0
        while True:
            j = line.find(EM_DASH_TOKEN, start)
            if j < 0:
                break
            hits.append(DeslopHit("emdash", i, j + 1, EM_DASH_TOKEN, "em-dash"))
            start = j + len(EM_DASH_TOKEN)

    tail = last_runes(prose, 200)
    tail_start = len(prose) - len(tail)
    for x in SOUP_ENDINGS:
        rel = prose.find(x, tail_start)
        if rel < 0:
            continue
        line, col = offset_to_line_col(prose, rel)
        hits.append(DeslopHit("soup", line, col, x, x))
    return hits


def scan_deslop(prose: str) -> tuple[list[str], int, bool]:
    hits = iter_deslop_hits(prose)
    toxic = list(dict.fromkeys(h.rule for h in hits if h.kind == "toxic"))
    n1 = sum(1 for h in hits if h.kind == "level1")
    soup = any(h.kind == "soup" for h in hits)
    return toxic, n1, soup


def format_hit_line(rel: str, hit: DeslopHit, prose_line: str = "") -> str:
    kind_label = {
        "toxic": "毒句式", "level1": "一级词", "soup": "鸡汤尾",
        "banned": "禁词", "english": "英文泄漏", "emdash": "破折号", "simile": "比喻词",
    }.get(hit.kind, hit.kind)
    excerpt = _excerpt(prose_line, hit.col) if prose_line else hit.match
    return f"{rel}:L{hit.line}: {kind_label}「{hit.match}」 | {excerpt}"


def format_deslop_locs(rel: str, hits: list[DeslopHit], limit: int = 5) -> str:
    if not hits:
        return ""
    return ", ".join(f"{rel}:L{h.line}" for h in hits[:limit])


def apply_deslop_to_report(prose_rel: str, prose: str, r: Report, hint_limit: int = 5) -> list[DeslopHit]:
    hits = iter_deslop_hits(prose)
    toxic = [h for h in hits if h.kind == "toxic"]
    level1 = [h for h in hits if h.kind == "level1"]
    soup = [h for h in hits if h.kind == "soup"]
    banned = [h for h in hits if h.kind == "banned"]
    english = [h for h in hits if h.kind == "english"]
    emdash = [h for h in hits if h.kind == "emdash"]
    simile = [h for h in hits if h.kind == "simile"]
    # 自检四计数（KB 05：审稿/润色报告必须引用）
    r.add_counts(
        em_dash_count=len(emdash),
        ai_vocab_count=len(toxic) + len(level1) + len(banned),
        english_leak_count=len(english),
        simile_count=len(simile),
    )
    if toxic:
        uniq = list(dict.fromkeys(h.rule for h in toxic))
        locs = format_deslop_locs(prose_rel, toxic, hint_limit)
        r.blocking("deslop", f"P0 毒句式 ×{len(uniq)} in {prose_rel}" + (f" @ {locs}" if locs else ""))
    if banned:
        uniq = list(dict.fromkeys(h.match for h in banned))
        locs = format_deslop_locs(prose_rel, banned, hint_limit)
        r.blocking(
            "deslop",
            f"禁词表命中 ×{len(banned)} ({', '.join(uniq[:hint_limit])}) in {prose_rel}"
            + (f" @ {locs}" if locs else ""),
        )
    if english:
        locs = format_deslop_locs(prose_rel, english, hint_limit)
        r.blocking(
            "deslop",
            f"英文泄漏 ×{len(english)} in {prose_rel}" + (f" @ {locs}" if locs else ""),
        )
    runes = rune_count(prose)
    if emdash and runes > 0:
        per_1k = len(emdash) * 1000.0 / runes
        if per_1k > MAX_EM_DASH_PER_1K:
            locs = format_deslop_locs(prose_rel, emdash, hint_limit)
            r.blocking(
                "deslop",
                f"破折号密度 {per_1k:.1f}/千字 > {MAX_EM_DASH_PER_1K}（{len(emdash)} 个 / {runes} 字）"
                + (f" @ {locs}" if locs else ""),
            )
        else:
            r.advisory("deslop", f"破折号 {len(emdash)} 个（{per_1k:.1f}/千字，限 {MAX_EM_DASH_PER_1K}）")
    if len(simile) > MAX_SIMILES_PER_CH:
        locs = format_deslop_locs(prose_rel, simile, hint_limit)
        r.blocking(
            "deslop",
            f"比喻词 ×{len(simile)} > {MAX_SIMILES_PER_CH}/章 in {prose_rel}"
            + (f" @ {locs}" if locs else ""),
        )
    else:
        per_line: dict[int, int] = {}
        for h in simile:
            per_line[h.line] = per_line.get(h.line, 0) + 1
        dense = sorted(n for n, c in per_line.items() if c > MAX_SIMILES_PER_PARA)
        if dense:
            r.advisory(
                "deslop",
                f"比喻词单段 >{MAX_SIMILES_PER_PARA} @ "
                + ", ".join(f"{prose_rel}:L{n}" for n in dense[:hint_limit]),
            )
    if len(level1) >= 3:
        locs = format_deslop_locs(prose_rel, level1, hint_limit)
        r.blocking(
            "deslop",
            f"一级词 dense ({len(level1)} hits) in {prose_rel}" + (f" @ {locs}" if locs else ""),
        )
    elif level1:
        locs = format_deslop_locs(prose_rel, level1, hint_limit)
        r.advisory(
            "deslop",
            f"一级词 ×{len(level1)} (blocking at ≥3)" + (f" @ {locs}" if locs else ""),
        )
    if soup:
        locs = format_deslop_locs(prose_rel, soup, hint_limit)
        r.blocking("deslop", "chicken-soup ending (P0)" + (f" @ {locs}" if locs else ""))
    return hits


def hit_lines_for(prose_rel: str, prose: str, hits: list[DeslopHit]) -> list[str]:
    lines = prose.splitlines()
    out: list[str] = []
    for h in hits:
        pline = lines[h.line - 1] if 0 < h.line <= len(lines) else ""
        out.append(format_hit_line(prose_rel, h, pline))
    return out

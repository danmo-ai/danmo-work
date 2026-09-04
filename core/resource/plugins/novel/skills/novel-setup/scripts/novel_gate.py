#!/usr/bin/env python3
"""Deterministic novel write-gate. Stdlib only. Invoked by novel skills via exec_shell.

python3 novel_gate.py --action doctor|preflight|precommit|postcommit|scan-deslop \\
  --workdir PROJECT [--book-id SLUG] [--chapter N] [--from A --to B] [--json]
Exit 0 PASS, 1 FAIL, 2 usage/error.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass
from pathlib import Path

HOOK_TYPES = {
    "信息缺口", "未兑现承诺", "高代价选择",
    "身份反转", "关系临界", "倒计时",
    "未完成动作", "突然揭示", "回声/意象",
}
MAX_OPEN_DEBTS = 5
TOXIC = [
    re.compile(r"不是.{1,20}而是"),
    re.compile(r"带着一丝|带着一抹"),
    re.compile(r"声音不大[，,]却带着"),
    re.compile(r"(他|她)知道|终于明白"),
    re.compile(r"眼中闪过|嘴角勾起|心中涌起"),
    re.compile(r"他不知道的是"),
]
LEVEL_ONE = [
    "仿佛", "犹如", "宛若", "不禁", "深吸一口气", "目光深邃",
    "不容置疑", "前所未有", "话锋一转", "心中暗道", "指节泛白", "瞳孔微缩",
]
SOUP_ENDINGS = [
    "或许，这只是个开始", "或许这只是个开始",
    "反击才刚刚开始", "这只是个开始",
]
# --- Quantitative anti-AI hard checks (single source of truth; KB 05 mirrors these) ---
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
CH_FILE_RE = re.compile(r"^ch(\d+)\.md$")
CH_CONTRACT_RE = re.compile(r"^ch(\d+)-contract\.yaml$")
UNIT_ID_RE = re.compile(r"^v\d+-U\d+$")
VOLUME_UNIT_ROW = re.compile(r"\|\s*U(\d+)\s*\|")
OPEN_STATUS = re.compile(r"(?i)\|\s*open\s*\|")
ADVANCED_STATUS = re.compile(r"(?i)\|\s*advanced\s*\|")
STYLE_MAX_RUNES = 480  # context hook: keep the injected style brief tiny


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


def load_yaml_map(text: str) -> dict:
    try:
        import yaml  # type: ignore

        data = yaml.safe_load(text)
        return data if isinstance(data, dict) else {}
    except Exception:
        pass
    root: dict = {}
    stack: list[tuple[int, dict]] = [(0, root)]
    pending_list_key: str | None = None
    pending_list_indent = 0
    for raw in text.splitlines():
        if "#" in raw:
            hash_i = raw.find("#")
            if hash_i == 0 or (hash_i > 0 and raw[hash_i - 1].isspace()):
                raw = raw[:hash_i]
        if not raw.strip():
            continue
        indent = len(raw) - len(raw.lstrip(" "))
        line = raw.strip()
        while stack and indent < stack[-1][0]:
            stack.pop()
            pending_list_key = None
        cur = stack[-1][1]
        if line.startswith("- ") and pending_list_key is not None and indent >= pending_list_indent:
            cur.setdefault(pending_list_key, [])
            if not isinstance(cur[pending_list_key], list):
                cur[pending_list_key] = []
            cur[pending_list_key].append(_unquote(line[2:]))
            continue
        if ":" not in line:
            continue
        key, _, rest = line.partition(":")
        key, rest = key.strip(), rest.strip()
        if rest == "":
            nxt: dict = {}
            cur[key] = nxt
            stack.append((indent + 2, nxt))
            pending_list_key = key
            pending_list_indent = indent + 2
            continue
        pending_list_key = None
        if rest.startswith("["):
            cur[key] = _parse_flow_list(rest)
        else:
            val = _unquote(rest)
            if re.fullmatch(r"-?\d+", val):
                cur[key] = int(val)
            else:
                cur[key] = val
    return root


def contract_rel(ch: int) -> str:
    return f"chapters/ch{ch:03d}-contract.yaml"


def chapter_rel(ch: int) -> str:
    return f"chapters/ch{ch:03d}.md"


def parse_chapter_num(name: str) -> int | None:
    m = CH_FILE_RE.match(name) or CH_CONTRACT_RE.match(name)
    if not m:
        return None
    n = int(m.group(1))
    return n if n > 0 else None


def file_exists(root: Path, rel: str) -> bool:
    return (root / rel).exists()


def read_text(root: Path, rel: str) -> str:
    return (root / rel).read_text(encoding="utf-8")


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


def load_state(path: Path) -> dict:
    st = load_yaml_map(path.read_text(encoding="utf-8"))
    if not st.get("book_id"):
        st["book_id"] = path.parent.name
    return st


def load_contract(book_root: Path, chapter: int) -> tuple[dict, str]:
    rel = contract_rel(chapter)
    path = book_root / rel
    data = load_yaml_map(path.read_text(encoding="utf-8"))
    return data, rel


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


class Report:
    def __init__(self, action: str, book_id: str, book_root: Path, chapter: int = 0):
        self.action = action
        self.book_id = book_id
        self.book_root = str(book_root)
        self.chapter = chapter
        self.verdict = "PASS"
        self.findings: list[dict] = []
        self.context_lines: list[str] = []
        self.counts: dict[str, int] = {}

    def add_counts(self, **kw: int) -> None:
        for k, v in kw.items():
            self.counts[k] = self.counts.get(k, 0) + int(v)

    def blocking(self, check: str, msg: str) -> None:
        self.findings.append({"severity": "blocking", "check": check, "message": msg})

    def advisory(self, check: str, msg: str) -> None:
        self.findings.append({"severity": "advisory", "check": check, "message": msg})

    def finalize(self) -> None:
        self.verdict = "FAIL" if any(f["severity"] == "blocking" for f in self.findings) else "PASS"

    def format(self) -> str:
        lines = ["### VERDICT", self.verdict, "", "### ACTION", self.action]
        if self.book_id:
            lines += ["", "### BOOK", self.book_id]
        if self.chapter:
            lines += ["", "### CHAPTER", str(self.chapter)]
        if self.context_lines:
            lines += ["", "### CONTEXT"]
            lines.extend(self.context_lines)
        if self.counts:
            lines += ["", "### COUNTS"]
            for k in ("em_dash_count", "ai_vocab_count", "english_leak_count", "simile_count"):
                if k in self.counts:
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
        return "\n".join(lines) + "\n"

    def as_dict(self) -> dict:
        return {
            "action": self.action,
            "book_id": self.book_id,
            "book_root": self.book_root,
            "chapter": self.chapter,
            "verdict": self.verdict,
            "findings": self.findings,
            "context": self.context_lines,
            "counts": self.counts,
        }


def last_runes(s: str, n: int) -> str:
    if n <= 0:
        return ""
    return s if len(s) <= n else s[-n:]


def rune_count(s: str) -> int:
    return sum(1 for r in s if r not in " \n\t\r")


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


def count_open_foreshadows(tracker: str) -> int:
    n = 0
    for line in tracker.splitlines():
        if "|" not in line or "---" in line:
            continue
        low = line.lower()
        if "status" in low and "summary" in low:
            continue
        if OPEN_STATUS.search(line) or ADVANCED_STATUS.search(line):
            n += 1
    return n


def unit_listed(outline_root: Path, unit_id: str) -> bool:
    unit_id = unit_id.strip()
    if not unit_id or not outline_root.is_dir():
        return False
    u_short = ""
    vol = ""
    i = unit_id.rfind("-U")
    if i >= 0:
        vol = unit_id[:i]
        u_short = unit_id[i + 1 :]
    for path in outline_root.rglob("*.md"):
        text = path.read_text(encoding="utf-8", errors="replace")
        if unit_id in text:
            return True
        if u_short and VOLUME_UNIT_ROW.search(text) and f"| {u_short} |" in text:
            base = path.stem
            if base.lower() == vol.lower() or vol in text:
                return True
    return False


def ledger_path(book_root: Path) -> Path:
    return book_root / "continuity/ledger.md"


def has_reader_continuity(book_root: Path) -> bool:
    """Prefer ledger.md; accept legacy public-lore + tracking pair."""
    if ledger_path(book_root).is_file():
        return True
    return file_exists(book_root, "continuity/public-lore.md") and file_exists(
        book_root, "continuity/tracking.md"
    )


def continuity_open_loops_text(book_root: Path) -> str:
    """Text used to count open foreshadow / loop rows."""
    ledger = ledger_path(book_root)
    if ledger.is_file():
        return ledger.read_text(encoding="utf-8", errors="replace")
    tracker = book_root / "continuity/foreshadow-tracker.md"
    if tracker.is_file():
        return tracker.read_text(encoding="utf-8", errors="replace")
    tracking = book_root / "continuity/tracking.md"
    if tracking.is_file():
        return tracking.read_text(encoding="utf-8", errors="replace")
    return ""


def open_debt_count(book_root: Path, contract: dict) -> int:
    n = len(nonempty_list(contract.get("reader_debt")))
    n += count_open_foreshadows(continuity_open_loops_text(book_root))
    return n


def list_chapter_nums(book_root: Path, kind: str) -> list[int]:
    d = book_root / "chapters"
    if not d.is_dir():
        return []
    out = []
    for e in d.iterdir():
        if e.is_dir():
            continue
        n = parse_chapter_num(e.name)
        if n is None:
            continue
        if kind == "md" and CH_FILE_RE.match(e.name):
            out.append(n)
        elif kind == "contract" and CH_CONTRACT_RE.match(e.name):
            out.append(n)
    return out


def summary_source_text(book_root: Path) -> tuple[str, str]:
    """Return (text, rel) for chapter summary blocks — ledger preferred."""
    ledger = ledger_path(book_root)
    if ledger.is_file():
        return ledger.read_text(encoding="utf-8", errors="replace"), "continuity/ledger.md"
    summaries = book_root / "continuity/chapter_summaries.md"
    if summaries.is_file():
        return summaries.read_text(encoding="utf-8", errors="replace"), "continuity/chapter_summaries.md"
    return "", ""


def has_summary(summaries: str, ch: int) -> bool:
    return f"## ch{ch:03d}" in summaries


def archived_summary_text(book_root: Path, ch: int) -> str:
    """Find an archived ## chNNN block under continuity/summaries/vNN.md (volume-close archive)."""
    d = book_root / "continuity" / "summaries"
    if not d.is_dir():
        return ""
    for path in sorted(d.glob("*.md")):
        text = path.read_text(encoding="utf-8", errors="replace")
        if has_summary(text, ch):
            return text
    return ""


def hook_of(contract: dict) -> tuple[str, str]:
    hook = contract.get("hook") or {}
    if not isinstance(hook, dict):
        return "", ""
    return str(hook.get("type") or "").strip(), str(hook.get("out") or "").strip()


SUMMARY_KEYS = ("事件", "状态变化", "伏笔", "钩子", "下章指向")
FS_ID_RE = re.compile(r"FS-\d+", re.I)
UNIT_BEAT_RE = re.compile(
    r"ch\s*(\d+)\s*(?:[-–—]\s*ch\s*(\d+))?\s*(建立期待|尝试|加压|决断|兑现|余波|切断)[：:]\s*(.*)",
    re.I,
)
CAST_ANCHOR_RE = re.compile(r"^\s*[-*]\s*\*?\*?(视觉|语言|行为)\*?\*?[：:]\s*(.*)")


def extract_chapter_summary_block(text: str, ch: int) -> str:
    needle = f"## ch{ch:03d}"
    i = text.find(needle)
    if i < 0:
        return ""
    rest = text[i:]
    nxt = re.search(r"\n## ch\d+", rest[1:])
    return rest if not nxt else rest[: nxt.start() + 1]


def summary_has_five_keys(block: str) -> list[str]:
    missing = []
    for key in SUMMARY_KEYS:
        if not re.search(rf"[-*]\s*{re.escape(key)}\s*[：:]", block):
            missing.append(key)
    return missing


def parse_cast_snapshot_names(ledger_text: str) -> set[str]:
    names: set[str] = set()
    in_cast = False
    for line in ledger_text.splitlines():
        if re.match(r"^###\s*Cast snapshot", line, re.I) or "Cast snapshot" in line and line.startswith("#"):
            in_cast = True
            continue
        if in_cast and line.startswith("#"):
            break
        if in_cast and line.strip().startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if not cells or cells[0] in ("角色", "----", "---") or set(cells[0]) <= {"-", ":"}:
                continue
            if cells[0]:
                names.add(cells[0])
    return names


def parse_open_loop_ids(ledger_text: str) -> set[str]:
    ids: set[str] = set()
    in_loops = False
    for line in ledger_text.splitlines():
        if re.match(r"^##\s*Open loops", line, re.I):
            in_loops = True
            continue
        if in_loops and line.startswith("## ") and "Open loops" not in line:
            break
        if in_loops:
            for m in FS_ID_RE.finditer(line):
                ids.add(m.group(0).upper())
    return ids


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


def foreshadow_ids_from_contract(contract: dict) -> list[str]:
    info = contract.get("info_control") or {}
    if not isinstance(info, dict):
        return []
    raw = info.get("foreshadowing") or []
    items = raw if isinstance(raw, list) else nonempty_list(raw)
    found: list[str] = []
    for item in items:
        for m in FS_ID_RE.finditer(str(item)):
            found.append(m.group(0).upper())
    return found


def previous_hook_out(book_root: Path, ch: int) -> str:
    if ch <= 1:
        return ""
    prev = ch - 1
    try:
        c, _ = load_contract(book_root, prev)
        _, hout = hook_of(c)
        if hout:
            return hout
    except OSError:
        pass
    text, _ = summary_source_text(book_root)
    block = extract_chapter_summary_block(text, prev)
    m = re.search(r"[-*]\s*下章指向\s*[：:]\s*(.+)", block)
    if m:
        return m.group(1).strip()
    m = re.search(r"[-*]\s*钩子\s*[：:]\s*(.+)", block)
    return m.group(1).strip() if m else ""


def unit_beat_line(book_root: Path, unit_id: str, ch: int) -> str:
    unit_id = (unit_id or "").strip()
    outline = book_root / "outline"
    if not unit_id or not outline.is_dir():
        return ""
    for path in outline.rglob("*.md"):
        text = path.read_text(encoding="utf-8", errors="replace")
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


def cast_snapshot_rows(ledger_text: str) -> list[str]:
    rows: list[str] = []
    in_cast = False
    for line in ledger_text.splitlines():
        if re.match(r"^###\s*Cast snapshot", line, re.I) or (
            "Cast snapshot" in line and line.startswith("#")
        ):
            in_cast = True
            continue
        if in_cast and line.startswith("#"):
            break
        if in_cast and line.strip().startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if not cells or cells[0] in ("角色",) or set(cells[0]) <= {"-", ":"}:
                continue
            rows.append("| " + " | ".join(cells) + " |")
    return rows


def open_loops_rows(ledger_text: str) -> list[str]:
    rows: list[str] = []
    in_loops = False
    for line in ledger_text.splitlines():
        if re.match(r"^##\s*Open loops", line, re.I):
            in_loops = True
            continue
        if in_loops and line.startswith("## ") and "Open loops" not in line:
            break
        if in_loops and line.strip().startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if not cells or cells[0].upper() in ("ID",) or set(cells[0]) <= {"-", ":"}:
                continue
            rows.append("| " + " | ".join(cells) + " |")
    return rows


def _match_cast_file(files: list[Path], name: str) -> tuple[Path, str] | tuple[None, None]:
    """Find the cast file for a character: exact filename (stem) match first,
    then fall back to content match (aliases). Content-first matching is wrong:
    another character's card may mention this name in its 关系 table."""
    for path in files:
        if path.stem == name:
            return path, path.read_text(encoding="utf-8", errors="replace")
    for path in files:
        text = path.read_text(encoding="utf-8", errors="replace")
        first = text.splitlines()[0] if text.splitlines() else ""
        if name in text or name in path.stem or name in first:
            return path, text
    return None, None


def load_cast_anchors(book_root: Path, names: list[str]) -> list[str]:
    cast_dir = book_root / "canon" / "cast"
    if not cast_dir.is_dir() or not names:
        return []
    files = list(cast_dir.glob("*.md"))
    out: list[str] = []
    for name in names:
        path, text = _match_cast_file(files, name)
        if path is None:
            continue
        anchors = []
        for line in text.splitlines():
            m = CAST_ANCHOR_RE.match(line)
            if m and m.group(2).strip():
                anchors.append(f"{m.group(1)}={m.group(2).strip()}")
        first = text.splitlines()[0] if text.splitlines() else ""
        label = first.lstrip("# ").strip() or path.stem
        if anchors:
            out.append(f"{label}: " + "; ".join(anchors[:3]))
        else:
            out.append(f"{label}: (无三锚点行)")
    return out


def load_cast_relations(book_root: Path, names: list[str]) -> list[str]:
    """Relationship rows between on-scene characters only: from each named
    character's cast card 关系 table, keep rows whose 对方 is also on scene."""
    cast_dir = book_root / "canon" / "cast"
    if not cast_dir.is_dir() or len(names) < 2:
        return []
    files = list(cast_dir.glob("*.md"))
    out: list[str] = []
    for name in names:
        path, text = _match_cast_file(files, name)
        if path is None:
            continue
        first = text.splitlines()[0] if text.splitlines() else ""
        label = first.lstrip("# ").strip() or path.stem
        in_rel = False
        for line in text.splitlines():
            s = line.strip()
            if s.startswith("#"):
                in_rel = "关系" in s
                continue
            if not in_rel or not s.startswith("|"):
                continue
            cells = [c.strip() for c in s.strip("|").split("|")]
            if not cells or cells[0] in ("对方",) or set(cells[0]) <= {"-", ":"}:
                continue
            other = cells[0]
            if other == name or other == label:
                continue
            if not any(other == w or other in w or w in other for w in names):
                continue
            detail = " / ".join(c for c in cells[1:3] if c)
            out.append(f"{label} → {other}: {detail}" if detail else f"{label} → {other}")
    return out


def build_preflight_context(book_root: Path, contract: dict, ch: int, r: Report) -> list[str]:
    lines: list[str] = []
    style = style_fingerprint_brief(book_root)
    if style:
        lines.append("- 风格指纹（本书固定，写入时对齐 POV/语域/句式/禁语/章末钩）:")
        for ln in style.splitlines():
            lines.append(f"  {ln}")
    prev_hook = previous_hook_out(book_root, ch)
    beats = nonempty_list(contract.get("beats"))
    debts = nonempty_list(contract.get("reader_debt"))
    if prev_hook:
        lines.append(f"- 接钩（上章）: {prev_hook}")
        joined = " ".join(beats + debts)
        if prev_hook not in joined and not any(
            len(tok) >= 4 and tok in joined for tok in re.split(r"[\s，,。；;、]+", prev_hook) if tok
        ):
            r.advisory(
                "hook-continue",
                "beats[0]/reader_debt 未明显接住上章 hook.out — 确认章首接钩",
            )
    else:
        lines.append("- 接钩（上章）: （首章或无上章钩）")

    ledger_text = ""
    ledger = ledger_path(book_root)
    if ledger.is_file():
        ledger_text = ledger.read_text(encoding="utf-8", errors="replace")

    who = state_delta_who(nonempty_list(contract.get("state_deltas")))
    # Also match cast files mentioned in beats
    cast_dir = book_root / "canon" / "cast"
    if cast_dir.is_dir():
        for path in cast_dir.glob("*.md"):
            stem = path.stem
            blob = " ".join(beats + nonempty_list(contract.get("state_deltas")))
            if stem in blob or any(stem in b for b in beats):
                if stem not in who:
                    who.append(stem)
    snap = cast_snapshot_rows(ledger_text)
    lines.append("- 人物现场（Cast snapshot）:")
    if snap:
        for row in snap:
            if who and not any(w in row for w in who):
                continue
            lines.append(f"  {row}")
        if who and not any(any(w in row for w in who) for row in snap):
            lines.append(f"  （合同点名 {', '.join(who)} 不在 snapshot — Commit 后须补）")
    else:
        lines.append("  （ledger 无 Cast snapshot 表）")

    anchors = load_cast_anchors(book_root, who)
    lines.append("- 三锚点（上场）:")
    if anchors:
        for a in anchors:
            lines.append(f"  - {a}")
    else:
        lines.append("  （无点名角色或无人物卡）")

    relations = load_cast_relations(book_root, who)
    if relations:
        lines.append("- 关系（在场角色间，来自人物卡关系表）:")
        for rel in relations:
            lines.append(f"  - {rel}")

    loops = open_loops_rows(ledger_text)
    lines.append("- 开放债务:")
    if loops:
        for row in loops[:8]:
            lines.append(f"  {row}")
    else:
        lines.append("  （无 open loops 行）")

    _, hout = hook_of(contract)
    lines.append("- 本章硬约束:")
    lines.append(f"  purpose: {str(contract.get('purpose') or '').strip()}")
    lines.append(f"  beats: {beats}")
    lines.append(f"  forbidden: {nonempty_list(contract.get('forbidden'))}")
    lines.append(f"  state_deltas: {nonempty_list(contract.get('state_deltas'))}")
    lines.append(f"  hook.out: {hout}")

    unit = str(contract.get("unit_id") or "").strip()
    beat_line = unit_beat_line(book_root, unit, ch)
    lines.append(f"- 单元功能 ({unit or '?'}): {beat_line or '（未在卷纲解析到本章节拍）'}")
    lines.append("- 加载纪律: 只消费本 CONTEXT + 本章合同；禁止扫树；禁止 author-lore。")
    return lines


def check_doctor(book_root: Path, st: dict, r: Report) -> None:
    for rel in ("novel-state.yaml", "book-bible.md", "canon/world.md"):
        if not file_exists(book_root, rel):
            r.blocking("layout", "missing " + rel)
    for d in ("canon", "canon/cast", "outline", "outline/volumes", "chapters", "continuity", "reviews"):
        if not file_exists(book_root, d):
            r.blocking("layout", "missing directory " + d + "/")
    # author-lore always seeded; reader continuity = ledger.md (or legacy pair)
    if not file_exists(book_root, "canon/author-lore.md"):
        if writing_stage(str(st.get("stage") or "")):
            r.blocking("lore-tracks", "missing canon/author-lore.md (required from outline/writing onward)")
        else:
            r.advisory("lore-tracks", "missing canon/author-lore.md — seed at setup")
    if not has_reader_continuity(book_root):
        if writing_stage(str(st.get("stage") or "")):
            r.blocking(
                "lore-tracks",
                "missing continuity/ledger.md (or legacy public-lore.md + tracking.md)",
            )
        else:
            r.advisory("lore-tracks", "missing continuity/ledger.md — seed at setup")
    contracts = {}
    for n in list_chapter_nums(book_root, "contract"):
        contracts[n] = True
        try:
            c, rel = load_contract(book_root, n)
        except OSError as e:
            r.blocking("orphan-contract", f"{contract_rel(n)}: {e}")
            continue
        ch_field = int(c.get("chapter") or 0)
        if ch_field and ch_field != n:
            r.blocking("orphan-contract", f"{rel} chapter field={ch_field} filename={n}")
        if str(c.get("status") or "").strip() in ("drafted", "reviewed"):
            if not file_exists(book_root, chapter_rel(n)):
                r.blocking("orphan-contract", f"{rel} status={c.get('status')} but {chapter_rel(n)} missing")
    for n in list_chapter_nums(book_root, "md"):
        if n not in contracts:
            r.blocking("orphan-prose", f"{chapter_rel(n)} has no {contract_rel(n)}")
    last = int(st.get("last_committed_ch") or 0)
    if last > 0 and not file_exists(book_root, chapter_rel(last)):
        r.blocking("state", f"last_committed_ch={last} but {chapter_rel(last)} missing")


def check_preflight(book_root: Path, st: dict, ch: int, r: Report) -> None:
    if not file_exists(book_root, "novel-state.yaml"):
        r.blocking("state", "missing novel-state.yaml")
        return
    if not has_reader_continuity(book_root):
        r.blocking(
            "lore-tracks",
            "missing continuity/ledger.md (or legacy public-lore + tracking) — draft from ledger only",
        )
    if file_exists(book_root, "canon/author-lore.md"):
        r.advisory("lore-tracks", "do not load canon/author-lore.md into the draft context")
    elif writing_stage(str(st.get("stage") or "")):
        r.blocking("lore-tracks", "missing canon/author-lore.md")
    try:
        c, rel = load_contract(book_root, ch)
    except OSError as e:
        r.blocking("contract", f"{contract_rel(ch)}: {e}")
        return
    ch_field = int(c.get("chapter") or 0)
    if ch_field and ch_field != ch:
        r.blocking("contract", f"{rel} chapter={ch_field} want {ch}")
    status = str(c.get("status") or "").strip()
    if status not in ("accepted", "drafted"):
        r.blocking("contract", f"{rel} status={status} (need accepted before prose)")
    unit = str(c.get("unit_id") or "").strip()
    if not unit:
        r.blocking("unit_id", f"{rel} unit_id empty — return to novel-plan")
    elif not UNIT_ID_RE.match(unit):
        r.blocking("unit_id", f"{rel} unit_id={unit} must match vNN-U#")
    elif not unit_listed(book_root / "outline", unit):
        r.blocking("unit_id", f"{unit} not found in outline/ — return to novel-plan")
    if is_blank(c.get("purpose")):
        r.blocking("contract", "purpose empty")
    if not nonempty_list(c.get("beats")):
        r.blocking("contract", "beats empty")
    htype, hout = hook_of(c)
    if htype not in HOOK_TYPES:
        r.blocking("hook", f"hook.type must be one of KB 爽点与追读 types, got {htype}")
    if not hout:
        r.blocking("hook", "hook.out empty (need a concrete event)")
    if tomato_profile(str(st.get("qc_profile") or "")) and is_blank(c.get("pleasure_point")):
        r.blocking("pleasure_point", f"qc_profile={st.get('qc_profile')} requires pleasure_point")
    debts = open_debt_count(book_root, c)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")
    r.context_lines = build_preflight_context(book_root, c, ch, r)


def check_precommit(book_root: Path, st: dict, ch: int, r: Report) -> None:
    try:
        c, rel = load_contract(book_root, ch)
    except OSError as e:
        r.blocking("contract", f"{contract_rel(ch)}: {e}")
        return
    prose_rel = chapter_rel(ch)
    try:
        prose = read_text(book_root, prose_rel)
    except OSError:
        r.blocking("prose", f"{prose_rel} missing")
        return
    status = str(c.get("status") or "").strip()
    if status not in ("drafted", "accepted", "reviewed"):
        r.advisory("contract", f"{rel} status={status} expected drafted before review")
    apply_deslop_to_report(prose_rel, prose, r)
    _, hout = hook_of(c)
    if hout and hout not in prose and rune_count(hout) >= 8:
        r.advisory("hook", "hook.out not found verbatim in prose — confirm the event landed")
    wt = int(c.get("word_target") or 0)
    if wt > 0:
        n = rune_count(prose)
        if n < wt * 6 // 10:
            r.advisory("length", f"prose ~{n} runes vs word_target={wt}")
    debts = open_debt_count(book_root, c)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")


def check_postcommit(book_root: Path, st: dict, ch: int, r: Report) -> None:
    try:
        c, rel = load_contract(book_root, ch)
    except OSError as e:
        r.blocking("contract", f"{contract_rel(ch)}: {e}")
        return
    if str(c.get("status") or "").strip() != "reviewed":
        r.blocking("contract", f"{rel} status={c.get('status')} (Commit requires reviewed)")
    if not file_exists(book_root, chapter_rel(ch)):
        r.blocking("prose", f"{chapter_rel(ch)} missing")
    text, src = summary_source_text(book_root)
    if not src:
        r.blocking("commit", "missing continuity/ledger.md (or legacy chapter_summaries.md)")
        return
    if not has_summary(text, ch):
        arch = archived_summary_text(book_root, ch)
        if arch:
            text = arch  # archived at volume close (continuity/summaries/vNN.md)
        else:
            r.blocking(
                "commit",
                f"no ## ch{ch:03d} block in {src} or continuity/summaries/ archive",
            )
            return
    block = extract_chapter_summary_block(text, ch)
    missing = summary_has_five_keys(block)
    if missing:
        r.blocking(
            "commit",
            f"## ch{ch:03d} missing summary keys: {', '.join(missing)} "
            f"(need 事件/状态变化/伏笔/钩子/下章指向)",
        )
    # state_deltas / FS-ids are always checked against the live ledger
    ledger_text = ""
    lp = ledger_path(book_root)
    if lp.is_file():
        ledger_text = lp.read_text(encoding="utf-8", errors="replace")
    # state_deltas → Cast snapshot
    who = state_delta_who(nonempty_list(c.get("state_deltas")))
    if who and ledger_text:
        names = parse_cast_snapshot_names(ledger_text)
        for w in who:
            if not any(w in n or n in w for n in names):
                r.blocking(
                    "commit",
                    f"state_deltas who={w} not found in ledger Cast snapshot after Commit",
                )
    # FS-ids → Open loops
    fs_ids = foreshadow_ids_from_contract(c)
    if fs_ids and ledger_text:
        loop_ids = parse_open_loop_ids(ledger_text)
        for fs in fs_ids:
            if fs not in loop_ids:
                r.blocking(
                    "commit",
                    f"foreshadowing {fs} not found in ledger Open loops after Commit",
                )
    last = int(st.get("last_committed_ch") or 0)
    if last < ch:
        r.blocking("state", f"last_committed_ch={last} want ≥{ch} after Commit")
    if not has_reader_continuity(book_root):
        r.blocking(
            "lore-tracks",
            "Commit must refresh continuity/ledger.md (or legacy public-lore + tracking)",
        )

def check_scan_deslop(book_root: Path, chapters: list[int], r: Report) -> list[str]:
    """Printable HIT lines; findings go on Report (same P0 thresholds as precommit)."""
    out: list[str] = []
    for ch in chapters:
        prose_rel = chapter_rel(ch)
        if not file_exists(book_root, prose_rel):
            r.blocking("prose", f"{prose_rel} missing")
            continue
        prose = read_text(book_root, prose_rel)
        hits = apply_deslop_to_report(prose_rel, prose, r)
        lines = prose.splitlines()
        for h in hits:
            pline = lines[h.line - 1] if 0 < h.line <= len(lines) else ""
            out.append(format_hit_line(prose_rel, h, pline))
    return out


def resolve_scan_chapters(chapter: int, from_ch: int, to_ch: int) -> list[int]:
    if chapter > 0:
        return [chapter]
    if from_ch > 0 and to_ch > 0:
        if to_ch < from_ch:
            raise ValueError(f"--to {to_ch} < --from {from_ch}")
        return list(range(from_ch, to_ch + 1))
    raise ValueError("scan-deslop requires --chapter N or --from A --to B")


def run_with_hits(
    workdir: str,
    book_id: str,
    action: str,
    chapter: int,
    from_ch: int = 0,
    to_ch: int = 0,
) -> tuple[Report, list[str]]:
    action = (action or "").strip().lower()
    if action not in {"preflight", "precommit", "postcommit", "doctor", "scan-deslop"}:
        raise ValueError(
            f"unknown action {action!r} (preflight|precommit|postcommit|doctor|scan-deslop)"
        )
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    hit_lines: list[str] = []
    if action == "scan-deslop":
        chapters = resolve_scan_chapters(chapter, from_ch, to_ch)
        r = Report(action, bid, root, chapters[0] if len(chapters) == 1 else 0)
        hit_lines = check_scan_deslop(root, chapters, r)
    else:
        r = Report(action, bid, root, chapter)
        if action == "doctor":
            check_doctor(root, st, r)
        else:
            if chapter <= 0:
                raise ValueError(f"chapter is required for {action}")
            if action == "preflight":
                check_preflight(root, st, chapter, r)
            elif action == "precommit":
                check_precommit(root, st, chapter, r)
            else:
                check_postcommit(root, st, chapter, r)
    r.finalize()
    return r, hit_lines


def run(workdir: str, book_id: str, action: str, chapter: int) -> Report:
    rep, _ = run_with_hits(workdir, book_id, action, chapter)
    return rep


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
        lines = fp.read_text(encoding="utf-8", errors="replace").strip().splitlines()
    else:
        bible = book_root / "book-bible.md"
        if not bible.is_file():
            return ""
        lines = _style_card_lines(bible.read_text(encoding="utf-8", errors="replace"))
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


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="Novel write-gate / doctor / deslop scan (skill script)")
    p.add_argument(
        "--action",
        default="doctor",
        help="preflight | precommit | postcommit | doctor | scan-deslop",
    )
    p.add_argument("--workdir", default=".", help="project root or book root")
    p.add_argument("--book-id", default="", help="slug under novel/<book-id>/")
    p.add_argument("--chapter", type=int, default=0)
    p.add_argument("--from", dest="from_ch", type=int, default=0, help="scan-deslop range start")
    p.add_argument("--to", dest="to_ch", type=int, default=0, help="scan-deslop range end")
    p.add_argument("--json", action="store_true")
    args = p.parse_args(argv)
    try:
        rep, hit_lines = run_with_hits(
            os.path.abspath(args.workdir),
            args.book_id,
            args.action,
            args.chapter,
            args.from_ch,
            args.to_ch,
        )
    except ValueError as e:
        print(e, file=sys.stderr)
        return 2
    except FileNotFoundError as e:
        print(e, file=sys.stderr)
        return 2
    except OSError as e:
        print(e, file=sys.stderr)
        return 2
    if args.json:
        payload = rep.as_dict()
        if (args.action or "").strip().lower() == "scan-deslop":
            payload["hits"] = hit_lines
        json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
    else:
        if (args.action or "").strip().lower() == "scan-deslop":
            sys.stdout.write("### HITS\n")
            if hit_lines:
                for line in hit_lines:
                    sys.stdout.write(line + "\n")
            else:
                sys.stdout.write("None.\n")
            sys.stdout.write("\n")
        sys.stdout.write(rep.format())
    return 0 if rep.verdict == "PASS" else 1


if __name__ == "__main__":
    sys.exit(main())

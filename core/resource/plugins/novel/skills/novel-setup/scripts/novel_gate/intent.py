"""Classify short novel goals into production pack stages (rule-based, no LLM).

Prefer explicit stage= / --stage parameters from the workbench. Regex only
runs on the first task line when those parameters are absent.
"""
from __future__ import annotations

import re
from dataclasses import dataclass

# No trailing \b: Chinese may glue to the id (「v01-U9正文写作」).
UNIT_RE = re.compile(r"v(\d+)-U(\d+)", re.I)
VOLUME_RE = re.compile(r"v(\d+)(?!-U)", re.I)

# Explicit params from workbench short prefill (preferred).
_STAGE_KV = re.compile(
    r"(?:^|\s)stage\s*=\s*(write|outline|finalize)\b",
    re.I,
)
_UNIT_KV = re.compile(r"(?:^|\s)unit\s*=\s*(v\d+-U\d+)\b", re.I)
_VOLUME_KV = re.compile(r"(?:^|\s)volume\s*=\s*(v\d+)\b", re.I)
_STAGE_FLAG = re.compile(
    r"--stage\s+(write|outline|finalize)\b",
    re.I,
)
_UNIT_FLAG = re.compile(r"--unit\s+(v\d+-U\d+)\b", re.I)
_VOLUME_FLAG = re.compile(r"--volume\s+(v\d+)\b", re.I)

# Order matters: more specific phrases first.
_FINALIZE_PAT = re.compile(
    r"(重新定稿|再定稿|重定稿|审阅定稿|定稿|finalize|re-?finalize|扩.?审.?润|postcommit)",
    re.I,
)
_WRITE_PAT = re.compile(
    r"(重写正文|重写单元|正文写作|写正文|写单元|写作|rewrite\s*prose|draft\s*unit)",
    re.I,
)
_OUTLINE_PAT = re.compile(
    r"(重写细纲|细纲复核|一批细纲|填细纲|写细纲|细纲|contract|outline-batch)",
    re.I,
)
_NONE_PAT = re.compile(
    r"(立项|规划|卷纲|accept-volume|迁移|migrate|卡文|续写|brainstorm|人物卡|cast-fix)",
    re.I,
)

GATE_REL = "plugins/novel/skills/novel-setup/scripts/novel_gate.py"


@dataclass(frozen=True)
class NovelIntent:
    kind: str  # outline | write | finalize | none
    stage: str  # outline | write | finalize | ""
    unit_id: str
    volume: str
    confidence: str  # high | medium | low
    reason: str


def extract_unit(goal: str) -> str:
    m = UNIT_RE.search(goal or "")
    if not m:
        return ""
    return f"v{int(m.group(1)):02d}-U{int(m.group(2))}"


def extract_volume(goal: str, unit_id: str = "") -> str:
    if unit_id:
        um = UNIT_RE.match(unit_id)
        if um:
            return f"v{int(um.group(1)):02d}"
    m = VOLUME_RE.search(goal or "")
    if not m:
        return ""
    return f"v{int(m.group(1)):02d}"


def _normalize_unit(raw: str) -> str:
    m = UNIT_RE.search(raw or "")
    if not m:
        return ""
    return f"v{int(m.group(1)):02d}-U{int(m.group(2))}"


def _normalize_volume(raw: str) -> str:
    m = VOLUME_RE.search(raw or "")
    if not m:
        return ""
    return f"v{int(m.group(1)):02d}"


def parse_explicit_stage(goal: str) -> NovelIntent | None:
    """Return intent when goal carries stage=… or --stage … (workbench / gate)."""
    text = goal or ""
    stage = ""
    reason = ""
    m = _STAGE_KV.search(text)
    if m:
        stage = m.group(1).lower()
        reason = "stage-kv"
    else:
        m = _STAGE_FLAG.search(text)
        if m:
            stage = m.group(1).lower()
            reason = "stage-flag"
    if not stage:
        return None

    unit = ""
    um = _UNIT_KV.search(text) or _UNIT_FLAG.search(text)
    if um:
        unit = _normalize_unit(um.group(1))
    if not unit:
        unit = extract_unit(text)

    vol = ""
    vm = _VOLUME_KV.search(text) or _VOLUME_FLAG.search(text)
    if vm:
        vol = _normalize_volume(vm.group(1))
    if not vol:
        vol = extract_volume(text, unit)

    return NovelIntent(stage, stage, unit, vol, "high", reason)


def first_task_line(goal: str) -> str:
    """First non-empty line that looks like the user task (skip footers)."""
    for line in (goal or "").splitlines():
        s = line.strip()
        if not s:
            continue
        if s.startswith("---"):
            break
        if s.startswith("技能 ") or s.startswith("stage="):
            continue
        if s.startswith("forbid:") or s.startswith("Team："):
            continue
        # Strip 【任务】 wrapper label
        if s in ("【任务】",):
            continue
        return s
    return (goal or "").strip().splitlines()[0].strip() if (goal or "").strip() else ""


def classify_goal(goal: str, *, active_unit: str = "", state_stage: str = "") -> NovelIntent:
    """Map user/delegate goal text to a pack stage.

    Explicit stage= / --stage wins. Otherwise regex on the first task line only.
    """
    text = (goal or "").strip()
    if not text:
        return NovelIntent("none", "", "", "", "low", "empty-goal")

    explicit = parse_explicit_stage(text)
    if explicit is not None:
        unit = explicit.unit_id or (active_unit or "").strip()
        if unit and not UNIT_RE.match(unit):
            unit = ""
        vol = explicit.volume or extract_volume("", unit)
        return NovelIntent(
            explicit.kind,
            explicit.stage,
            unit,
            vol,
            explicit.confidence,
            explicit.reason,
        )

    # Regex only on the first task line — not protocol / forbid footnotes.
    line = first_task_line(text)
    unit = extract_unit(line) or extract_unit(text) or (active_unit or "").strip()
    if unit and not UNIT_RE.match(unit):
        unit = ""
    vol = extract_volume(line, unit) or extract_volume(text, unit)

    if _NONE_PAT.search(line) and not (
        _FINALIZE_PAT.search(line) or _WRITE_PAT.search(line) or _OUTLINE_PAT.search(line)
    ):
        return NovelIntent("none", "", unit, vol, "medium", "planning-or-setup")

    if _FINALIZE_PAT.search(line):
        conf = "high" if unit else "medium"
        return NovelIntent("finalize", "finalize", unit, vol or extract_volume(unit), conf, "finalize-phrase")

    if _WRITE_PAT.search(line):
        conf = "high" if unit else "medium"
        return NovelIntent("write", "write", unit, vol or extract_volume(unit), conf, "write-phrase")

    if _OUTLINE_PAT.search(line):
        conf = "high" if (unit or vol) else "medium"
        return NovelIntent("outline", "outline", unit, vol or extract_volume(unit), conf, "outline-phrase")

    if unit and re.fullmatch(r"v\d+-U\d+", line, re.I):
        return NovelIntent("none", "", unit, vol, "low", "unit-id-only")

    st = (state_stage or "").strip().lower()
    if unit and st in ("writing", "units", "draft"):
        return NovelIntent("write", "write", unit, vol, "low", "state-hint-write")

    return NovelIntent("none", "", unit, vol, "low", "no-match")


def gate_cmd(
    *,
    book_id: str,
    stage: str,
    unit_id: str = "",
    volume: str = "",
) -> str:
    """Pinned one-liner for exec_shell fallback (WORK_HOME expanded by shell)."""
    g = f'"$WORK_HOME/{GATE_REL}"'
    base = f'python3 {g} --workdir . --book-id {book_id} --action prompt-pack --stage {stage}'
    if stage == "outline":
        return f"{base} --volume {volume or 'vNN'}"
    return f"{base} --unit {unit_id or 'vNN-U#'}"

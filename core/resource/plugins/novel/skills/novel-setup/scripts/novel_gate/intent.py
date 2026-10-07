"""Classify short novel goals into production pack stages (rule-based, no LLM)."""
from __future__ import annotations

import re
from dataclasses import dataclass

# No trailing \b: Chinese may glue to the id (「v01-U9正文写作」).
UNIT_RE = re.compile(r"v(\d+)-U(\d+)", re.I)
VOLUME_RE = re.compile(r"v(\d+)(?!-U)", re.I)

# Order matters: more specific phrases first.
_FINALIZE_PAT = re.compile(
    r"(重新定稿|再定稿|重定稿|审阅定稿|定稿|finalize|re-?finalize|扩.?审.?润|postcommit)",
    re.I,
)
_WRITE_PAT = re.compile(
    r"(重写正文|重写单元|正文写作|写正文|写单元|写作|rewrite\s*prose|draft\s*unit|"
    r"prompt-pack\s*--stage\s*write)",
    re.I,
)
_OUTLINE_PAT = re.compile(
    r"(重写细纲|细纲复核|一批细纲|填细纲|写细纲|细纲|contract|outline-batch|"
    r"prompt-pack\s*--stage\s*outline)",
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


def classify_goal(goal: str, *, active_unit: str = "", state_stage: str = "") -> NovelIntent:
    """Map user/delegate goal text to a pack stage.

    Short goals like「v01-U9正文写作」must route; planning/setup stays none.
    """
    text = (goal or "").strip()
    unit = extract_unit(text) or (active_unit or "").strip()
    if unit and not UNIT_RE.match(unit):
        unit = ""
    vol = extract_volume(text, unit)

    if not text:
        return NovelIntent("none", "", unit, vol, "low", "empty-goal")

    if _NONE_PAT.search(text) and not (
        _FINALIZE_PAT.search(text) or _WRITE_PAT.search(text) or _OUTLINE_PAT.search(text)
    ):
        return NovelIntent("none", "", unit, vol, "medium", "planning-or-setup")

    # Finalize before write: 「审阅定稿」contains both 审 and 定稿
    if _FINALIZE_PAT.search(text):
        conf = "high" if unit else "medium"
        return NovelIntent("finalize", "finalize", unit, vol or extract_volume(unit), conf, "finalize-phrase")

    if _WRITE_PAT.search(text):
        conf = "high" if unit else "medium"
        return NovelIntent("write", "write", unit, vol or extract_volume(unit), conf, "write-phrase")

    if _OUTLINE_PAT.search(text):
        conf = "high" if (unit or vol) else "medium"
        return NovelIntent("outline", "outline", unit, vol or extract_volume(unit), conf, "outline-phrase")

    # Soft: state stage hints when goal is just a unit id
    if unit and re.fullmatch(r"v\d+-U\d+", text, re.I):
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

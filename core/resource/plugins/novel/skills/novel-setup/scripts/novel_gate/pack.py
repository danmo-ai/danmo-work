"""prompt-pack: write a disk pack, keep gate stdout short (path + write target)."""
from __future__ import annotations

import re
from pathlib import Path

from .common import Report, write_book_text
from .context import plugin_root
from .outline import unit_prose_rel

PACK_DIR = ".pack"
HEADING = re.compile(r"^## Pipeline system\s*$", re.M)

STAGE_WRITE = "write"
STAGE_OUTLINE = "outline"
STAGE_FINALIZE = "finalize"
STAGES = (STAGE_WRITE, STAGE_OUTLINE, STAGE_FINALIZE)

_SKILL_REL = {
    STAGE_WRITE: "skills/novel-write/references/unit-write.md",
    STAGE_OUTLINE: "skills/novel-write/references/unit-outline.md",
    STAGE_FINALIZE: "skills/novel-review/SKILL.md",
}


def extract_pipeline_system(md: str) -> str:
    m = HEADING.search(md or "")
    if not m:
        return ""
    rest = md[m.end() :]
    nxt = re.search(r"^## ", rest, re.M)
    body = rest[: nxt.start()] if nxt else rest
    return body.strip()


def pipeline_system_for(stage: str) -> str:
    rel = _SKILL_REL.get((stage or "").strip().lower())
    if not rel:
        return ""
    path = plugin_root() / rel
    if not path.is_file():
        return ""
    try:
        return extract_pipeline_system(path.read_text(encoding="utf-8"))
    except OSError:
        return ""


def pack_filename(stage: str, unit_id: str = "", volume: str = "") -> str:
    st = (stage or "").strip().lower()
    if st == STAGE_OUTLINE:
        return f"outline-{volume or 'vNN'}.md"
    if st == STAGE_FINALIZE:
        return f"finalize-{unit_id or 'unit'}.md"
    return f"write-{unit_id or 'unit'}.md"


def write_hint(stage: str, unit_id: str = "", volume: str = "") -> str:
    st = (stage or "").strip().lower()
    if st == STAGE_OUTLINE:
        vol = volume or "vNN"
        return f"outline/units/{vol}-U#.yaml (this batch ≤4 proposed)"
    if unit_id:
        if st == STAGE_FINALIZE:
            return (
                f"{unit_prose_rel(unit_id)}; continuity/summaries + facts + "
                f"commits/{unit_id}.md (one Commit patch)"
            )
        return unit_prose_rel(unit_id)
    return "(see pack)"


def materialize_pack(
    r: Report,
    book_root: Path,
    stage: str,
    *,
    unit_id: str = "",
    volume: str = "",
) -> str:
    """Move CONTEXT / extra sections / counts onto disk; stdout keeps PACK pointer.

    Writes even on FAIL so repair can read the pack. Returns book-relative path.
    """
    system = pipeline_system_for(stage)
    has_body = bool(system or r.context_lines or r.extra_sections or r.counts)
    if not has_body:
        return ""
    parts: list[str] = []
    if system:
        parts.append("## Pipeline system\n\n" + system)
    if r.context_lines:
        parts.append("## CONTEXT\n\n" + "\n".join(r.context_lines))
    for title, body in r.extra_sections:
        parts.append(f"## {title}\n\n" + "\n".join(body if body else ["None."]))
    if r.counts:
        count_lines = [f"{k}: {v}" for k, v in sorted(r.counts.items())]
        parts.append("## COUNTS\n\n" + "\n".join(count_lines))
    name = pack_filename(stage, unit_id, volume)
    rel = f"{PACK_DIR}/{name}"
    dest = book_root / PACK_DIR / name
    dest.parent.mkdir(parents=True, exist_ok=True)
    write_book_text(dest, "\n\n".join(parts).rstrip() + "\n")
    r.context_lines = []
    r.extra_sections = []
    r.counts = {}
    hint = write_hint(stage, unit_id, volume)
    st = (stage or "").strip().lower()
    do = "do: read_file the pack file only; then write the target; stop"
    forbid = "forbid: read_skill long refs, glob/scan tree, second-read YAML, search_kb"
    if st == STAGE_WRITE:
        do = (
            "do: read_file PACK CONTEXT only → write one units/vNN-U#.md (overwrite; do not read old prose) "
            "→ set outline drafted → stop. No preflight, no qc-pack."
        )
        forbid = (
            "forbid: read_skill, search_kb, glob, reread YAML/cast/facts/style-fingerprint, "
            "preflight, qc-pack, read existing prose first"
        )
    elif st == STAGE_OUTLINE:
        do = (
            "do: read_file PACK → fill ≤4 proposed YAML → lint-units ONCE. "
            "PASS: stop (YAML frozen). FAIL: fix those units, lint ONCE more, then stop."
        )
        forbid = (
            "forbid: read_skill, read cast cards, glob, write prose, lint-units>2"
        )
    elif st == STAGE_FINALIZE:
        do = (
            "do: read PACK (EXPAND anchors + HITS only; do not reread full prose) → "
            "one edit batch → qc-pack ONCE. PASS: prose FROZEN, commit COMMIT-card files only, postcommit. "
            "FAIL: stop. Never edit units/*.md after PASS. Never a second qc-pack."
        )
        forbid = (
            "forbid: read_skill, glob, search_kb, reread facts/yaml/cast, "
            "qc-pack>1, edit prose after PASS, until-exit-0"
        )
    r.section(
        "PACK",
        [
            f"file: {rel}",
            f"write: {hint}",
            do,
            forbid,
        ],
    )
    return rel

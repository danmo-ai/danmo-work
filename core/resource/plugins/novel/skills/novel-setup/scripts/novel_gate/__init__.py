"""Deterministic novel write-gate (package). Stdlib only. Invoked via exec_shell:

python3 novel_gate.py --action ACTION --workdir PROJECT [--book-id SLUG] [--unit vNN-U#] [--volume vNN] [--json]

Actions: doctor | init | accept-volume | lint-units | cast-lint | preflight | qc-pack |
precommit | scan-deslop | postcommit | migrate. Exit 0 PASS, 1 FAIL, 2 usage/error.

Modules: common (YAML/IO/Report), outline (unit YAML + volume index, accept-volume,
lint-units), cast (cards, cast-lint, promote), ledger (facts / summaries / postcommit),
context (state, KB articles, preflight CONTEXT), deslop + qc (scans, precommit, qc-pack),
doctor, init, migrate, book (cache), cli.
"""
from __future__ import annotations

from .book import BookCache
from .cast import (
    CAST_ANCHOR_RE,
    CAST_KV_RE,
    CastCard,
    check_cast_lint,
    load_cast_anchors,
    load_cast_cards,
    load_cast_relations,
    parse_cast_card,
    promote_cards,
)
from .cli import ACTIONS, main, run, run_with_hits
from .common import (
    UNIT_ID_RE,
    Report,
    _load_yaml_map_fallback,
    load_state,
    load_yaml_map,
    read_book_text,
    resolve_book,
    rune_count,
    write_book_text,
)
from .context import (
    CRAFT_LANES,
    GENRES,
    QC_PROFILES,
    STYLE_MAX_RUNES,
    build_preflight_context,
    check_preflight,
    craft_lane_of,
    genre_articles,
    kb_cite_errors,
    knowledge_article,
    plugin_kb_cite_errors,
    plugin_root,
    render_unit_card,
    style_fingerprint_brief,
    validate_state_fields,
)
from .deslop import (
    BANNED_PHRASES,
    LEVEL_ONE,
    TOXIC,
    DeslopHit,
    apply_deslop_to_report,
    iter_deslop_hits,
    scan_deslop,
)
from .doctor import check_doctor
from .init import init_book
from .ledger import (
    MAX_OPEN_DEBTS,
    check_postcommit,
    extract_chapter_summary_block,
    ledger_path,
    load_locked_terms,
    scan_locked_terms,
    split_chapter_summary_blocks,
    summaries_rel,
    summary_has_five_keys,
)
from .migrate import run_migrate
from .outline import (
    BEAT_NAMES,
    HOOK_TYPES,
    accept_volume,
    chapter_range_of,
    lint_units,
    list_unit_ids,
    load_unit,
    parse_volume_cast,
    parse_volume_index,
    seed_unit_yaml,
    split_unit_prose,
    state_delta_who,
    unit_listed,
    validate_unit_shape,
)
from .qc import check_precommit, check_qc_pack, check_scan_deslop

__all__ = [name for name in dir() if not name.startswith("__")]

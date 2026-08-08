# Chapter contract（章合同）

**No prose without an accepted contract** for that chapter (or an explicit user waiver).

Official name: **章合同** / chapter contract. Do not introduce other product names for this artifact.

## Canonical path & format

| Rule | Value |
|------|--------|
| Path | `novel/<book-id>/chapters/chNNN-contract.yaml` (zero-pad chapter to 3+ digits) |
| Format | **YAML only** — copy `assets/templates/chapter-contract.yaml` via `read_skill` |
| Forbidden | Markdown contracts, `outline/*-contract.*`, alternate directories or suffixes |

`table_upsert` into `chapter_contracts` is an **index/mirror** (queryable status), not a substitute for the YAML file. Always write the file first; table row should point at that path (e.g. `file: chapters/ch001-contract.yaml`).

Book/volume planning stays in `outline/` (`outline.md`). Per-chapter acceptance specs are **only** chapter contracts under `chapters/`.

## Template fields

See `assets/templates/chapter-contract.yaml`. Minimum:

| Field | Purpose |
|-------|---------|
| `chapter` / `title_working` | Identity |
| `pov` / `time` / `location` | Scene anchors |
| `purpose` | One-sentence chapter job |
| `must_happen` | Beats that must land |
| `must_not_happen` | Ban list / spoilers to avoid |
| `character_state_in` / `out` | Deltas only |
| `reveals` / `secrets_preserved` | Information control |
| `foreshadowing` | Plant or advance FS-ids |
| `hook_out` | Concrete open loop |
| `continuity_risks` | What could break |
| `status` | `proposed` → `accepted` → `drafted` → `reviewed` |

## Process

1. Draft contract at `chapters/chNNN-contract.yaml` (YAML template).  
2. `table_upsert` `chapter_contracts` with matching `book_id` / `chapter` / `status` / `file`.  
3. `ask_user` to accept if this is a milestone chapter or first chapter of a batch.  
4. Only then proceed to `chapter-write.md`.

## Status vocabulary

`missing | in_progress | ready | stale | blocked` for artifacts in `novel-state.yaml`.

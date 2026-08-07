# Chapter contract

**No prose without an accepted contract** for that chapter (or an explicit user waiver).

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

1. Draft contract (`chapters/chNNN-contract.yaml`, or `outline/`, or `table_upsert` `chapter_contracts`).  
2. `ask_user` to accept if this is a milestone chapter or first chapter of a batch.  
3. Only then proceed to `chapter-write.md`.

## Status vocabulary

`missing | in_progress | ready | stale | blocked` for artifacts in `novel-state.yaml`.

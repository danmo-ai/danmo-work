# Chapter write (Draft A)

## Preflight

1. Read `novel-state.yaml` — confirm next chapter index.  
2. **knowledge_gate:** `search_kb` for style / 爽点 / pacing relevant to this chapter.  
3. **asset_gate:** `table_query` characters/locations/foreshadows involved; all on-page major cast should be `canon` (or user-approved exception).  
4. Load accepted contract from `novel/<book-id>/chapters/chNNN-contract.yaml` (`status=accepted` or user waiver).  
5. `memory_read` prefs + recent summaries (last 3–5 chapters).  

## Context package (optional file)

If you write a context note, list **source paths/ids** only; do not fork Canon into a new editable truth.

## Draft

- `write` `novel/<book-id>/chapters/chNNN.md` (zero-pad to 3+ digits).  
- Honor `beats` / `forbidden` and POV knowledge boundary.  
- Land `pleasure_point`; end on `hook.out` (event, not chicken-soup). Respect `word_target` (番茄 2000–3500 unless user says otherwise).
- Modes: **full** (default) / **fast** (shorter, still contracted) — only if user asks.

## After draft

1. Update `chapters/chNNN-contract.yaml` `status=drafted` (and mirror `table_upsert` if used).  
2. Immediately run **one** review round (`review-gates.md`). Do not skip.  
3. Do not start the next chapter until Commit or user explicitly queues a batch with stop rules.

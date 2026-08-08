# Outline

## Hierarchy

1. **Book outline** — acts/volumes, endgame direction, major reversals  
2. **Volume outline** — volume goal, midpoint, climax, open threads  
3. **Chapter list** — one-line purpose + hook per chapter (not full prose, not a contract)  
4. **Chapter contracts** — separate stage (`chapter-contract.md`); YAML under `chapters/`

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.  
- Mark 爽点 / hooks on the chapter list (even as tags).  
- `search_kb` 节奏与结构 before locking volume shape (knowledge_gate).  
- After user OK on volume outline, update `novel-state.yaml` and `memory_update` project checkpoint.  
- Do not batch-write chapters until asset_gate: core cast + world skeleton are `canon`.  
- Per-chapter acceptance specs belong in **章合同** (`chapters/chNNN-contract.yaml`), never under `outline/`.  
- Write outline files only under `outline/` (use `outline/volumes/` for per-volume briefs).

## Outputs

- `novel/<book-id>/outline/book_outline.md`  
- `outline/volumes/vXX.md` (or equivalent names under `outline/volumes/`)  
- Chapter list may live inside those outline files (one line per chapter)  
- Near-term contracts: leave to the contract stage (file + optional `table_upsert` mirror)

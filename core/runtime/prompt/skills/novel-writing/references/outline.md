# Outline

## Hierarchy

1. **Book outline** — acts/volumes, endgame direction, major reversals  
2. **Volume outline** — volume goal, midpoint, climax, open threads  
3. **Chapter list** — one-line purpose + hook per chapter (not full prose)  
4. **Chapter contracts** — detailed only for the next 1–3 chapters  

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.  
- Mark 爽点 / hooks on the chapter list (even as tags).  
- `search_kb` 节奏与结构 before locking volume shape (knowledge_gate).  
- After user OK on volume outline, update `novel-state.yaml` and `memory_update` project checkpoint.  
- Do not batch-write chapters until asset_gate: core cast + world skeleton are `canon`.

## Outputs

- `novel/<book-id>/outline/book_outline.md`  
- `outline/volumes/vXX.md` as needed  
- Optional `table_upsert` rows into `chapter_contracts` for near-term chapters with `status=proposed`

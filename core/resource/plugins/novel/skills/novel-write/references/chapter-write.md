# Chapter write (Draft A)

## Preflight

1. `read_skill` → `preflight.md` — 读取回执 + 更新 `novel-state` gates。
2. Read `novel-state.yaml` — confirm next chapter index.  
3. **knowledge_gate:** `search_kb` 文风与去 AI 味；写章末钩子或章首接钩前再 `search_kb` 爽点与追读（不要复述选型表）。**ch001–ch003** → `read_skill` `opening-chapters.md` + `search_kb` 节奏与结构（黄金三章 + 章内节奏）。
4. **asset_gate:** `table_query` characters/locations/foreshadows involved; all on-page major cast should be `canon` (or user-approved exception).  
5. Load accepted contract from `novel/<book-id>/chapters/chNNN-contract.yaml` (`status=accepted` or user waiver; `unit_id` must match a volume 剧情单元).  
6. `memory_read` prefs + recent summaries (last 3–5 chapters).  
7. 若 beats 含场景标签 → `read_skill` `scene-routing.md`（含 `scene:establish` / `scene:transition` → KB 场景沉浸）。

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

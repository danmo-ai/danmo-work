# Outline

## Hierarchy

1. **Book outline** — 一句话故事, 读者承诺, 分卷结构表, 主线伏笔, 结局方向  
2. **Volume outline** — 卷目标, 核心冲突, 节奏锚点（高潮/中点章位）, 章纲表  
3. **Chapter contracts** — separate stage (`chapter-contract.md`); YAML under `chapters/`

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.  
- Use the templates: `assets/templates/book-outline.md`, `assets/templates/volume-outline.md` — fixed table formats, no free-form reinvention.  
- 章纲表每行必须含「爽点」和「钩子」两列；**连续 3 行无爽点必须重排**（KB 节奏与结构）。  
- `search_kb` 节奏与结构 before locking volume shape (knowledge_gate).  
- After user OK on volume outline, update `novel-state.yaml` and `memory_update` project checkpoint.  
- Do not batch-write chapters until asset_gate: core cast + world skeleton are `canon`.  
- Per-chapter acceptance specs belong in **章合同** (`chapters/chNNN-contract.yaml`), never under `outline/`.  
- Write outline files only under `outline/` (use `outline/volumes/` for per-volume briefs).

## Outputs

- `novel/<book-id>/outline/book_outline.md` — from `book-outline.md` template  
- `outline/volumes/vXX.md` — from `volume-outline.md` template (chapter list lives inside as 章纲表)  
- Near-term contracts: leave to the contract stage (file + optional `table_upsert` mirror)

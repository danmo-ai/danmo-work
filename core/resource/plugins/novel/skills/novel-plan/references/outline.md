# Outline

## Hierarchy

1. **总纲** — 一句话故事, 读者承诺, 分卷结构表, 主线伏笔, 结局方向, **终局储备**（与圣经同一张表）
2. **卷纲** — 卷目标, 冲突与起终, 终局边界, 节奏锚点, **剧情单元（一段章）**, 情绪/人物弧, 反转, 伏笔
3. **章合同** — 下一技能 `novel-write`（`chapter-contract.md`）；YAML under `chapters/`；必填 `unit_id`

卷纲写到「一段章」的剧情单元为止。禁止在 `outline/` 写单章任务/爽点/钩子。

剧情单元表须填：功能、本段主角目标、上因、主爽点形态、禁止提前释放、下一单元钩子。缺表或关键列空，**不要进入章合同**：合同的 `unit_id` / `purpose` / `beats` / `pleasure_point` 必须能指回某个单元 + 锚点。

## Rules

- Plot branches / what-if docs **must not** mutate Canon until the user picks one.
- Use the templates: `assets/templates/book-outline.md`, `assets/templates/volume-outline.md` — fixed sections, no free-form reinvention.
- `search_kb` 节奏与结构（五步锁卷 / 八节点 / 节点法）before locking volume shape (knowledge_gate). 终局台阶与透支两问另查「强约束」；细节只写 `canon/author-lore.md`，总纲抄解锁卷。
- After user OK on volume outline, update `novel-state.yaml` and `memory_update` project checkpoint.
- Do not batch-write chapters until asset_gate: core cast + world skeleton are `canon`.
- **Next stage:** `novel-write` 章合同；批量写正文前再走 `novel-write/references/batch-freeze.md`。单章可经用户确认 bypass 冻结，仍须该章合同 `accepted`。
- Per-chapter planning belongs in **章合同** (`chapters/chNNN-contract.yaml`) only. Never under `outline/`.
- Write outline files only under `outline/` (use `outline/volumes/` for per-volume briefs).

## Outputs

- `novel/<book-id>/outline/book_outline.md` — from `book-outline.md` template（含与圣经一致的终局储备）
- `outline/volumes/vXX.md` — from `volume-outline.md` template（单元表含禁提前 / 下钩；合同 `unit_id` = `vXX-U#`）
- 章合同：交给 `novel-write`（从所属剧情单元下推，文件 + 可选 `table_upsert` 索引）

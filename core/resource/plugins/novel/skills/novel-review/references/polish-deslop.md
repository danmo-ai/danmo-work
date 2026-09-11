# Polish / deslop (Draft C)

Use after review PASS, or when user asks only for deslop.

## Steps

1. `exec_shell` gate `--action scan-deslop --chapter N`（见 `novel-setup/references/gate.md`）。按 `### HITS` 的 `chapters/chNNN.md:L行号:` 定点 `edit`。批扫：`--from A --to B`。
2. `search_kb` / `get_kb_doc`「文风与去 AI 味」(knowledge_gate) — 处理脚本未覆盖的 **P1**。仅 `craft_lane=crime-human` 改查「刑侦人味文风」（加严表 + 改写对照）。禁止只凭 `mystery` 改查。
3. Apply **P0** first（毒句式 1 处即修、一级词密集 ≥3、鸡汤尾），then P1。`crime-human` 另扫：情绪代办词、旁白划线、档案进场、感叹号/单句打点超标。
4. Prefer `edit` for local fixes; `write` only if whole-file rewrite is clearer.
5. Re-run `scan-deslop`（必要时再 `precommit`）；**exit 0** 才可宣称去 AI 味。Do not claim deslop without file changes.

## Focus

- Kill template endings and 套话
- Show-don't-tell（动作顶替情绪、数字顶替形容词）
- Distinct dialogue（遮名测试；刑侦允许低效废话）
- Concrete 章末钩子；过程人味、尾段收紧
- 刑侦：闲笔 ≥ 章内可见；死者生前琐碎；无「心里一凛」族

Do not change plot Canon during polish; if a plot fix is required, return to review/chapter-outline.

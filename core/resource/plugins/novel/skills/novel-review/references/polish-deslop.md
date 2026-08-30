# Polish / deslop (Draft C)

Use after review PASS, or when user asks only for deslop.

## Steps

1. `exec_shell` gate `--action scan-deslop --chapter N`（见 `novel-setup/references/gate.md`）。按 `### HITS` 的 `chapters/chNNN.md:L行号:` 定点 `edit`。批扫：`--from A --to B`。
2. `search_kb` / `get_kb_doc`「文风与去 AI 味」(knowledge_gate) — 处理脚本未覆盖的 **P1**。
3. Apply **P0** first（毒句式 1 处即修、一级词密集 ≥3、鸡汤尾），then P1。
4. Prefer `edit` for local fixes; `write` only if whole-file rewrite is clearer.
5. Re-run `scan-deslop`（必要时再 `precommit`）；**exit 0** 才可宣称去 AI 味。Do not claim deslop without file changes.

## Focus

- Kill template endings and 套话
- Show-don't-tell
- Distinct dialogue
- Concrete 章末钩子

Do not change plot Canon during polish; if a plot fix is required, return to review/contract.

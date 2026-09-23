# Polish / deslop (Draft C)

Use after review PASS, or when user asks only for deslop.

## Steps

1. 用定稿轮开头 `qc-pack --unit vNN-U#` 已打印的 `### HITS`（未跑或改后复扫用 `--action scan-deslop --unit vNN-U#`；见 `novel-setup/references/gate.md`）。按 `### HITS` 的 `units/vNN-U#.md:L行号:` 定点 `edit`。
2. `search_kb` / `get_kb_doc`「文风与去 AI 味」(knowledge_gate) — 处理脚本未覆盖的 **P1**。仅 `craft_lane=crime-human` 改查「刑侦人味文风」（加严表 + 改写对照）。禁止只凭 `mystery` 改查。
3. Apply **P0** first（毒句式 1 处即修、一级词密集 ≥3、鸡汤尾），then P1。`crime-human` 另扫：情绪代办词、旁白划线、档案进场、感叹号/单句打点超标。
4. Prefer `edit` for local fixes; `write` only if whole-file rewrite is clearer.
5. Re-run `qc-pack`（或只 `scan-deslop`）；**exit 0** 且 `polish_needed: no` 才可宣称去 AI 味。Do not claim deslop without file changes.

## Focus

- Kill template endings and 套话
- Show-don't-tell（动作顶替情绪、数字顶替形容词）
- Distinct dialogue（遮名测试；刑侦允许低效废话）
- Concrete 章末钩子；过程人味、尾段收紧
- 刑侦：闲笔 ≥ 章内可见；死者生前琐碎；无「心里一凛」族
- 人名：无同批文艺双字扎堆；无正文临时发明正式全名

Do not change plot Canon during polish; if a plot fix is required, return to the unit outline (`novel-write/references/unit-outline.md`).

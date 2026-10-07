# Unit write（单元正文首稿）

## Pipeline system

写正文只读本 pack 的 CONTEXT（含风格指纹）。禁止 `preflight`、`qc-pack`、`read_skill`、`search_kb`、扫树、二读 YAML、人物卡、facts、`author-lore`、`find`/`glob`、通读已有正文。

1. 若 ephemeral 已 `PACK ready`：直接 `read_file` 包文件。否则钉死 `G="$WORK_HOME/plugins/novel/skills/novel-setup/scripts/novel_gate.py"` 跑 `--action prompt-pack --stage write --unit vNN-U#`。exit ≠ 0 → 停。
2. 只消费 pack。人名闭集 = CONTEXT 已出现称呼。兑现场面 `情:` / `→读:` / `潜:`。指纹已在 CONTEXT，缺失也不要另读 `style-fingerprint.md`。
3. `write` **一份** `units/vNN-U#.md`（覆盖即可，不要先读旧稿）：`## 第N章`，章间单独一行 `---`。细纲 `status=drafted`。停。不定稿。禁止写完再跑 gate。

## Preflight

1. `exec_shell` gate `--action preflight --unit vNN-U#`（`novel-setup/references/gate.md`）。exit ≠ 0 → **停止**（常见：`on_stage` 含 `candidate` / 不在本卷人物 / 无 canon protagonist / 细纲未 `accepted` / 钩子类型与卷纲不一致）。接手旧书另跑 `--action doctor`。
2. 读 stdout 的 `### CONTEXT`，**状态优先**依次：书级一行 → 卷纲索引 + 本卷时间线 → **本单元/上一单元时钟** → **单元卡**（合同/场面/章切口；不含重复 deltas）→ 接钩 → **人物**（`identity@unit` + 三锚点 + 台词；`pov`「不知」）→ **本单元身份转变目标**（勿当开场人设）→ 开放债务 → 锁词 → **题材专有文（可截断）** → 风格指纹 → 加载纪律。**这是本轮唯一额外上下文。**
3. 可选：`search_kb` **至多 1** 次，且只在单元含 ch1–3 时查「节奏与结构」并 `read_skill` `opening-chapters.md` **一次**；或题材截断不足时查同名 KB 篇。其余情况**不查**。
4. 场面 `beat` 含场景标签时，用 `scene-routing.md` 决定是否把那一次 KB 用在「情绪与场景」。不要开第二次 `search_kb`。
5. 仅当细纲 `continuity_risks` 非空 → 才可 `read_file` 点名旧单元正文。

**禁止：** `canon/author-lore.md`、人物卡全文、细纲 YAML 二读、整本 bible 终局细节、全卷纲、全书 `canon/` 通读、facts 全文、`chapters/`。

### 写作前（全部能答才动笔）

1. 工作窗口：场面序 + CONTEXT 给的上一单元末钩是什么？
2. `on_stage` 每人的 **identity@unit** + 三锚点 + 台词都在 CONTEXT 里？开场称呼/职位是否对齐 identity（不是细纲 `to`）？`pov` 的「不知」不得在正文里被他说破？
3. 本单元 FS-id（plant/advance/payoff）与 Open loops 对得上？
4. 世界规则已在 canon/world.md 立过？新规则是否在影响剧情之前建立？
5. 开篇 500 字内是否接住上一单元 `next_hook.out`（首单元除外）？
6. 时间线与上一单元不矛盾（对照 CONTEXT 本单元/上一单元时钟 / `gap_from_prev`）？
7. POV 知情范围：第一人称不知他人想法；全知换头用空行，不用单独一行 `---`？
8. 人名是否都来自 CONTEXT（单元卡 + on_stage 人物）？禁止临时发明正式全名；龙套用工称。
9. 本单元身份 `to` 只在单元过程中兑现，开场不得写穿？

写入 `novel-state.yaml`：

```yaml
gates:
  knowledge: pass|fail|unknown
  asset: pass|fail|unknown
  qc: unknown
blockers: []
active_unit: v01-U1
last_preflight: "[YYYY-MM-DD v01-U1] state:writing | outline:accepted | gate:PASS"
```

## Draft

- `write` **一份** `units/vNN-U#.md`。
- 按 `scenes` 顺序写。每场把 `must_land` 写成动作或对白，不要一句口号收场。
- **同时兑现场面契约**（CONTEXT「场面序」里的 `情:` / `→读:` / `潜:`）：情绪弧要在场面内可感；读者效果靠动作/对白/信息差达成，禁止旁白宣布「他很愤怒」；有对白时表面话题 ≠ 真实诉求（对齐 `subtext`）。只推进情节清单、不落地情绪与读者效果 = 失败。
- 章界只在细纲 `chapters[]` 的切口处断开。格式：

```markdown
## 第1章 工作标题

正文。场景转换用空行，禁止单独一行 ---。

---

## 第2章 工作标题

正文。
```

- 标题行 `## 第N章`，N 与细纲章号一致，顺序一致。除第一章外，标题前（跳过空行）必须是单独一行 `---`。第一章前不要分隔线。
- 对齐 `word_share`（单章 2000–3500）与单元 `word_target`。不要写完场面清单就停。除第一章外，章首接住上一章 `cut_hook`。
- **人名：** 只用 CONTEXT 已出现的称呼。必须升格新名 → 停笔，回 `novel-plan` 补 `candidate` 卡 + 加进本卷人物，再回细纲 `on_stage`。

## After draft

1. 细纲 `status=drafted`。
2. **本轮到此结束。** 勿在同 turn 做扩写 / 去 AI 味 / 审稿 / Commit（留给 `novel-review` 定稿轮：`qc-pack` 一次输出决定扩写 / 润色是否需要）。
3. 建议用户换模后开定稿轮。

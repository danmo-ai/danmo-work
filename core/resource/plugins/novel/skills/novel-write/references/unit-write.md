# Unit write（单元正文首稿）

写正文：**先跑 gate preflight，只消费它打印的 `### CONTEXT`。** CONTEXT 已含渲染后的单元卡，不再第二遍通读 YAML；不读人物卡；不要为走流程扫全书树。

一轮只写 **一个** 单元，落成 **一份** `units/vNN-U#.md`。章是这份文件里的切口，不是单独文件。一份文件是为了写和读时不要丢掉章与章的衔接和一致性，不是为了把单章写长。

风格指纹随 preflight CONTEXT 注入。本轮上下文未见风格指纹 → `read_file canon/style-fingerprint.md`（无则 bible `## Style card`）。

## Preflight

1. `exec_shell` gate `--action preflight --unit vNN-U#`（`novel-setup/references/gate.md`）。exit ≠ 0 → **停止**（常见：`on_stage` 含 `candidate` / 不在本卷人物 / 无 canon protagonist / 细纲未 `accepted` / 钩子类型与卷纲不一致）。接手旧书另跑 `--action doctor`。
2. 读 stdout 的 `### CONTEXT`，依次：风格指纹 → **题材专有文全文**（`genre`；`crime-human` 接「刑侦人味文风」）→ 卷纲索引行 → **单元卡**（function / entry / desire / obstacle / choice / payoff / pleasure / forbidden / reveals / foreshadowing / state_deltas / on_stage / pov / 场面序 / 章切口 / next_hook.out）→ 接钩 → **人物**（仅 `on_stage`：snapshot 行 + 三锚点 + 1 条台词；`pov` 加「不知」；在场关系行）→ 开放债务 → 锁词 → 加载纪律。**这是本轮唯一额外上下文。**
3. 可选：`search_kb` **至多 1** 次，且只在单元含 ch1–3 时查「节奏与结构」并 `read_skill` `opening-chapters.md` **一次**。其余情况**不查**：题材篇与人味篇已在 CONTEXT 里。
4. 场面 `beat` 含场景标签时，用 `scene-routing.md` 决定是否把那一次 KB 用在「情绪与场景」。不要开第二次 `search_kb`。
5. 仅当细纲 `continuity_risks` 非空 → 才可 `read_file` 点名旧单元正文。

**禁止：** `canon/author-lore.md`、人物卡全文、细纲 YAML 二读、整本 bible 终局细节、全卷纲、全书 `canon/` 通读、facts 全文、`chapters/`。

### 写作前（全部能答才动笔）

1. 工作窗口：场面序 + CONTEXT 给的上一单元末钩是什么？
2. `on_stage` 每人的 snapshot 行 + 三锚点 + 台词都在 CONTEXT 里？`pov` 的「不知」不得在正文里被他说破？
3. 本单元 FS-id（plant/advance/payoff）与 Open loops 对得上？
4. 世界规则已在 canon/world.md 立过？新规则是否在影响剧情之前建立？
5. 开篇 500 字内是否接住上一单元 `next_hook.out`（首单元除外）？
6. 时间线与上一单元不矛盾？
7. POV 知情范围：第一人称不知他人想法；全知换头用空行，不用单独一行 `---`？
8. 人名是否都来自 CONTEXT（单元卡 + on_stage 人物）？禁止临时发明正式全名；龙套用工称。

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

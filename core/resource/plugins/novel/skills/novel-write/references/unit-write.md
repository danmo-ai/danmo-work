# Unit write（单元正文首稿）

写正文：**先跑 gate preflight，只消费它打印的 `### CONTEXT` + 本单元细纲。** 不要为走流程扫全书树。

一轮只写 **一个** 单元，落成 **一份** `units/vNN-U#.md`。章是这份文件里的切口，不是单独文件。一份文件是为了写和读时不要丢掉章与章的衔接和一致性，不是为了把单章写长。

风格指纹随 preflight CONTEXT 注入。本轮上下文未见风格指纹 → `read_file canon/style-fingerprint.md`（无则 bible `## Style card`）。

## Preflight

1. `exec_shell` gate `--action preflight --unit vNN-U#`（`novel-setup/references/gate.md`）。exit ≠ 0 → **停止**。接手旧书另跑 `--action doctor`。
2. 读 stdout 的 `### CONTEXT`（风格指纹 / 上一单元末钩 / 人物现场 / 三锚点 / 在场关系 / 开放债务 / 单元功能、入口、欲望、阻碍 / 场面序 / 章切口）。**这是本轮唯一额外上下文。**
3. 读 `outline/units/vNN-U#.yaml`（须 `accepted`；`unit_id` 对上卷纲）。
4. 可选：`search_kb` **至多 1** 次。默认「文风与去 AI 味」。单元含 ch1–3 → **必须**改查「节奏与结构」，并另 `read_skill` `opening-chapters.md` **一次**。`craft_lane=crime-human` 且不含 ch1–3 →「刑侦人味文风」。禁止只凭 `qc_profile=mystery` 改查人味。
5. 场面 `beat` 含场景标签时，本 turn 已选定的那 1 次 KB 只加载命中小节（`scene-routing.md`）。不要为场景再开第二次 `search_kb`。
6. 仅当细纲 `continuity_risks` 非空 → 才可 `read_file` 点名旧单元正文。

**禁止：** `canon/author-lore.md`、整本 bible 终局细节、全卷纲、全书 `canon/` 通读、`chapters/`。

### 写作前（全部能答才动笔）

1. 工作窗口：场面序 + CONTEXT 给的上一单元末钩是什么？
2. 上场角色 Current State 与 Cast snapshot 一致？
3. 本单元 FS-id（plant/advance/payoff）与 Open loops 对得上？
4. 世界规则已在 canon/world.md 立过？新规则是否在影响剧情之前建立？
5. 开篇 500 字内是否接住上一单元 `next_hook.out`（首单元除外）？
6. 时间线与上一单元不矛盾？
7. POV 知情范围：第一人称不知他人想法；全知换头用空行，不用单独一行 `---`？
8. 人名是否都来自细纲 / CONTEXT / cast？禁止临时发明正式全名。

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
- **人名：** 只用细纲 / CONTEXT / cast 已出现的称呼。必须升格新名 → 停笔，先补细纲 + `candidate` 卡。

## After draft

1. 细纲 `status=drafted`。
2. **本轮到此结束。** 勿在同 turn 做扩写 / 去 AI 味 / 审稿 / Continuity Commit（留给 `novel-review`）。
3. Hand off：字数不足 → `expansion.md`；然后审 → 可选润色 → 一次 Commit 整单元。

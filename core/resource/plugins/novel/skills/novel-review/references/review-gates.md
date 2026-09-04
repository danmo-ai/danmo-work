# Review gates (one round)

## Policy

- **Exactly one** review round per draft cycle before Commit.
- Run gate `--action precommit` **before** claiming PASS. Script FAIL → `qc_gate` FAIL。
- Writers own fixes; review diagnoses。
- 审稿报告必须引用 gate `### COUNTS` 四计数（em_dash / ai_vocab / english_leak / simile）。

## Happy path（PASS — 不落盘 review 文件）

precommit PASS、无 P0 craft 问题、且量化评分 ≥85 时：

1. **不要**写 `reviews/chNNN-review.md`。
2. 更新 `novel-state.yaml`：`gates.qc: pass`，清掉本章相关 `blockers`。
3. 章纲保持 `drafted`（或本轮直接进入 Commit 时再改 `reviewed`）。
4. 可选短润色 → `polish-deslop.md`；然后 `continuity-commit.md`。

用户点「深审」或发现 P0 → 走 FAIL / 六镜全文（必须落盘）。

任一项分数 ≤4 且该透镜 blocking → 按 FAIL 全文写盘。

## 量化评分门（10 维加权）

每章按 10 维打分（/10），按下表权重加权为百分制总分：

| 维度 | 权重 | 检查要点 |
|------|------|----------|
| Opening 开篇 | 10% | 前 3 段出现冲突；接住上章 hook.out；无禁用开头（天气/作息/回顾/慢背景/寒暄/设定讲座） |
| Plot & Scene | 15% | 至少一个不可逆事件；场景遵循 目标→冲突→结果；因果链驱动，无巧合推进 |
| Character | 15% | 无 OOC；人物对事件有情绪+行动反应；POV 章至少一次内心冲突或艰难抉择 |
| Dialogue | 10% | 潜台词 ≥30%；无解释型对白；遮名测试通过 |
| Hook & Suspense | 10% | hook 类型不与上章连续同类；关闭/推进一个旧张力并开新张力；悬念强度不连续 3 章下陷 |
| Show vs Tell | 10% | 情绪经身体/动作/对话呈现；闪回 ≤2 段 |
| Pacing & Rhythm | 10% | 句长段长有变化；信息密度高低交替；无中部塌陷/高潮仓促 |
| Sensory & World | 5% | 每场景 ≥3 种感官且 ≥1 非视觉；契诃夫之枪纪律 |
| Language & Anti-AI | 10% | gate 四计数在阈值内；无禁词/毒句式；四字格 ≤2/段 |
| Continuity | 5% | 承接上章 hook.out；身体状态/时间线/伏笔与 ledger 一致；POV 无信息泄漏 |

**门限：**

- **≥85 PASS** → Happy path。
- **75–84 WARNING** → 定向修复简报（只列扣分维度的具体行号证据），修复后复评一次。
- **<75 REVISE** → 详细修复简报，最多 2 轮重写。

### 硬失败（无视总分，直接 REVISE）

1. 破折号密度超限（gate blocking）
2. 英文泄漏（gate blocking）
3. 人名/人物状态与 `canon/cast` 或 ledger Cast snapshot 矛盾
4. 时间线与 ledger 摘要矛盾（顺序/昼夜/旅程时长）

### 升级机制

REVISE 两轮仍不过 → **FORCED PASS**：遗留问题打 severity 标签（`CONTINUITY_BREAK` / `CHARACTER_INCONSISTENCY` / `STYLE_ISSUE` / `AI_FINGERPRINT`）追加进 `reviews/backpatch.md` 回填队列，卷收束或全书组装前必须清零。

## FAIL / 深审 — 六镜全文

仅 FAIL 或用户深审时 `write` `reviews/chNNN-review.md`：

| Lens | Blocking if… |
|------|----------------|
| Structure / purpose | Chapter misses `purpose` or `beats` |
| Character / OOC | Breaks desire/wound, knowledge boundary, or 三锚点 |
| World / lore | Contradicts Canon rules without change request |
| Tension / pacing | Dead air with no intentional蓄势; or hook missing |
| Voice / style | Wrong POV or severe style break |
| Reader / 爽点 | Broken reader promise for this beat (advisory unless catastrophic) |

### Extended lenses（blocking when applicable）

| Lens | Blocking if… |
|------|----------------|
| **ReaderPull** | 开放钩子债务 >5；未在 500 字内接上一章钩；近 5 章无爽点（开篇期 blocking） |
| **PacingDensity** | 番茄向：章内无 300–500 字波动；连续 3 章纯铺垫 |
| **StrongConstraints** | 金手指代劳关键抉择；时间线冲突；活跃叙事线 >3；提前打光终局底牌；无关角色夺走高光 |
| **追更指数** | 章末无可感知悬念且中段无加压（番茄向 ch1–3 blocking） |

番茄向/免费网文额外检查（ch1–3 blocking，之后 advisory）：开篇 3 句内有冲突、首章末必钩、章纲 `pleasure_point` 与 `hook` 非空且正文兑现。

Plus **anti-AI P0** from KB「文风与去 AI 味」— always blocking（含量化硬指标四项）。

```markdown
### VERDICT
FAIL

### BLOCKING
- ...

### ADVISORY
- ...

### COUNTS
em_dash_count: n | ai_vocab_count: n | english_leak_count: n | simile_count: n（引自 gate）

### SCORES
（10 维加权表 + 加权总分 + 门限判定 PASS/WARNING/REVISE）

### QUOTE_GROUNDS
- "..." — lens — note
```

## 卷末 / 全书深审清单（Assembly Checklist）

卷收束或全书组装时逐项核验（配合 `novel-review` 卷收束动作）：

1. 伏笔无 OPEN/DANGLING（OPEN 超 1 卷未推进 = 违规）
2. 线索无无由 ACTIVE；PARKED 有叙事理由且已恢复
3. POV 全书一致；多 POV 换头均有场景分隔
4. 时间线单调、跨章无昼夜/季节错位
5. 角色声音可区分（遮名测试抽查 3 章）
6. 契诃夫之枪：重点描写的物件全部兑现
7. 首章钩子仍成立；结尾与开篇形成回响
8. 全文反 AI 复扫（`scan-deslop --from 1 --to N`）exit 0
9. 场景经济性：无"无事发生的过渡章"
10. 世界规则一致性：无未登记的规则例外
11. hook 系统：每章 hook-out 均被下章 hook-in 承接
12. 人物弧光完成度：Arc 起点→终点兑现（含拒绝改变的反向弧）
13. 卷弧完整：卷目标与卷高潮对齐卷纲
14. 字数达标：各章 ≥ word_target × 0.6；总量符合立项预期

## QC Profile

读 `novel-state.yaml` 的 `qc_profile`；按 KB「题材与平台」加权. 默认 `general`.

## Severity

- **P0 / blocking:** must fix before Commit
- **P1 / advisory:** can ship；记在 state blockers 或 FAIL 文件即可

## qc_gate

- Gate 脚本 precommit FAIL → FAIL。
- FAIL → 落盘 review → fix → 短复核 blocking → 再 Commit。
- PASS → 不落盘 → polish（可选）或 `continuity-commit.md`。
- 更新 `novel-state.yaml` `gates.qc` 与 `blockers`。

`table_upsert` continuity_issues **optional**（默认不做）。

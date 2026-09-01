# Review gates (one round)

## Policy

- **Exactly one** review round per draft cycle before Commit.
- Run gate `--action precommit` **before** claiming PASS. Script FAIL → `qc_gate` FAIL.
- Writers own fixes; review diagnoses.

## Happy path（PASS — 不落盘 review 文件）

precommit PASS 且无 P0 craft 问题时：

1. **不要**写 `reviews/chNNN-review.md`。
2. 更新 `novel-state.yaml`：`gates.qc: pass`，清掉本章相关 `blockers`。
3. 合同保持 `drafted`（或本轮直接进入 Commit 时再改 `reviewed`）。
4. 可选短润色 → `polish-deslop.md`；然后 `continuity-commit.md`。

用户点「深审」或发现 P0 → 走 FAIL / 六镜全文（必须落盘）。

任一项分数 ≤4 且该透镜 blocking → 按 FAIL 全文写盘。

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

番茄向/免费网文额外检查（ch1–3 blocking，之后 advisory）：开篇 3 句内有冲突、首章末必钩、合同 `pleasure_point` 与 `hook` 非空且正文兑现。

Plus **anti-AI P0** from KB「文风与去 AI 味」— always blocking.

```markdown
### VERDICT
FAIL

### BLOCKING
- ...

### ADVISORY
- ...

### SCORES
| 维度 | 分 | 证据引用 |
|------|-----|----------|
| 钩子力度 | /10 | "..." |
| 逻辑可回溯 | /10 | "..." |
| 人物活感 | /10 | "..." |
| 语言质地 | /10 | "..." |
| 反转锋利度 | /10 | "..." |
| 余味延展 | /10 | "..." |
| 追更指数 | /10 | "..." |

### QUOTE_GROUNDS
- "..." — lens — note
```

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

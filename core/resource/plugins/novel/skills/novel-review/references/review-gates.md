# Review gates (one round)

## Policy

- **Exactly one** review round per draft cycle before Commit.  
- Run gate script `--action precommit` **before** writing VERDICT PASS. Script FAIL → `qc_gate` FAIL.  
- Writers own fixes; review diagnoses.

## Happy path（PASS stub）

If precommit PASS and no P0 craft issues, `write` a **short** `reviews/chNNN-review.md`:

```markdown
### VERDICT
PASS

### BLOCKING
None.

### ADVISORY
- ...

### SCORES
| 维度 | 分 |
|------|-----|
| 钩子力度 | /10 |
| 逻辑可回溯 | /10 |
| 人物活感 | /10 |
| 语言质地 | /10 |
| 反转锋利度 | /10 |
| 余味延展 | /10 |
| 追更指数 | /10 |

### 追更 / 读者期待（可选）
- 章末钩是否可感知：是/否
- 中段加压：有/无
- 一句话：读者为什么想点下一章
```

任一项 ≤4 且该透镜 blocking → 改写为 FAIL 全文（见下）。用户点「深审」或发现 P0 → 全文六镜。

## FAIL / 深审 — 六镜全文

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

FAIL 全文模板：

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
- **P1 / advisory:** can ship with note in review stub  

## qc_gate

- Gate 脚本 precommit FAIL → FAIL.  
- FAIL → fix → short re-check of blocking list only → then Commit.  
- PASS → polish (optional) or `continuity-commit.md`.  
- 更新 `novel-state.yaml` `gates.qc` 与 `blockers`.  

`table_upsert` continuity_issues **optional**.

# Review gates (one round)

## Policy

- **Exactly one** review round per draft cycle before Commit.  
- Do not fake or skip review.  
- Fail only the failed step: if review finds P0, fix those; do not silently rewrite unrelated chapters.  
- Writers own fixes; review diagnoses.

## Six lenses

| Lens | Blocking if… |
|------|----------------|
| Structure / purpose | Chapter misses `purpose` or `beats` |
| Character / OOC | Breaks desire/wound or knowledge boundary |
| World / lore | Contradicts Canon rules without change request |
| Tension / pacing | Dead air with no intentional蓄势; or hook missing |
| Voice / style | Wrong POV or severe style break |
| Reader / 爽点 | Broken reader promise for this beat (advisory unless catastrophic) |

## Extended lenses（blocking when applicable）

| Lens | Blocking if… |
|------|----------------|
| **ReaderPull** | 开放钩子债务 >5；未在 500 字内接上一章钩；近 5 章无爽点（开篇期 blocking） |
| **PacingDensity** | 番茄向：章内无 300–500 字波动；连续 3 章纯铺垫 |
| **StrongConstraints** | 金手指代劳关键抉择；时间线冲突；活跃叙事线 >3 |

番茄向/免费网文书籍额外检查（ch1–3 blocking，之后 advisory）：开篇 3 句内有冲突、首章末必钩、合同 `pleasure_point` 与 `hook` 非空且正文兑现（KB 番茄平台）。

Plus **anti-AI P0** from KB「去 AI 味」— always blocking.

## QC Profile 加权

读 `novel-state.yaml` 的 `qc_profile`；按 KB「题材 QC Profile」加权 SCORES。默认 `general`。

## Severity

- **P0 / blocking:** must fix before Commit  
- **P1 / advisory:** can ship with debt logged in `continuity_issues`  

## Output

`write` `novel/<book-id>/reviews/chNNN-review.md`:

```markdown
### VERDICT
PASS | FAIL

### BLOCKING
- ...

### ADVISORY
- ...

### SCORES
| 维度 | 分 | 证据引用 |
|------|-----|----------|
| ... | /10 | "..." |

### QUOTE_GROUNDS
- "..." — lens — note
```

`### VERDICT` 仅 `PASS` 或 `FAIL`（工作台解析用）。

Also `table_upsert` any `continuity_issues` with `status=open` for blocking items.

## qc_gate

- FAIL → fix → short re-check of blocking list only → then Commit path.  
- PASS → proceed to polish (optional) or `continuity-commit.md`.
- 更新 `novel-state.yaml` `gates.qc` 与 `blockers`。

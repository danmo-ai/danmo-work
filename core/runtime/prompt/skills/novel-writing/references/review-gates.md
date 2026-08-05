# Review gates (one round)

## Policy

- **Exactly one** review round per draft cycle before Commit.  
- Do not fake or skip review.  
- Fail only the failed step: if review finds P0, fix those; do not silently rewrite unrelated chapters.  
- Writers own fixes; review diagnoses.

## Six lenses

| Lens | Blocking if… |
|------|----------------|
| Structure / purpose | Chapter misses `purpose` or `must_happen` |
| Character / OOC | Breaks desire/wound or knowledge boundary |
| World / lore | Contradicts Canon rules without change request |
| Tension / pacing | Dead air with no intentional蓄势; or hook missing |
| Voice / style | Wrong POV or severe style break |
| Reader / 爽点 | Broken reader promise for this beat (advisory unless catastrophic) |

Plus **anti-AI P0** from KB「去 AI 味」— always blocking.

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

### QUOTE_GROUNDS
- "..." — lens — note
```

Also `table_upsert` any `continuity_issues` with `status=open` for blocking items.

## qc_gate

- FAIL → fix → short re-check of blocking list only → then Commit path.  
- PASS → proceed to polish (optional) or `continuity-commit.md`.

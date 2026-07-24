---
name: debugging
description: Systematic root-cause debugging before proposing fixes, plus evidence-gated completion. Use when encountering errors, bugs, unexpected behavior, build/test failures, or when about to claim something is fixed or passing.
license: MIT
compatibility: Requires read_file, grep, and usually exec_shell
metadata:
  author: danmo-work
  version: "2.0"
  category: coding
  adapted_from: "https://github.com/obra/superpowers/tree/main/skills/systematic-debugging"
  also_adapted_from: "https://github.com/obra/superpowers/tree/main/skills/verification-before-completion"
  upstream_license: MIT
---

# Debugging Skill

> Adapted from obra/superpowers `systematic-debugging` and `verification-before-completion` (© Jesse Vincent / contributors, MIT).
> Rewritten for Danmo Work tools and agents; not a verbatim copy.

**Iron law:** no fixes without root-cause investigation first. **Completion law:** no “fixed/passing/done” claims without fresh verification evidence.

## Workflow

### Phase 1 — Root cause (before any fix)

1. **Read errors completely** — stack traces, exit codes, first failing assertion.
2. **Reproduce** — run the failing command/test via `exec_shell` when available. If not reproducible, gather more data; do not guess.
3. **Locate** — use `grep` / `read_file` / recent git history to find the failing path.
4. **Trace to source** — where does the bad value/state originate? Fix the source, not only the symptom.
5. **Multi-layer systems** — if CI → build → runtime or API → service → DB, identify *which boundary* breaks before changing code.

Stop Phase 1 only when you can state the root cause in one sentence with evidence.

### Phase 2 — Minimal fix

- Prefer `edit` / `apply_patch` for targeted changes.
- Do not “improve” unrelated code while fixing.
- If the bug needs a regression test, write the failing test first when TDD skill applies.

### Phase 3 — Verify (evidence before claims)

Before saying fixed, passing, or complete:

1. Identify the command that proves the claim (`make test`, `go test ./...`, the original failing command, etc.).
2. Run it freshly in this turn.
3. Read full output and exit code.
4. Claim only what the output supports — quote the evidence.

| Claim | Requires |
|-------|----------|
| Bug fixed | Original symptom re-run passes |
| Tests pass | Fresh test run, 0 failures |
| Build OK | Fresh build, exit 0 |

### Phase 4 — No regression

- Run related tests / lint / typecheck when the project has them.
- See `references/patterns.md` for language-specific checks.

## Anti-patterns

- Proposing a fix before reproducing or reading the stack trace
- Symptom patches (“add nil check”) when the real bug is upstream
- “Should work now” / “looks fine” without running verification
- Claiming success from a previous run or from memory
- Thrashing multiple unrelated changes hoping one sticks

## Stop condition

Deliver: root cause, change summary, and verification evidence (command + result). Do not claim done without that evidence.

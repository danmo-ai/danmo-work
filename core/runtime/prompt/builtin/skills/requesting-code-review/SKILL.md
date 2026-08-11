---
name: requesting-code-review
source: builtin
description: Prepare a focused code-review package (scope, requirements, diff evidence) before merge or after major work. Use when completing tasks, finishing features, or verifying work meets requirements.
license: MIT
compatibility: Requires read_file, grep, glob; git via exec_shell when available
metadata:
  author: danmo-work
  version: "1.0"
  category: coding
  adapted_from: "https://github.com/obra/superpowers/tree/main/skills/requesting-code-review"
  upstream_license: MIT
---

# Requesting Code Review

> Adapted from obra/superpowers `requesting-code-review` (© Jesse Vincent / contributors, MIT).
> Rewritten for Danmo Work tools and agents; not a verbatim copy.

Review early against the *work product*, not the author’s chat history. Give the reviewer precise scope and evidence.

## When

**Mandatory:** after a major feature, before merge to main, after complex bug fixes.

**Valuable:** when stuck, before large refactors, at plan task checkpoints.

## Package the review

1. **Scope** — one paragraph: what changed and why.
2. **Requirements** — the plan/spec/acceptance criteria being checked.
3. **Base / head** — SHAs or clear branch/diff bounds:

```bash
git rev-parse HEAD
git merge-base HEAD origin/main 2>/dev/null || git merge-base HEAD main
git diff --stat <base>..<head>
```

4. **Key files** — list paths the reviewer must read fully.
5. **Risks** — security, data loss, concurrency, migration, API compatibility.
6. **Verification already run** — exact commands and results (see debugging skill).

## If reviewing yourself (Reviewer agent)

Walk every modified file; cite path + line; separate Critical / Important / Minor; do not speculate without evidence. Output a structured report the parent can act on.

## Acting on feedback

- Fix Critical immediately; fix Important before proceeding
- Note Minor for later
- Push back only with technical evidence

## Anti-patterns

- Skipping review because “it’s small”
- Sending the whole session transcript instead of a diff package
- Ignoring Critical/Important findings
- Claiming ready to merge without fresh verification evidence

## Stop condition

Deliver the review package (or the review report). Do not merge until Critical/Important items are resolved or explicitly waived by the user.

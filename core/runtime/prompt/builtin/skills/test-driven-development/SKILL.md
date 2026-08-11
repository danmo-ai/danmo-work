---
name: test-driven-development
source: builtin
description: RED-GREEN-REFACTOR before writing production code. Use when implementing features, bug fixes, behavior changes, or refactoring — before writing implementation code.
license: MIT
compatibility: Requires project test runner via exec_shell when available; implementer may write tests with write/edit only
metadata:
  author: danmo-work
  version: "1.0"
  category: coding
  adapted_from: "https://github.com/obra/superpowers/tree/main/skills/test-driven-development"
  upstream_license: MIT
---

# Test-Driven Development

> Adapted from obra/superpowers `test-driven-development` (© Jesse Vincent / contributors, MIT).
> Rewritten for Danmo Work tools and agents; not a verbatim copy.

**Iron law:** no production code without a failing test first. If you wrote implementation before a failing test, delete it and start over.

## When to use

Always for new features, bug fixes, behavior changes, and refactoring.

Exceptions (ask the user): throwaway prototypes, generated code, pure config/docs.

## Cycle

### 1. RED — write one failing test

- One behavior, clear name, real code (mocks only if unavoidable).
- Prefer the project’s existing test layout and runner.

### 2. Verify RED

- Run the test (`exec_shell` when available).
- Confirm it fails for the right reason (feature missing), not a typo/import error.
- If it passes immediately, you are testing existing behavior — fix the test.

### 3. GREEN — minimal code

- Smallest change to pass that one test.
- No extra features, no drive-by refactors.

### 4. Verify GREEN

- Re-run the test; keep the suite green.

### 5. REFACTOR

- Clean names/duplication only while tests stay green.
- Then next failing test.

## Bug fixes

Write a failing regression test that reproduces the bug, then GREEN, then REFACTOR.

## Anti-patterns

- “I’ll add tests after” — tests that pass on first run prove little
- Keeping pre-written code as “reference” while writing tests (that is tests-after)
- Over-mocking so the test asserts mock behavior, not product behavior
- One giant test covering many behaviors
- Rationalizing “too simple to test”

## Checklist before done

- [ ] Each new behavior has a test that failed first
- [ ] Minimal implementation only
- [ ] Suite green with fresh run evidence
- [ ] Edge/error cases covered when relevant

## Stop condition

State what was tested, the red→green evidence, and files touched. Do not claim TDD if you skipped verify-RED.

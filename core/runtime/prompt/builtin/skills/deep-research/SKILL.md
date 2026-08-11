---
name: deep-research
source: builtin
description: Disciplined multi-source research with triangulation, citations, and adversarial review. Use for high-stakes questions, comparisons, strategy groundwork, or hypothesis validation — not quick fact-checks.
license: MIT
compatibility: Requires web_search, web_fetch, write; read_file/grep for existing project context
metadata:
  author: danmo-work
  version: "1.0"
  category: work
  adapted_from: "https://github.com/alirezarezvani/claude-skills/tree/main/research/deep-research/skills/deep-research"
  upstream_license: MIT
---

# Deep Research

> Adapted from alirezarezvani/claude-skills `deep-research` (© Alireza Rezvani / contributors, MIT).
> Rewritten for Danmo Work tools and agents; not a verbatim copy.

Use when a wrong answer is expensive. Prefer a short direct answer for low-risk fact checks.

## Pipeline

1. **Reframe** — restate the decision behind the question; list 2–4 falsifiable hypotheses.
2. **Plan** — scope, report genre (qa / decision / landscape / validation), stop criteria, opposition queries.
3. **Existing work** — search the workspace/KB first; do not re-research what you already have.
4. **Search** — multiple `web_search` queries; fetch primary sources with `web_fetch`. Prefer diverse source types (primary, docs, academic/industry, reputable discussion).
5. **Persist sources** — when writing files is allowed, save each source (metadata + verbatim quotes) under a research folder; never invent URLs.
6. **Triangulate** — each major thesis needs ≥3 independent sources of different types, or mark “insufficient evidence”.
7. **Synthesize + adversarial** — answer hypotheses; steel-man counter-arguments; note confidence and gaps.
8. **Deliver** — final report with citations; optional `refresh_targets` (entities/numbers to revisit later).

## Output shape (when persisting)

```
research/<slug>/
  plan.md
  sources/01_....md
  report.md
```

If file writes are not available, keep the same structure in the chat report and quote sources with URLs.

## Anti-patterns

- One-shot search then confident prose
- Fabricated or unfetched citations
- Skipping the adversarial pass on medium/deep work
- Treating same-outlet reprints as independent sources
- Dumping only chat text when the user asked for reusable research files

## Stop condition

Report that confirms/refutes each hypothesis (or marks under-determined), with citations and explicit confidence. Do not propose implementation unless asked.

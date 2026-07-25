---
name: document-writing
description: Structure and produce long-form workplace documents (reports, RFCs, specs, explainers) as Markdown and optional single-file HTML. Use when writing or converting substantial documents — not for code files or slide decks.
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danmo-work
  version: "1.0"
  category: work
  adapted_from: "https://github.com/alirezarezvani/claude-skills/tree/main/markdown-html/skills/md-document"
  upstream_license: MIT
---

# Document Writing

> Adapted from alirezarezvani/claude-skills `md-document` (© Alireza Rezvani / contributors, MIT).
> Rewritten for Danmo Work (no upstream design-system/scripts required); not a verbatim copy.

Produce clear long-form documents. Prefer Markdown as the source of truth; optionally emit a self-contained HTML reader.

## Workflow

1. **Purpose** — skim, decide, or deep-read? Audience and success criteria.
2. **Outline** — H1 title + H2 sections before drafting body prose.
3. **Draft Markdown** — headings hierarchy, short paragraphs, tables where they clarify, fenced code with language tags, callouts as blockquotes when useful.
4. **Self-edit** — remove filler; check consistency of terms; no TBD left behind.
5. **Optional HTML export** — if the user wants a shareable **read-only** page, write one `.html` file with:
   - inlined CSS
   - sticky or top TOC from H2+
   - readable typography
   - no framework runtime (vanilla HTML/CSS/JS only)
   - HTML is **not** the edit source; keep `.md` as the source of truth
6. **Deliver paths** — report written file paths and note they open in **Doc Stage** (Files → click the `.md`).

## [office-edit] turns

When the user message starts with `[office-edit]` and `kind: doc`:

1. `read_file` the given `path` (GFM Markdown).
2. Apply `action` / `instruction` to the `selection` (or full doc if selection is the whole file).
3. Prefer `edit` for local replacements; `write` only when replacing the whole file is clearer.
4. Do **not** rewrite as HTML or invent a parallel JSON doc format.
5. SUMMARY: describe what changed (sections / headings).

## Structure defaults

| Doc type | Skeleton |
|----------|----------|
| Report | Summary → Context → Findings → Recommendations → Appendix |
| RFC / Spec | Status → Motivation → Design → Alternatives → Risks → Rollout |
| Explainer | Problem → Mental model → Walkthrough → Pitfalls → References |

## When not to use

- Slide decks → `playable-slides`
- Emails / chat polish → Comms agent persona
- Source code / config → Implementer

## Anti-patterns

- Walls of unsectioned text
- Card-heavy / dashboard-like noise in a prose doc
- HTML that depends on a local build step or heavy frameworks
- Claiming “done” without the actual file written

## Stop condition

Deliver Markdown (and HTML if requested) with a one-line summary of audience + purpose. Stop; do not invent next documents unless asked.

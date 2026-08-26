---
name: document-writing
source: builtin
description: "Long-form workplace docs as GFM `.md` (default). Not for slide decks or spreadsheets — those are Univer IR / CSV. Edit with write/edit/apply_patch; do not invent parallel JSON docs unless path is already `.udoc.json`."
license: MIT
compatibility: Requires write, edit, apply_patch, read_file
metadata:
  author: danmo-work
  version: "2.0"
  category: work
  adapted_from: "https://github.com/alirezarezvani/claude-skills/tree/main/markdown-html/skills/md-document"
  upstream_license: MIT
---

# Document Writing

> Adapted from alirezarezvani/claude-skills `md-document` (© Alireza Rezvani / contributors, MIT).
> Rewritten for Danmo Work; not a verbatim copy.

## Format matrix (read first)

| Artifact | Extension | Skill |
|----------|-----------|--------|
| Report / RFC / explainer / notes | **`.md`** (GFM) | **this skill** |
| Optional shareable reader | `.html` (read-only export; `.md` stays SoT) | this skill |
| Rich Univer document (from Stage / `.docx` convert) | `.udoc.json` | edit existing path only — see `references/format-matrix.md` |
| Slide deck | `.uslides.json` | `playable-slides` — **never** `*-slides.md` |
| Table | `.csv` or `.usheet.json` | `sheet-writing` |

**Default for new prose is always `.md`.** Do not create `.udoc.json` / `.uslides.json` / `.usheet.json` from this skill unless the user path already is that IR file (office-edit) or they explicitly demand Univer doc IR.

## Hard rules

1. Edit with **`read_file` + `edit` / `apply_patch` / `write`** only.
2. Do **not** `web_search` Univer to invent a document JSON format for a Markdown task.
3. Do **not** deliver a slide deck as Markdown “for Stage”.

## Workflow (new `.md`)

1. Purpose + audience.
2. Outline H1 + H2.
3. Draft GFM Markdown.
4. Self-edit; no TBD.
5. Optional one-file HTML export (inlined CSS; HTML is not SoT).
6. Deliver paths (Files → Doc Stage opens `.md`).

## [office-edit] turns

When `[office-edit]` and `kind: doc`:

1. `read_file` `path`.
2. If path ends with `.md` / `.markdown`: patch Markdown text.
3. If path ends with `.udoc.json`: patch Univer `IDocumentData` inside the Danmo envelope (`danmo.format = "univer-doc"`). Prefer changing `snapshot.body.dataStream` + paragraph indices carefully, or full `write` of a coherent snapshot. Do **not** convert the file to `.md` unless asked.
4. If path is `.docx` / `engine: ms-office`: do not write OOXML; ask to convert to `.udoc.json` first (or stop with blocker).
5. SUMMARY: sections / headings (or IR fields) changed.

## Structure defaults

| Doc type | Skeleton |
|----------|----------|
| Report | Summary → Context → Findings → Recommendations → Appendix |
| RFC / Spec | Status → Motivation → Design → Alternatives → Risks → Rollout |
| Explainer | Problem → Mental model → Walkthrough → Pitfalls → References |

## When not to use

- Slides → `playable-slides`
- Tables → `sheet-writing`
- Code / config → Implementer
- Email polish → Comms tone still OK here, but keep as message text unless asked for a file

## Anti-patterns

- Walls of unsectioned text
- Marp / `type: slides` Markdown
- Claiming done without writing the file
- Web-learning IR schemas instead of tools + references / KB

## Stop condition

Deliver Markdown (and HTML if requested) with a one-line audience + purpose note. Stop.

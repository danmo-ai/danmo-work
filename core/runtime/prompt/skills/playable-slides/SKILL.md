---
name: playable-slides
description: Create a browser-playable HTML slide deck from an outline or Markdown (keyboard navigation, optional presenter notes). Use when the user wants slides, a talk deck, or a presentation — PPTX is not required.
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danqing-teams
  version: "1.0"
  category: work
  adapted_from: "https://github.com/alirezarezvani/claude-skills/tree/main/markdown-html/skills/md-slides"
  upstream_license: MIT
---

# Playable Slides

> Adapted from alirezarezvani/claude-skills `md-slides` (© Alireza Rezvani / contributors, MIT).
> Rewritten for Danmo Work as self-contained HTML decks (not PPTX); not a verbatim copy.

Ship a **single HTML file** the user can open in a browser and present. Do not require PowerPoint/Keynote unless the user explicitly asks for those formats.

## Workflow

1. **Confirm it’s a deck** — discrete slides with clear boundaries; if it’s a long continuous doc, use `document-writing` instead.
2. **Outline** — titles only first; aim for one idea per slide; split slides that would exceed ~6 bullets / ~40 source lines.
3. **Author source** — Markdown with `---` between slides (or H1-per-slide). Optional HTML comments for notes: `<!-- notes: ... -->`.
4. **Generate HTML** — one self-contained file with:
   - one slide visible at a time
   - keyboard: `→`/`Space`/`PgDn` next; `←`/`PgUp` prev; `Home`/`End`; `P` presenter (notes + next preview if notes exist); `Esc` exit presenter
   - hash deep link (`#3`)
   - progress indicator + slide counter
   - `@media print` → one slide per page
   - inlined CSS/JS; no React/Vue; optional CDN fonts only
5. **Smoke check** — list controls in the delivery message; ensure ≥2 slides.

## Content rules

- Prefer visuals/short phrases over paragraph slides
- Code slides: large font, minimal lines
- Title slide + closing/next-steps when appropriate

## Anti-patterns

- Emitting `.pptx` by default
- One giant scrollable HTML page labeled “slides”
- Decks with a single slide
- Unreadable walls of text per slide
- Framework SPAs that need `npm run build` to present

## Stop condition

Deliver the `.html` path (and source `.md` if kept), plus how to present (open file → arrow keys / F11). Stop.

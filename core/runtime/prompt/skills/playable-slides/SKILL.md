---
name: playable-slides
description: Author slide decks as Marp-compatible GFM Markdown (`type: slides`, `---` page breaks) for Danmo Slides Stage. Office Stage programmatically renders playable HTML — do not invent full HTML decks. Use for talks, pitches, training, or presentations — PPTX is not required.
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danmo-work
  version: "1.3"
  category: work
  adapted_from: "https://github.com/alirezarezvani/claude-skills/tree/main/markdown-html/skills/md-slides"
  upstream_license: MIT
  content_dialect: "marp-compatible-subset"
---

# Playable Slides

> Adapted from alirezarezvani/claude-skills `md-slides` (© Alireza Rezvani / contributors, MIT).
> Rewritten for Danmo Work: Markdown source of truth; HTML is a Stage program derivative.

Produce a **deck** as Markdown only. Office Stage syncs a sibling playable `.html` on save / Present — **you do not generate or “call” that renderer**.

| File | Role |
|------|------|
| `*-slides.md` | **Source of truth** — edit + AI 批改 |
| `*-slides.html` | **Stage-generated** Present artifact (do not author by default) |

Do not require PowerPoint/Keynote unless the user explicitly asks.

For richer Markdown patterns, see `references/md-examples.md`.

## Contract

1. Always write / edit **only** the `.md` (even if the user said “slides” or “PPT”).
2. Prefer `foo-slides.md` under `slides/` when that folder exists.
3. Include YAML frontmatter with `type: slides`.
4. On `[office-edit]`, change only the given Markdown `path`.
5. **Do not** write a full playable HTML deck, SPA, or parallel JSON slide format.
6. Exception: user explicitly asks for a **custom HTML skin** (not the default Stage shell) — then you may write HTML; otherwise never.

## Markdown dialect (Marp-compatible subset)

Aligned with Marp/Marpit facts of life so Stage (and future Marp engines) can render deterministically:

- CommonMark / GFM body
- YAML frontmatter between opening/closing `---`
- Slide breaks: a line that is only `---`
- Speaker notes: `<!-- notes: ... -->`

### Detection (must be Stage-routable)

Satisfy at least one; prefer **both** filename and frontmatter:

- Filename `*-slides.md`, or path contains `/slides/` / `/deck/`, or basename starts with `slides.`
- Frontmatter `type: slides` (required for content-based routing — bare `---` body dividers alone do **not** open as slides; reports use those as section breaks)

Page breaks inside a deck remain a line that is only `---`.

### Skeleton

```markdown
---
type: slides
title: Deck title
theme: default
fragments: true
transition: fade
---

# Title slide

Subtitle or one-line context

<!-- notes: opening line for the speaker -->

---

## Section / idea slide

- Short bullet
- Short bullet
- Short bullet

---

## Closing

Next steps or ask
```

### Page rules

| Rule | Detail |
|------|--------|
| Separator | A line that is exactly `---` between slides (blank lines around it OK) |
| Frontmatter | First `---` / `---` YAML block is **not** a slide; put `type: slides` there |
| Title | First non-empty line of a slide is the thumb title — prefer `#` / `##` |
| Density | One idea per slide; ≤6 bullets; avoid >~40 source lines per slide |
| Notes | Optional `<!-- notes: ... -->` for presenter view (`P` in Present) |
| Fragments | Deck `fragments: true` auto-steps list items (Space); per-page `<!-- fragments: false -->` to disable |
| Layout | `<!-- _class: lead -->` or `columns` |
| Theme | `default` · `light` · `academic` · `moon` |
| Transition | `fade` (default) or `none` |
| Code | Fenced blocks with language tag; ≤8 visible lines |
| Images | Relative paths from the deck file when assets exist in the project |
| Prose | Phrases over paragraphs; no walls of text |

### Present keys (Stage-generated HTML)

| Key | Action |
|-----|--------|
| Space / → | Next fragment, then next slide |
| ← | Previous fragment / slide |
| P | Speaker notes + next-slide peek + timer |
| O | Overview grid |
| Esc | Exit overview / notes |

### Slide shapes (pick intentionally)

- **Title** — H1 + one subtitle line
- **Bullets** — H2 + 3–6 short bullets
- **Code** — H2 + one fenced block
- **Quote / callout** — short blockquote or single bold line
- **Section break** — H1 only (or H1 + one line)
- **Closing** — next steps, decision, or Q&A

## Workflow

1. **Confirm it’s a deck** — discrete slides; if long-form prose, use `document-writing`.
2. **Outline** — titles only (≥2 slides); split overloaded slides.
3. **Author `.md`** — dialect above.
4. **Deliver** — Markdown path; tell the user to open it in Slides Stage and use **Present** (Stage syncs HTML). Do not claim you rendered HTML unless you wrote a custom-skin exception.

## Use cases (Markdown only)

### 1) Product launch (5–8 slides)

Audience: all-hands / customers. Shape: title → problem → product → 2–3 feature slides → proof → CTA.

```markdown
---
type: slides
title: Acme Launch
---

# Acme Launch

Ship faster without the chaos

<!-- notes: thank the pilot team; 12 minutes -->

---

## The problem

- Tooling is scattered
- Handoffs lose context
- Reviews take days

---

## What we're shipping

One workspace for docs, sheets, and decks

---

## CTA

Try it on your next project kickoff
```

### 2) Tech talk with code

Audience: engineers. Keep code slides sparse; put deeper detail in notes.

```markdown
---
type: slides
title: Compaction in TurnRunner
---

# Compaction in TurnRunner

How we keep long sessions under the context budget

---

## When it fires

- Token estimate exceeds threshold
- Before the next model call
- After tool-heavy turns

---

## Core loop

~~~go
if est > threshold {
    compact(history)
}
~~~

<!-- notes: summaries are stored, not discarded raw -->

---

## Takeaways

- Measure before / after tokens
- Never compact the active user turn
- Log compaction reason for debug
```

### 3) Weekly status (short)

Audience: leads. Prefer section breaks + bullets; skip decorative slides.

```markdown
---
type: slides
title: Week 32 Status
---

# Week 32

Platform / Office Stage

---

## Done

- Slides Stage edit + Present
- Sheet CSV open/save
- office-edit for docs

---

## In flight

- Program HTML sync from Markdown
- Marp-compatible dialect

---

## Asks

- Confirm launch audience for v0.9
- Design review Friday
```

### 4) Workshop / training outline

Audience: learners. One skill per slide; end with practice + resources.

```markdown
---
type: slides
title: Writing Playable Decks
---

# Writing Playable Decks

Markdown in → Stage edit → Present

---

## Source of truth

Always the `.md` with `---` page breaks

---

## Practice

Rewrite a 2-page doc into a 6-slide deck

---

## Resources

- Open `*-slides.md` in Slides Stage
- Present: Stage syncs HTML; keys ← → Space P
```

## [office-edit] turns

When the user message starts with `[office-edit]` and `kind: slides`:

1. `read_file` the Markdown `path` (pages separated by `---`).
2. If `page` is set, change **only** that slide fragment (0-based; frontmatter is not a page).
3. Prefer `edit`; keep `type: slides` intact.
4. Do **not** write or regenerate HTML.
5. SUMMARY: which slide(s) changed (titles / indexes).

## Content rules

- Prefer visuals/short phrases over paragraph slides
- Code slides: large font intent, minimal lines
- Title slide + closing / next-steps when appropriate
- Match tone to audience (launch ≠ weekly status ≠ tech talk)

## Anti-patterns

- Emitting `.pptx` by default
- Writing playable `.html` as the primary deliverable
- Markdown without `---` separators (Stage won’t paginate)
- Single-slide “decks”
- Walls of text per slide
- Framework SPAs that need `npm run build` to present
- Using `---` inside a slide body as decoration (it splits pages)

## Stop condition

Deliver the `.md` path and note that **Present** in Slides Stage plays it (HTML synced by the app). Stop.

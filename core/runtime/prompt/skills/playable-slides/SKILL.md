---
name: playable-slides
<<<<<<< HEAD
description: Author slide decks as Marp-compatible GFM Markdown (`type: slides`, `---` page breaks) for Danmo Slides Stage. Office Stage programmatically renders playable HTML — do not invent full HTML decks. Use for talks, pitches, training, or presentations — PPTX is not required.
=======
description: Author slide decks as GFM Markdown (`---` page breaks, optional YAML type: slides) for Slides Stage edit, plus a self-contained playable HTML for Present mode. Use when the user wants slides, a talk deck, pitch, training deck, or presentation — PPTX is not required.
>>>>>>> origin/main
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danmo-work
<<<<<<< HEAD
  version: "1.2"
=======
  version: "1.1"
>>>>>>> origin/main
  category: work
  adapted_from: "https://github.com/alirezarezvani/claude-skills/tree/main/markdown-html/skills/md-slides"
  upstream_license: MIT
  content_dialect: "marp-compatible-subset"
---

# Playable Slides

> Adapted from alirezarezvani/claude-skills `md-slides` (© Alireza Rezvani / contributors, MIT).
<<<<<<< HEAD
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
- Frontmatter `type: slides`
- Body has multiple `---` page breaks
=======
> Rewritten for Danmo Work Office Stage (Markdown edit + HTML present); not a verbatim copy.

Produce a **deck**, not a long doc. Default deliverables are **two files**:

| File | Role in Office Stage |
|------|----------------------|
| `*-slides.md` | **Edit / AI 批改** source of truth |
| `*-slides.html` | **Present** mode (keyboard playable) |

Do not require PowerPoint/Keynote unless the user explicitly asks. Prefer regenerating HTML from Markdown over hand-editing HTML.

For richer Markdown patterns, see `references/md-examples.md`.

## Dual-artifact contract

1. Always write the `.md` first (even if the user only asked for “slides”).
2. Generate a matching `.html` beside it (same basename: `foo-slides.md` → `foo-slides.html`).
3. Put decks under `slides/` when the project already has that folder; otherwise write next to related docs.
4. On `[office-edit]`, edit the `.md` path only; regenerate `.html` if it already exists.
5. Never treat HTML as the edit source; never invent a parallel JSON slide format.

## Markdown source format (canonical)

### Detection (must be Stage-routable)

Office routes a file to **Slides** when any of these hold — satisfy at least one reliably:

- Filename ends with `-slides.md`, or path contains `/slides/` / `/deck/`, or basename starts with `slides.`
- YAML frontmatter includes `type: slides`
- Body has multiple slides separated by a line that is only `---`

**Prefer both** filename `*-slides.md` **and** frontmatter `type: slides`.
>>>>>>> origin/main

### Skeleton

```markdown
---
type: slides
title: Deck title
<<<<<<< HEAD
theme: default
=======
>>>>>>> origin/main
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
<<<<<<< HEAD
| Separator | A line that is exactly `---` between slides |
| Frontmatter | First `---` / `---` YAML block is **not** a slide |
| Title | First non-empty line of a slide is the thumb title — prefer `#` / `##` |
| Density | One idea per slide; ≤6 bullets; avoid >~40 source lines per slide |
| Notes | Optional `<!-- notes: ... -->` |
| Code | Fenced blocks with language tag; ≤8 visible lines |
| Images | Relative paths from the deck file |
| Prose | Phrases over paragraphs |

### Slide shapes
=======
| Separator | A line that is exactly `---` between slides (blank lines around it OK) |
| Frontmatter | Opening `---` / closing `---` block is **not** a slide; put `type: slides` there |
| Title | First non-empty line of a slide is the thumb title — prefer `#` / `##` |
| Density | One idea per slide; aim ≤6 bullets; avoid >~40 source lines per slide |
| Notes | Optional `<!-- notes: ... -->` (HTML comment) for presenter view |
| Code | Fenced blocks with language tag; keep ≤8 lines visible; large intent over completeness |
| Images | Relative paths from the deck file when assets exist in the project |
| Prose | Prefer phrases over paragraphs; no walls of text |

### Slide shapes (pick intentionally)
>>>>>>> origin/main

- **Title** — H1 + one subtitle line
- **Bullets** — H2 + 3–6 short bullets
- **Code** — H2 + one fenced block
<<<<<<< HEAD
- **Quote / callout** — short blockquote
=======
- **Quote / callout** — short blockquote or single bold line
>>>>>>> origin/main
- **Section break** — H1 only (or H1 + one line)
- **Closing** — next steps, decision, or Q&A

## Workflow

<<<<<<< HEAD
1. **Confirm it’s a deck** — discrete slides; if long-form prose, use `document-writing`.
2. **Outline** — titles only (≥2 slides).
3. **Author `.md`** — dialect above.
4. **Deliver** — Markdown path; tell the user to open it in Slides Stage and use **Present** (Stage syncs HTML). Do not claim you rendered HTML unless you wrote a custom-skin exception.

## Use cases (Markdown only)

### 1) Product launch
=======
1. **Confirm it’s a deck** — discrete slides; if continuous long-form, use `document-writing`.
2. **Outline** — titles only (≥2 slides); split overloaded slides.
3. **Author `.md`** — follow the canonical format above.
4. **Generate `.html`** — one self-contained file with:
   - one slide visible at a time
   - keyboard: `→`/`Space`/`PgDn` next; `←`/`PgUp` prev; `Home`/`End`; `P` presenter (notes + next preview if notes exist); `Esc` exit presenter
   - hash deep link (`#3`)
   - progress indicator + slide counter
   - `@media print` → one slide per page
   - inlined CSS/JS; no React/Vue; optional CDN fonts only
5. **Smoke check** — ≥2 slides; Stage can open `.md` for edit and `.html` for Present.
6. **Deliver** — both paths + how to present (Files → `.html` → Present, or open file → arrow keys / F11).

## Use cases (Markdown)

### 1) Product launch (5–8 slides)

Audience: all-hands / customers. Shape: title → problem → product → 2–3 feature slides → proof → CTA.
>>>>>>> origin/main

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

<<<<<<< HEAD
=======
Audience: engineers. Keep code slides sparse; put deeper detail in notes.

>>>>>>> origin/main
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

<<<<<<< HEAD
~~~go
if est > threshold {
    compact(history)
}
~~~

<!-- notes: summaries are stored, not discarded raw -->
=======
```go
if est > threshold {
    compact(history)
}
```

<!-- notes: mention that summaries are stored, not discarded raw -->
>>>>>>> origin/main

---

## Takeaways

- Measure before / after tokens
- Never compact the active user turn
- Log compaction reason for debug
```

<<<<<<< HEAD
### 3) Weekly status
=======
### 3) Weekly status (short)

Audience: leads. Prefer section breaks + bullets; skip decorative slides.
>>>>>>> origin/main

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

<<<<<<< HEAD
- Program HTML sync from Markdown
- Marp-compatible dialect
=======
- MD format guidance for decks
- HTML regenerate after AI edit
>>>>>>> origin/main

---

## Asks

- Confirm launch audience for v0.9
- Design review Friday
```

<<<<<<< HEAD
### 4) Workshop outline
=======
### 4) Workshop / training outline

Audience: learners. One skill per slide; end with practice + resources.
>>>>>>> origin/main

```markdown
---
type: slides
title: Writing Playable Decks
---

# Writing Playable Decks

<<<<<<< HEAD
Markdown in → Stage edit → Present
=======
Markdown in → Stage edit → HTML present
>>>>>>> origin/main

---

## Source of truth

Always the `.md` with `---` page breaks

---

## Practice

Rewrite a 2-page doc into a 6-slide deck

---

## Resources

<<<<<<< HEAD
- Open `*-slides.md` in Slides Stage
- Present: Stage syncs HTML; keys ← → Space P
=======
- `*-slides.md` + `*-slides.html`
- Present mode keyboard: ← → Space P
>>>>>>> origin/main
```

## [office-edit] turns

When the user message starts with `[office-edit]` and `kind: slides`:

<<<<<<< HEAD
1. `read_file` the Markdown `path`.
2. If `page` is set, change **only** that slide fragment (0-based; frontmatter is not a page).
3. Prefer `edit`; keep `type: slides` intact.
4. Do **not** write or regenerate HTML.
5. SUMMARY: which slide(s) changed (titles / indexes).
=======
1. `read_file` the given Markdown `path` (pages separated by `---`).
2. If `page` is set, change **only** that slide fragment (0-based index into split pages; frontmatter is not a page).
3. Prefer `edit` for local replacements; keep frontmatter `type: slides` intact.
4. Regenerate the sibling `*-slides.html` when it already exists.
5. Do **not** rewrite the deck as a single HTML-only artifact or a long doc.
6. SUMMARY: which slide(s) changed (titles / indexes).

## Content rules

- Prefer visuals/short phrases over paragraph slides
- Code slides: large font intent, minimal lines
- Title slide + closing / next-steps when appropriate
- Match tone to audience (launch ≠ weekly status ≠ tech talk)
>>>>>>> origin/main

## Anti-patterns

- Emitting `.pptx` by default
<<<<<<< HEAD
- Writing playable `.html` as the primary deliverable
- Markdown without `---` separators
- Single-slide “decks”
- Walls of text per slide
- Using `---` inside a slide body as decoration (it splits pages)
- Asking the user to run a build step to present

## Stop condition

Deliver the `.md` path and note that **Present** in Slides Stage plays it (HTML synced by the app). Stop.
=======
- Markdown without `---` separators (Stage won’t paginate)
- Only HTML, no `.md` source
- One giant scrollable HTML page labeled “slides”
- Decks with a single slide
- Unreadable walls of text per slide
- Framework SPAs that need `npm run build` to present
- Using `---` inside a slide body as decoration (it splits pages)

## Stop condition

Deliver the `.md` path (edit) and `.html` path (present), plus how to present. Stop.
>>>>>>> origin/main

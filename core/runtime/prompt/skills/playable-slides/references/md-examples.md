# Playable Slides — Markdown examples

Load this file when you need more slide patterns. Keep decks Stage-routable: `type: slides` frontmatter and/or `*-slides.md` filename, pages split by a line that is only `---`.

## Two-column intent (approximate in MD)

Markdown has no real columns; use two short lists under one H2, or split into two slides.

```markdown
---
type: slides
title: Build vs Buy
---

## Build vs buy

**Build**
- Fits our Stage model
- We own the edit path

**Buy**
- Faster to demo
- Lock-in risk
```

## Quote / decision slide

```markdown
---

## Decision

> Ship MD-first decks; HTML is Present-only.

Owner: Platform · By: Friday
```

## Agenda slide

```markdown
---

## Agenda

1. Problem
2. Approach
3. Demo
4. Risks
5. Ask
```

## Before / after

Prefer two slides (clearer thumbs) over one dense comparison.

```markdown
---

## Before

- Docs in chat
- No Present mode
- Edits rewrite whole files

---

## After

- `*-slides.md` in Stage
- HTML Present
- `[office-edit]` per page
```

## Image-led slide

```markdown
---

## Architecture

![Stage flow](../assets/stage-flow.png)

Markdown edit → HTML present
```

## Speaker notes patterns

```markdown
<!-- notes: 30s — don’t read bullets; tell the pilot story -->

<!-- notes: if asked about PPTX: only on explicit request -->
```

## Bad → good rewrite

**Bad** (doc-shaped, won’t paginate well):

```markdown
# Q2 Plan
We will improve Office Stage. First, slides need markdown…
Then HTML present… Also sheets…
```

**Good**:

```markdown
---
type: slides
title: Q2 Plan
---

# Q2 Plan

Office Stage focus

---

## Slides

- MD source of truth
- HTML Present
- Per-page AI edit

---

## Sheets

- CSV default
- Multi-sheet JSON when needed
```

# Playable Slides — Markdown examples

Marp-compatible subset for Danmo Slides Stage. Stage programmatically renders playable HTML — author **Markdown only**.

Keep decks Stage-routable: `type: slides` frontmatter and/or `*-slides.md` filename; pages split by a line that is only `---`.

## Fragments + theme (Present)

```markdown
---
type: slides
title: Stepped bullets
theme: moon
fragments: true
transition: fade
---

<!-- _class: lead -->
# Demo

---

## Why it matters

- Context stays in the file
- Space reveals one bullet at a time
- O opens overview

<!-- notes: pause after the second bullet -->

---

<!-- fragments: false -->
## All at once

- No stepping on this page
```

Present keys: Space (fragment → next slide), P (notes), O (overview).


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

> Ship MD-first decks; HTML is Stage-rendered.

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
- No Present sync
- Edits rewrite whole files

---

## After

- `*-slides.md` in Stage
- Present syncs HTML from Markdown
- `[office-edit]` per page
```

## Image-led slide

```markdown
---

## Architecture

![Stage flow](../assets/stage-flow.png)

Markdown edit → Stage Present
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
- Stage renders Present HTML
- Per-page AI edit

---

## Sheets

- CSV default
- Multi-sheet JSON when needed
```

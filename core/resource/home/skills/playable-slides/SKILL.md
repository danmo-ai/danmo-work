---
name: playable-slides
source: builtin
description: "Author slide decks as Univer IR `.uslides.json` (ISlideData envelope) for Danmo Slides Stage. Use for talks, pitches, training, or presentations — do not write Marp Markdown or HTML decks as source of truth."
license: MIT
compatibility: Requires write, edit, read_file
metadata:
  author: danmo-work
  version: "2.0"
  category: work
---

# Playable Slides (Univer IR)

Produce slide decks as **`.uslides.json`** — a Danmo envelope around Univer `ISlideData`.

## Formats

| Format | Role |
|--------|------|
| `*.uslides.json` | **Source of truth** — edit + AI 批改 |
| `.pptx` | Import only (Stage: view → convert to `.uslides.json`) |

Do **not** create `*-slides.md`, Marp frontmatter (`type: slides`), or hand-authored `*-slides.html` as the deliverable.

## File shape

```json
{
  "danmo": { "format": "univer-slides", "version": 1 },
  "snapshot": {
    "id": "slide_…",
    "title": "Deck title",
    "pageSize": { "width": 960, "height": 540 },
    "body": {
      "pageOrder": ["p0"],
      "pages": {
        "p0": {
          "id": "p0",
          "pageType": 0,
          "zIndex": 0,
          "title": "Title",
          "description": "",
          "pageBackgroundFill": { "rgb": "FFFFFF" },
          "pageElements": {
            "p0_title": {
              "id": "p0_title",
              "zIndex": 1,
              "left": 60,
              "top": 80,
              "width": 840,
              "height": 80,
              "title": "title",
              "description": "",
              "type": 0,
              "richText": { "text": "Title", "left": 60, "top": 80, "width": 840, "height": 80 }
            },
            "p0_body": {
              "id": "p0_body",
              "zIndex": 2,
              "left": 60,
              "top": 200,
              "width": 840,
              "height": 360,
              "title": "body",
              "description": "",
              "type": 0,
              "richText": { "text": "- point a\n- point b", "left": 60, "top": 200, "width": 840, "height": 360 }
            }
          }
        }
      }
    }
  }
}
```

## Workflow

1. Outline titles (≥2 slides).
2. Write `.uslides.json` with `write` (or patch with `edit` / `apply_patch`).
3. Deliver the path — Files → open in Slides Stage.

## [office-edit] turns

When the user message starts with `[office-edit]` and `kind: slides`:

1. `read_file` the given `.uslides.json` path.
2. Update only that path (prefer editing `richText.text` on the target page).
3. Stop with SUMMARY of pages changed.

## Anti-patterns

- Marp / `type: slides` Markdown as SoT
- Full HTML decks as authoring format
- Defaulting to `.pptx` binary as the editable artifact

## Stop condition

Deliver the `.uslides.json` path. Stop.

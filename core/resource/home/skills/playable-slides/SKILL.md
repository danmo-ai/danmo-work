---
name: playable-slides
source: builtin
description: "Author/edit slide decks as Univer IR `.uslides.json` using write/edit/apply_patch. Never Marp Markdown, never web-search Univer docs — IR shape lives in this skill's references/ and kb-office-ir."
license: MIT
compatibility: Requires write, edit, apply_patch, read_file; optional read_skill
metadata:
  author: danmo-work
  version: "3.0"
  category: work
---

# Playable Slides (Univer IR)

## Hard rules

1. **SoT is always `.uslides.json`** (Danmo envelope + Univer `ISlideData` snapshot).
2. **Change IR only with tools**: `read_file` → `edit` / `apply_patch` / `write`. No other path.
3. **Do not** `web_search` / `web_fetch` Univer documentation to "learn" the schema. Load `references/ir-slides.md` via `read_skill` (or `search_kb` theme「幻灯片 IR」 if KB bound).
4. **Do not** create `*-slides.md`, Marp (`type: slides`), or HTML decks as the editable artifact.
5. **Do not** confuse with `.md` reports (`document-writing`) or `.csv` / `.usheet.json` (`sheet-writing`).

## Format pick (slides only)

| User intent | Write |
|-------------|--------|
| Talk / pitch / training deck | `.uslides.json` |
| Import from PowerPoint | Stage converts `.pptx` → sibling `.uslides.json`; then edit the IR |
| Long-form report | **Wrong skill** → `.md` via `document-writing` |

## Tool workflow (create)

1. Outline ≥2 slide titles (optional `todowrite`).
2. `read_skill` `references/ir-slides.md` if you need the exact JSON skeleton.
3. `write` a valid `.uslides.json` (pretty-printed JSON, trailing newline OK).
4. Deliver the path. Stop.

## Tool workflow ([office-edit])

When the user message starts with `[office-edit]` and `kind: slides`:

1. `read_file` the listed `path` (must be `.uslides.json`).
2. Locate the target page (`page:` / `scope: slide` / selection). Prefer patching `richText.text` (and nearby layout fields) on that page only.
3. `edit` or `apply_patch` that file. Use `write` only for a full-file rewrite.
4. Do **not** invent a parallel `.md` deck. Do **not** browse the web.
5. SUMMARY: which page ids / `richText` fields changed.

## Minimal valid file

See `references/ir-slides.md` for a complete copy-paste template. Critical fields:

- Envelope: `danmo.format = "univer-slides"`, `danmo.version = 1`, `snapshot` = `ISlideData`
- Text boxes: `pageElements.*.type = 2` (`TEXT`), not `0` (`SHAPE`)
- `pageBackgroundFill.rgb` like `"rgb(255,255,255)"`

## Anti-patterns

- Web research on Univer instead of editing the project file
- Writing Markdown slides “for now”
- Emitting `.pptx` as the editable deliverable
- Leaving invalid JSON / wrong `danmo.format`

## Stop condition

Deliver the `.uslides.json` path (and SUMMARY for office-edit). Stop.

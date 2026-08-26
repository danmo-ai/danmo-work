# Univer slides IR (`.uslides.json`)

Danmo envelope around Univer `ISlideData`. Edit with `write` / `edit` / `apply_patch` only — do not fetch Univer docs from the web.

## Envelope

```json
{
  "danmo": { "format": "univer-slides", "version": 1 },
  "snapshot": { }
}
```

`snapshot` is `ISlideData`: `id`, `title`, `pageSize`, `body.pageOrder`, `body.pages`.

## Enums (use numbers)

| Field | Value | Meaning |
|-------|-------|---------|
| `pageType` | `0` | SLIDE |
| `type` (page element) | `2` | TEXT (title/body boxes) |
| `type` | `0` | SHAPE (rect/mask — not for plain titles) |
| `type` | `1` | IMAGE |

## Copy-paste skeleton (2 slides)

```json
{
  "danmo": { "format": "univer-slides", "version": 1 },
  "snapshot": {
    "id": "slide_demo",
    "title": "Demo deck",
    "pageSize": { "width": 960, "height": 540 },
    "body": {
      "pageOrder": ["p0", "p1"],
      "pages": {
        "p0": {
          "id": "p0",
          "pageType": 0,
          "zIndex": 0,
          "title": "Cover",
          "description": "",
          "pageBackgroundFill": { "rgb": "rgb(255,255,255)" },
          "pageElements": {
            "p0_title": {
              "id": "p0_title",
              "zIndex": 1,
              "left": 60,
              "top": 180,
              "width": 840,
              "height": 80,
              "title": "title",
              "description": "",
              "type": 2,
              "richText": {
                "text": "Demo deck",
                "fs": 40,
                "cl": { "rgb": "rgb(30,30,30)" },
                "bl": 1
              }
            },
            "p0_body": {
              "id": "p0_body",
              "zIndex": 2,
              "left": 60,
              "top": 280,
              "width": 840,
              "height": 120,
              "title": "body",
              "description": "",
              "type": 2,
              "richText": {
                "text": "One-line subtitle",
                "fs": 18,
                "cl": { "rgb": "rgb(80,80,80)" }
              }
            }
          }
        },
        "p1": {
          "id": "p1",
          "pageType": 0,
          "zIndex": 1,
          "title": "Agenda",
          "description": "",
          "pageBackgroundFill": { "rgb": "rgb(255,255,255)" },
          "pageElements": {
            "p1_title": {
              "id": "p1_title",
              "zIndex": 1,
              "left": 60,
              "top": 60,
              "width": 840,
              "height": 60,
              "title": "title",
              "description": "",
              "type": 2,
              "richText": {
                "text": "Agenda",
                "fs": 32,
                "cl": { "rgb": "rgb(30,30,30)" },
                "bl": 1
              }
            },
            "p1_body": {
              "id": "p1_body",
              "zIndex": 2,
              "left": 60,
              "top": 140,
              "width": 840,
              "height": 340,
              "title": "body",
              "description": "",
              "type": 2,
              "richText": {
                "text": "- Point A\n- Point B\n- Point C",
                "fs": 20,
                "cl": { "rgb": "rgb(50,50,50)" }
              }
            }
          }
        }
      }
    }
  }
}
```

## How to edit with tools

1. `read_file` the `.uslides.json`.
2. Find the page in `snapshot.body.pages[pageId]` (order from `pageOrder`).
3. Change copy under `pageElements.*.richText.text` (and `fs` / colors if needed).
4. `apply_patch` or `edit` — keep valid JSON.
5. Adding a page: append id to `pageOrder` and add a matching object under `pages`.

Canvas is typically **960×540**. Keep text boxes inside that rect.

---
name: danmo-make
source: builtin
description: >-
  Create and edit images/videos/audio via the Danmo Make MCP connector.
  Use when the user or parent agent needs local generation (Danmo Make / danqing).
license: MIT
compatibility: Requires built-in danmo-make connector enabled; mcp_danmo_make_* tools
metadata:
  author: danqing
  version: "0.3.0"
  category: creative
---

# Danmo Make

Call tools with full names from the tool list (`mcp_danmo_make_<tool>`). Short
names below are the MCP action suffix only. Tool contracts match Make MCP
`tools/list` (17 tools).

## Tools (suffix)

| Tool | Role |
|------|------|
| `health` | Probe Make runtime; abort if not ok |
| `list_models` | Filter by `media` / `action` / `installed` |
| `get_model` | Card: `actions`, `media_roles`, `type`, defaults, parameters |
| `upload_asset` | Local path → `ast_*` |
| `get_asset` | Metadata + local `path` |
| `get_task` / `wait_task` / `cancel_task` / `diagnose_task` | Task lifecycle |
| `generate_image` / `edit_image` / `upscale_image` | Image |
| `generate_video` / `edit_video` / `upscale_video` | Video |
| `generate_audio` / `edit_audio` | Audio / music |

## Workflow

1. **`health`** — abort if not ok.
2. **`list_models`** — always `installed=true` + `media` + **`action`** for the
   tool you will call:

   | Intent | `action` | Next tool | Card must allow |
   |--------|----------|-----------|-----------------|
   | Text → image/video | `generate` | `generate_image` / `generate_video` | `generate` |
   | Edit image/video/audio | `edit` | `edit_*` | `edit` |
   | Upscale image/video | `upscale` | `upscale_*` | `upscale` |
   | Text → audio/music | `generate` | `generate_audio` | `create_music` |

   Aliases OK: `create`→`generate` (audio→`create_music`); `rewrite`/`retouch`/
   `extend`/`animate`/`cover`/`repaint`→`edit`.

   Lists are pre-sorted. Treat `[0]` as the **suggestion**, not a choice.
   Only pick models from this filtered list (no LoRA / edit-only for generate).
3. **`get_model(model_id)`** on the suggested (or user-named) id — confirm
   `actions` + `type` (reject `lora` for `generate_*`); read
   `parameters.size.options` / `duration_sec.options` / `duration`, and
   **`media_roles`** before attaching references.
4. **`ask_user`** — required before generate/edit/upscale unless the goal
   **literally** already contains both a model id **and** the core size /
   duration value. A creative brief (subject, style, clothing, background) does
   **not** count.

   | Media | Ask |
   |-------|-----|
   | image | model (suggest `[0]`) + size (`WIDTHxHEIGHT` from options) |
   | video | model + size + `duration_sec` (from options, not frame count) |
   | audio | model + duration (seconds) |

   Short `options` + `defaultOption`. If the user picks a different model than
   the one you `get_model`'d, call `get_model` again before generate.
   If `ask_user` errors, **stop and report** — do not invent params and continue.
5. References / edits: `upload_asset` → `ast_*`. Prefer ordered **`media`**
   `[{role, asset_id}, …]` when the card lists roles:

   | Tool | Typical roles |
   |------|----------------|
   | `edit_image` | `reference_image`, `mask` (+ `source_asset_id`; `operation` = rewrite/retouch/extend) |
   | `generate_video` | `reference_image` / `reference_video` / `reference_audio` (empty = text-to-video) |
   | `edit_video` | `first_frame`, `last_frame`, `source_video`, refs (`source_asset_id` = first frame or source video; first/last cannot mix with reference media) |

6. Call the matching generate/edit/upscale tool with **confirmed** model +
   params (`wait=true`; image default timeout 300s, video ≥ 900, audio 600).
   `generate_image`: leave `media` empty (refs → `edit_image`).
7. From the result: `primary_asset_id` / `asset_ids` → `get_asset` for local
   `path`. On failure: `diagnose_task` and report the real error.

## Anti-patterns

- `list_models` without `installed=true` or without `action`
- Skipping `get_model`, or using a model whose `actions` / `media_roles` do not match
- Choosing a model not in the filtered list, or a LoRA id as `model`
- Skipping `ask_user`, hardcoding `1024x1024` or guessing ids
- Continuing after a failed `ask_user`
- Calling Make REST via `http_request`
- Claiming success without a completed task + asset ids

---
name: danmo-make
description: >-
  Create and edit images/videos/audio via the Danmo Make MCP connector.
  Use when the user or parent agent needs local generation (Danmo Make / danqing).
license: MIT
compatibility: Requires built-in danmo-make connector enabled; mcp_danmo_make_* tools
metadata:
  author: danqing
  version: "0.2.0"
  category: creative
---

# Danmo Make

Call tools with full names from the tool list (`mcp_danmo_make_<tool>`). Short
names below are the MCP action suffix only.

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
3. **`get_model(id)`** on the suggested (or user-named) id — confirm `actions`;
   read `parameters.size.options` / `duration_sec.options` / `duration`.
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
5. Edits with a reference file: `upload_asset` → `ast_*`, then edit with
   `source_asset_id`.
6. Call the matching generate/edit/upscale tool with **confirmed** model +
   params (`wait=true`; video `wait_timeout_seconds` ≥ 900 when needed).
7. From the result: `primary_asset_id` / `asset_ids` → `get_asset` for local
   `path`. On failure: `diagnose_task` and report the real error.

## Anti-patterns

- `list_models` without `installed=true` or without `action`
- Choosing a model not in the filtered list, or a LoRA id as `model`
- Skipping `get_model` / `ask_user`, hardcoding `1024x1024` or guessing ids
- Continuing after a failed `ask_user`
- Calling Make REST via `http_request`
- Claiming success without a completed task + asset ids

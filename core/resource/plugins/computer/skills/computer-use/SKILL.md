---
name: computer-use
source: builtin
description: >-
  Operate desktop applications through the computer tool: find and focus
  windows, take screenshots to see the screen, then click, type, and press
  keys. Use for GUI automation of real apps a human would drive by hand.
license: Apache-2.0
compatibility: >-
  Requires the `computer` tool bound and runtime.computer.enabled=true.
  With runtime.computer.target=auto (default), sandbox.profile=desktop targets
  the Xvfb sandbox; host and sandbox can also be selected explicitly. Linux needs
  an X11 DISPLAY + xdotool + a capture tool; macOS uses the bundled CoreGraphics
  helper and needs Accessibility + Screen Recording permission; Windows uses PowerShell.
  Screenshots require a vision model.
metadata:
  author: danmo
  version: "0.1.0"
  category: automation
---

# Computer Use (desktop GUI control)

Drive real application windows through the single `computer` tool. The first
call asks the user for approval; the user may allow desktop control for the rest
of the session. With `runtime.sandbox.profile=desktop` this drives
the sandboxed Xvfb desktop (same machine as sandbox Chromium / noVNC) — not the
host GUI. Otherwise it controls the host. Act deliberately and verify each step.

For web pages prefer the `browser` expert (`browser_*`). CDP is always
`http://127.0.0.1:9222` (sandbox or host).

## Core loop

Always follow: **see → act → verify**.

1. `list_windows` (optionally with `query`) to find the target window and its id.
2. `focus_window` with that `window_id` to bring it forward.
3. `screenshot` (pass the same `window_id` to capture just that window) to SEE
   the current state before acting.
4. Act: `left_click` / `type` / `key` / `scroll` / `left_click_drag`.
5. `screenshot` again to confirm the result before the next action.

Do not click blind. If you have not taken a screenshot since the screen last
changed, take one first.

## Coordinates

- A full-display screenshot (`screenshot` with no `window_id`) returns
  **absolute screen** pixels. Use absolute coordinates for later actions.
- A window screenshot (`screenshot` with a `window_id`) returns pixels
  **relative to that image**. Pass the same `window_id` on the follow-up click/
  scroll and the tool maps the coordinate back to the screen automatically.
- Read a target's location from the screenshot, not from memory. Re-screenshot
  after anything that can move or resize the window.

## Actions reference

- `list_windows` — `query` filters by title/app substring.
- `focus_window` — needs `window_id`.
- `screenshot` — optional `window_id`.
- `mouse_move` — needs `coordinate: [x, y]`.
- `left_click` / `right_click` / `middle_click` / `double_click` / `triple_click` — optional
  `coordinate` (defaults to current cursor); optional `modifiers`.
- `zoom` — needs a screen-coordinate `zoom` rectangle; returns native-resolution detail.
  Follow-up coordinates belong to that zoomed image until the next normal screenshot.
- `get_screen_size` — reports the current region in native input units.
- `left_mouse_down` / `left_mouse_up` — hold/release at the current cursor.
- `left_click_drag` — needs `start_coordinate` and `coordinate`.
- `scroll` — needs `scroll_direction` (up/down/left/right); optional
  `coordinate` and `scroll_amount` (default 3).
- `type` — needs `text` (literal characters).
- `key` — needs `text`: a key or chord such as `Return`, `ctrl+s`, `alt+Tab`.
- `cursor_position` — returns the current cursor location.
- `wait` — `duration` seconds (1-10) to let the UI settle.

## Good practice

- Prefer keyboard shortcuts (`key`) over fragile mouse clicks when an app
  supports them (e.g. `ctrl+s`, `ctrl+l`, `Return`).
- After launching or switching apps, `wait` 1-2s and re-`screenshot`; UIs are not
  instant.
- If an action does not change the screen as expected, do not repeat it blindly.
  Re-screenshot, re-read coordinates, and try a different approach.
- Never use `exec_shell` to script the GUI when `computer` is available.

## Platform notes

- **Linux/X11**: works when `DISPLAY` is set and `xdotool` plus a capture tool
  are installed. Wayland-only sessions are not supported (reported as degraded).
- **macOS**: the host app must have Accessibility and Screen Recording
  permission (System Settings → Privacy & Security). The bundled CoreGraphics
  helper handles mouse, scroll, and Unicode keyboard input.
- **Windows**: uses PowerShell + Win32; no extra install.

## Limitations

- Screenshots are returned to the model as images, so the routed model must
  support vision. Text-only models are rejected before capture.
- If `computer` reports desktop control is disabled or unavailable, stop and
  report the blocker (enable `runtime.computer.enabled`, install the missing
  tool, or grant OS permission) rather than retrying.

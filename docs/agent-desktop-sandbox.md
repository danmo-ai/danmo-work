# Agent desktop sandbox — design notes

## Goal

Give agents an isolated **minimal Linux desktop** for software testing: browser use
(CDP / a11y) and computer use (screenshot + mouse/keyboard) inside a sandbox,
with optional noVNC observation — without driving the host GUI.

## Selection

| Approach | Decision |
|----------|----------|
| OCI desktop image on existing `runtime.sandbox` container backends | **Adopt** |
| Anthropic computer-use-demo / E2B Desktop stack (Xvfb + WM + Chromium + xdotool + VNC) | **Copy recipe** |
| Firecracker / E2B cloud / OpenSandbox control plane | **Reject** — conflicts with local tar-load, never-pull model |
| MCP-only nested Xephyr (glovebox, workspace-linux) | **Reject** as Hard isolation backend |
| Replace `browser_*` with Stagehand/Steel | **Defer** — keep chromedp; attach to sandbox CDP |

## Profiles

| Profile | Image tag | Asset | Purpose |
|---------|-----------|-------|---------|
| `cli` (default) | `localhost/danmo-work-env:bundled` | `danmo-work-env-linux-<arch>.tar.gz` | Alpine shell isolation |
| `desktop` | `localhost/danmo-work-env:desktop` | `danmo-work-env-desktop-linux-<arch>.tar.gz` | GUI + browser + computer |

Config: `runtime.sandbox.profile` (`cli` \| `desktop`). Container backends only.

## Architecture

```
exec_shell / browser_* / computer
        │
        ▼
 runtime.sandbox (podman|docker|apple-container)
        │
        ├─ profile=cli     → Alpine sleep-infinity container
        └─ profile=desktop → Ubuntu Xvfb + openbox + Chromium(CDP) + x11vnc/noVNC
                                │
                                ├─ computer → container exec DISPLAY=:1 + xdotool/scrot
                                └─ browser  → attach published CDP (127.0.0.1:<host>)
```

## Non-goals (this phase)

- Hosted sandbox platforms (E2B, Daytona, OpenSandbox)
- Firecracker / KVM microVMs
- Treating host `computer` as the software-test environment
- Shipping LibreOffice / heavy demo apps in the desktop image
- Replacing semantic `browser_*` tools

## Testing

- Unit: profile asset naming, CreateOpts port/cmd flags, status JSON fields
- Smoke (when docker available): start desktop container → CDP probe → screenshot → navigate localhost

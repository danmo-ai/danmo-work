# Agent desktop environment image (optional download)

Minimal Ubuntu desktop OCI image for `runtime.sandbox.profile=desktop` with a
container backend (`podman` / `docker` / `apple-container`).

**Not packaged inside app/server releases.** CI publishes gzipped GitHub Release
assets:

- `danmo-work-env-desktop-linux-amd64.tar.gz`
- `danmo-work-env-desktop-linux-arm64.tar.gz`

Download into `~/.danmo-work/env/` (Settings buttons or `WORK_ENV_TAR`).

## Contents

| Layer | Packages |
|-------|----------|
| Base | Ubuntu 24.04, bash, git, curl, jq, ca-certificates |
| Display | Xvfb `:1`, openbox, xterm |
| Computer use | xdotool, scrot, ImageMagick |
| Browser | Chromium with `--remote-debugging-port=9222` |
| Observe | x11vnc + noVNC/websockify on `:6080` |

## Build

```bash
DQ_ENV_PROFILE=desktop make build-env-tar
# → out/env/danmo-work-env-desktop-linux-<arch>.tar.gz
```

- **Tag:** `localhost/danmo-work-env:desktop`
- **Runtime:** load tar only (never registry pull); per-project detached container
- **Workspace:** bind-mount project at the same absolute host path

## Ports (published to host loopback)

| Host ↔ Container | Purpose |
|------------------|---------|
| `127.0.0.1:9222` → `:9222` | Chromium CDP (`browser_*` / Settings) |
| `127.0.0.1:6080` → `:6080` | noVNC web viewer |

Same CDP port as a host Chrome with remote debugging (`http://127.0.0.1:9222`).

## Related

See [docs/agent-desktop-sandbox.md](../../docs/agent-desktop-sandbox.md).

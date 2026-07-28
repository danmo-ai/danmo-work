# Agent environment image (optional download)

Ultra-light OCI image for `runtime.environment.backend=container`.

**Not packaged inside app/server releases.** CI publishes **two** GitHub Release
assets (matrix build):

- `danmo-work-env-linux-amd64.tar`
- `danmo-work-env-linux-arm64.tar`

Download into `~/.danmo-work/env/` (Settings buttons or `WORK_ENV_TAR`).

- **Base:** `debian:bookworm-slim` + apt (`bash`, `ca-certificates`, `curl`, `git`, `jq`, `xz-utils`)
- **Not baked in:** Node, Python, Go — install via `apt-get` when needed
- **Build / save (no push):** `make build-env-tar` → `out/env/danmo-work-env-linux-<arch>.tar`
- **Runtime:** `podman|docker|container image load` then per-project `exec` (never `pull`)
- **Tag:** `localhost/danmo-work-env:bundled`
- **Workspace:** bind-mount project directory at `/workspace`

Expected packed size: roughly **80–120 MB**.

# Bundled agent environment

Ultra-light OCI image for `runtime.environment.backend=container`.

- **Base:** `debian:bookworm-slim` + apt (`bash`, `ca-certificates`, `curl`, `git`, `jq`, `xz-utils`)
- **Not baked in:** Node, Python, Go — install via `apt-get` when needed
- **Build / save (no push):** `make build-env-tar` → `out/env/danmo-work-env-linux-<arch>.tar`
- **Runtime:** `podman|docker|container image load` then per-project `exec` (never `pull`)
- **Tag:** `localhost/danmo-work-env:bundled`
- **Workspace:** bind-mount project directory at `/workspace`

Expected packed size: roughly **80–120 MB** (vs ~350–450 MB with Node+Python preinstalled).

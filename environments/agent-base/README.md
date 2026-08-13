# Agent environment image (optional download)

Ultra-light OCI image for `runtime.environment.backend=container`.

**Not packaged inside app/server releases.** CI publishes **two** gzipped GitHub
Release assets (matrix build):

- `danmo-work-env-linux-amd64.tar.gz`
- `danmo-work-env-linux-arm64.tar.gz`

Download into `~/.danmo-work/env/` (Settings buttons or `WORK_ENV_TAR`).

- **Base:** `alpine` (busybox coreutils) + `bash`, `ca-certificates-bundle`, `git`, `curl`, `jq`
- **Package manager:** `apk` — install on demand: `apk add --no-cache python3 nodejs …`
- **Not baked in:** Node, Python, Go — install via `apk` when needed
- **Build / save (no push):** `make build-env-tar` → `out/env/danmo-work-env-linux-<arch>.tar.gz`
  (`docker save | gzip`; the runtime gunzips before engine load)
- **Runtime:** `podman|docker|container image load` then per-project `exec` (never `pull`)
- **Tag:** `localhost/danmo-work-env:bundled`
- **Workspace:** bind-mount project directory at the **same absolute path** as on the host
  (so `read_file` / `exec_shell` paths match). Override with `workspace_mount` if needed.

Expected asset size: roughly **8–10 MB** gzipped (~20 MB plain tar; unpacked
image ~20 MB on disk).

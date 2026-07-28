# Bundled agent environment

Ultra-light OCI image for `runtime.environment.backend=container`.

- **Build / save (no push):** `make build-env-tar` → `out/env/danmo-work-env-linux-<arch>.tar`
- **Runtime:** `podman|docker load -i <tar>` then per-project `exec` (never `pull`)
- **Tag:** `localhost/danmo-work-env:bundled`
- **Workspace:** bind-mount project directory at `/workspace`

Contents: Ubuntu 24.04, bash, git, curl, jq, python3, Node.js LTS (official tarball).
No project source is copied into the image.

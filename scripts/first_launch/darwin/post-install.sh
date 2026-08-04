#!/usr/bin/env bash
# macOS first-launch / post-install hooks (idempotent).
# Invoked asynchronously by the backend after startup.
# Env: DANMO_HOME (default ~/.danmo-work)
#
# CodeGraph CLI is installed via the market connector (assets), not here.
set -euo pipefail

DANMO_HOME="${DANMO_HOME:-${HOME}/.danmo-work}"
mkdir -p "$DANMO_HOME"

log() { echo "[first-launch:darwin] $*"; }

# Future darwin-only hooks go below (fonts, quarantine xattr, etc.).

log "done"

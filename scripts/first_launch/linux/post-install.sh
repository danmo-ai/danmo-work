#!/usr/bin/env bash
# Linux first-launch / post-install hooks (idempotent).
# Invoked asynchronously by the backend after startup.
# Env: DANMO_HOME (default ~/.danmo-work)
#
# CodeGraph CLI is installed via the market connector (assets), not here.
set -euo pipefail

DANMO_HOME="${DANMO_HOME:-${HOME}/.danmo-work}"
mkdir -p "$DANMO_HOME"

log() { echo "[first-launch:linux] $*"; }

# Future linux-only hooks go below (desktop entry, AppArmor notes, etc.).

log "done"

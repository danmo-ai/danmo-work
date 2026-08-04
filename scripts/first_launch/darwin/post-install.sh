#!/usr/bin/env bash
# macOS first-launch / post-install hooks (idempotent).
# Invoked asynchronously by the backend after startup.
# Env: DANMO_HOME (default ~/.danmo-work)
set -euo pipefail

DANMO_HOME="${DANMO_HOME:-${HOME}/.danmo-work}"
BIN_DIR="${DANMO_HOME}/bin"
mkdir -p "$BIN_DIR"

log() { echo "[first-launch:darwin] $*"; }

# --- CodeGraph: unpack compressed CLI if present ---
install_codegraph() {
  local archive="$BIN_DIR/codegraph.tar.gz"
  local dest="$BIN_DIR/codegraph"
  if [[ ! -f "$archive" ]]; then
    log "codegraph archive not found — skip"
    return 0
  fi
  if [[ -x "$dest" ]] && ! head -c 2 "$dest" 2>/dev/null | grep -q '#!'; then
    log "codegraph binary already present"
    return 0
  fi
  local tmp
  tmp="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "rm -rf '$tmp'" RETURN
  tar -xzf "$archive" -C "$tmp"
  local found
  found="$(find "$tmp" -type f -name codegraph | head -n 1 || true)"
  if [[ -z "$found" ]]; then
    log "ERROR: codegraph binary missing inside archive"
    return 1
  fi
  cp -f "$found" "$dest"
  chmod +x "$dest"
  log "extracted codegraph → $dest"
}

install_codegraph

# Future darwin-only hooks go below (fonts, quarantine xattr, etc.).

log "done"

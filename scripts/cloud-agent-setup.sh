#!/usr/bin/env bash
# Idempotent Cloud Agent bootstrap for Danmo Work.
#
# Prepares a fresh checkout so `make dev-web` (backend :7801 + Vite :5801) works:
#   - installs Go (version pinned in go.mod) when the toolchain is missing
#   - clones + builds the sibling dq-ui repo (the frontend consumes it via
#     `file:../../dq-ui/packages/*`, so it must live next to the repo root)
#   - installs frontend deps and warms the Go module cache
#   - seeds ~/.danmo-work/config.yaml from config.example.yaml
#
# Safe to run repeatedly and against cached/partial state.
set -euo pipefail

GO_VERSION="1.26.0"
DQ_UI_REPO="https://github.com/danmo-ai/dq-ui.git"

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# The frontend resolves dq-ui at ../../dq-ui relative to frontend/, i.e. a
# sibling of the repo root. Derive the same path here regardless of checkout dir.
DQ_UI_DIR="$(dirname "$REPO_DIR")/dq-ui"

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

log() { printf '==> %s\n' "$*"; }

# ── Go toolchain ────────────────────────────────────────────────────────────
install_go() {
  if command -v go >/dev/null 2>&1 && go version 2>/dev/null | grep -Eq 'go1\.(2[6-9]|[3-9][0-9])'; then
    log "Go already satisfies go.mod ($(go version | awk '{print $3}'))"
    return
  fi

  local goarch
  case "$(uname -m)" in
    x86_64|amd64) goarch=amd64 ;;
    aarch64|arm64) goarch=arm64 ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac

  local tmp
  tmp="$(mktemp -d)"
  log "Installing Go ${GO_VERSION} (linux/${goarch})"
  curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-${goarch}.tar.gz" -o "$tmp/go.tgz"
  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf "$tmp/go.tgz"
  $SUDO ln -sf /usr/local/go/bin/go /usr/local/bin/go
  $SUDO ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  rm -rf "$tmp"
}

# ── Sibling dq-ui workspace (built dist artifacts the frontend imports) ──────
setup_dq_ui() {
  if [ ! -d "$DQ_UI_DIR/.git" ]; then
    log "Cloning dq-ui into $DQ_UI_DIR"
    local parent
    parent="$(dirname "$DQ_UI_DIR")"
    if [ ! -w "$parent" ]; then
      $SUDO mkdir -p "$DQ_UI_DIR"
      $SUDO chown -R "$(id -u):$(id -g)" "$DQ_UI_DIR"
    else
      mkdir -p "$DQ_UI_DIR"
    fi
    git clone --depth 1 "$DQ_UI_REPO" "$DQ_UI_DIR"
  else
    log "dq-ui already present; refreshing"
    git -C "$DQ_UI_DIR" pull --ff-only || true
  fi

  # npm (hoisted) install matches CI: lucide-vue-next/reka-ui resolve for the
  # shell package's Rollup build.
  ( cd "$DQ_UI_DIR" && npm install --no-audit --no-fund )
  # Build order matters: tokens -> ui -> shell.
  for pkg in tokens ui shell; do
    log "Building @danqing/dq-$pkg"
    ( cd "$DQ_UI_DIR/packages/$pkg" && npm run build )
  done
}

# ── Frontend + Go deps ──────────────────────────────────────────────────────
setup_repo_deps() {
  export PATH="/usr/local/go/bin:$PATH"
  log "Installing frontend dependencies"
  ( cd "$REPO_DIR/frontend" && npm install --no-audit --no-fund )
  log "Warming Go module cache"
  ( cd "$REPO_DIR" && go mod download )
}

# ── User config (~/.danmo-work/config.yaml) ─────────────────────────────────
seed_config() {
  local home_dir="${WORK_HOME:-$HOME/.danmo-work}"
  mkdir -p "$home_dir"
  if [ ! -f "$home_dir/config.yaml" ]; then
    log "Seeding $home_dir/config.yaml from config.example.yaml"
    cp "$REPO_DIR/config.example.yaml" "$home_dir/config.yaml"
  else
    log "Config already present at $home_dir/config.yaml"
  fi
}

install_go
setup_dq_ui
setup_repo_deps
seed_config

log "Setup complete. Start the dev stack with: make dev-web"

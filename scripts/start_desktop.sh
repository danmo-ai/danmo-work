#!/usr/bin/env bash
# Dev: Go API + Tauri Desktop (Vite HMR via beforeDevCommand)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"
# shellcheck source=dev_process.sh
source "$SCRIPT_DIR/dev_process.sh"

APP_NAME="${DQ_APP_NAME:-danmo-work}"
BACKEND_PORT="${DQ_BACKEND_PORT}"

dq_ensure_out_layout
"$SCRIPT_DIR/stop.sh" 2>/dev/null || true

echo "==> Starting $APP_NAME (dev-desktop) [${DQ_PROJECT}]"
echo "    Backend : http://127.0.0.1:${BACKEND_PORT}"
echo "    Desktop : Tauri webview (Vite HMR on :${DQ_FRONTEND_PORT})"

cd "$DQ_ROOT/frontend"
if [[ ! -d node_modules ]] || [[ package-lock.json -nt node_modules ]]; then
  npm install
fi

# Tauri externalBin requires a target-tripled sidecar even in dev.
OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS-$ARCH" in
  Darwin-arm64) TRIPLE="aarch64-apple-darwin" ;;
  Darwin-x86_64) TRIPLE="x86_64-apple-darwin" ;;
  Linux-x86_64) TRIPLE="x86_64-unknown-linux-gnu" ;;
  MINGW*-x86_64|MSYS*-x86_64|CYGWIN*-x86_64) TRIPLE="x86_64-pc-windows-msvc" ;;
  *)
    echo "Unsupported platform: $OS-$ARCH" >&2
    exit 1
    ;;
esac
SIDECAR_NAME="danmo-work-backend-$TRIPLE"
if [[ "$TRIPLE" == *"-pc-windows-msvc" ]]; then
  SIDECAR_NAME="$SIDECAR_NAME.exe"
fi
SIDECAR_PATH="$DQ_ROOT/desktop/src-tauri/bin/$SIDECAR_NAME"
# Always rebuild the Go binary so go:embed (agents/skills) stays current.
# A stale sidecar was why new experts (e.g. danmo-make) showed as "custom".
DEV_BACKEND_BIN="$DQ_RUN_DIR/backend-bin"
echo "==> Building dev backend -> $DEV_BACKEND_BIN"
(cd "$DQ_ROOT" && go build -o "$DEV_BACKEND_BIN" ./server)
mkdir -p "$(dirname "$SIDECAR_PATH")"
cp -f "$DEV_BACKEND_BIN" "$SIDECAR_PATH"
# Keep ~/.danmo-work/bin in sync when present (some launch paths use it).
if [[ -d "$HOME/.danmo-work/bin" ]]; then
  cp -f "$DEV_BACKEND_BIN" "$HOME/.danmo-work/bin/danmo-work-backend"
fi
# Stage platform first-launch script (post-install hooks; CodeGraph CLI comes from market).
case "$(uname -s)" in
  Darwin)
    "$SCRIPT_DIR/stage_first_launch.sh" darwin "$HOME/.danmo-work/first_launch" || true
    "$SCRIPT_DIR/stage_first_launch.sh" darwin "$DQ_ROOT/desktop/src-tauri/resources/first_launch" || true
    ;;
  Linux)
    "$SCRIPT_DIR/stage_first_launch.sh" linux "$HOME/.danmo-work/first_launch" || true
    "$SCRIPT_DIR/stage_first_launch.sh" linux "$DQ_ROOT/desktop/src-tauri/resources/first_launch" || true
    ;;
esac
# Fetch/stage bundled ripgrep (grep tool engine); cached and non-fatal so dev
# works offline (grep falls back to the pure-Go walker).
"$SCRIPT_DIR/fetch_ripgrep.sh" "$DQ_ROOT/desktop/src-tauri/resources/rg" || true
echo "==> Sidecar binary: $SIDECAR_PATH"
echo ""

# Tauri always used to reclaim :7801 + spawn ~/.danmo-work/bin sidecar. That
# fights the script-started backend below (one `make dev-desktop`, two owners).
# Tell the Rust shell to leave :7801 alone whenever we already have / expect API.
export WORK_EXTERNAL_BACKEND=1

if [[ "${SKIP_BACKEND:-0}" == "1" ]]; then
  echo "==> SKIP_BACKEND=1: using external backend (e.g. GoLand on port ${BACKEND_PORT})"
  echo "    WORK_EXTERNAL_BACKEND=1 (Tauri will not reclaim/spawn sidecar)"
  echo ""
else
  export DQ_DEV_ENV=$'WORK_AUTO_APPROVE='"${WORK_AUTO_APPROVE:-false}"$'\nWORK_ADDR=0.0.0.0:'"${BACKEND_PORT}"
  dq_dev_start backend "$DQ_ROOT" "$DEV_BACKEND_BIN"
  unset DQ_DEV_ENV

  echo "==> Backend PID: $(cat "$DQ_RUN_DIR/backend.pid")"
  echo "    Logs: $DQ_RUN_DIR/backend.log"
  echo "    WORK_EXTERNAL_BACKEND=1 (Tauri will not reclaim/spawn sidecar)"
  echo ""
fi

echo "==> Starting Tauri dev..."
echo "    Press Ctrl+C to stop"
echo ""

# Cleanup on exit
cleanup() {
  echo ""
  echo "==> Stopping dev processes..."
  "$SCRIPT_DIR/stop.sh" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Start Tauri dev in foreground
cd "$DQ_ROOT/desktop"
if [[ ! -d node_modules ]]; then
  npm install
fi
npm run tauri dev

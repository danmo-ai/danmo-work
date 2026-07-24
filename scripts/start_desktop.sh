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
if [[ ! -d node_modules ]]; then
  npm install
fi

if [[ "${SKIP_BACKEND:-0}" == "1" ]]; then
  echo "==> SKIP_BACKEND=1: using external backend (e.g. GoLand on port ${BACKEND_PORT})"
  echo ""

  # Verify sidecar binary exists for Tauri build
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
  if [[ ! -f "$SIDECAR_PATH" ]]; then
    echo "ERROR: sidecar binary not found: $SIDECAR_PATH" >&2
    echo "Run: make build-sidecar" >&2
    exit 1
  fi
  echo "==> Sidecar binary: $SIDECAR_PATH"
  echo ""
else
  DEV_BACKEND_BIN="$DQ_RUN_DIR/backend-bin"
  echo "==> Building dev backend -> $DEV_BACKEND_BIN"
  (cd "$DQ_ROOT" && go build -o "$DEV_BACKEND_BIN" ./server)

  export DQ_DEV_ENV=$'WORK_AUTO_APPROVE='"${WORK_AUTO_APPROVE:-false}"$'\nWORK_ADDR=0.0.0.0:'"${BACKEND_PORT}"
  dq_dev_start backend "$DQ_ROOT" "$DEV_BACKEND_BIN"
  unset DQ_DEV_ENV

  echo "==> Backend PID: $(cat "$DQ_RUN_DIR/backend.pid")"
  echo "    Logs: $DQ_RUN_DIR/backend.log"
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

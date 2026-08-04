#!/usr/bin/env bash
# Dev: Go API only (for Go debugger or separate frontend)
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

# Stage first-launch hooks for this host (CodeGraph extract, …).
case "$(uname -s)" in
  Darwin) "$SCRIPT_DIR/stage_first_launch.sh" darwin "$HOME/.danmo-work/first_launch" || true ;;
  Linux) "$SCRIPT_DIR/stage_first_launch.sh" linux "$HOME/.danmo-work/first_launch" || true ;;
esac

echo "==> Starting $APP_NAME (backend only) [${DQ_PROJECT}]"
echo "    Backend : http://127.0.0.1:${BACKEND_PORT}"

DEV_BACKEND_BIN="$DQ_RUN_DIR/backend-bin"
DEV_VERSION=$(git -C "$DQ_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)
echo "==> Building dev backend ($DEV_VERSION) -> $DEV_BACKEND_BIN"
(cd "$DQ_ROOT" && go build -ldflags "-X 'danmo-work/server/api/v1.Version=$DEV_VERSION'" -o "$DEV_BACKEND_BIN" ./server)

export DQ_DEV_ENV=$'WORK_AUTO_APPROVE='"${WORK_AUTO_APPROVE:-false}"$'\nWORK_ADDR=0.0.0.0:'"${BACKEND_PORT}"
dq_dev_start backend "$DQ_ROOT" "$DEV_BACKEND_BIN"
unset DQ_DEV_ENV

echo "==> Backend PID: $(cat "$DQ_RUN_DIR/backend.pid")"
echo "    Logs: $DQ_RUN_DIR/backend.log"
echo "    Stop: make stop"

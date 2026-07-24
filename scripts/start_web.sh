#!/usr/bin/env bash
# Dev: Go API + Vite HMR (frontend proxies /api)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"
# shellcheck source=dev_process.sh
source "$SCRIPT_DIR/dev_process.sh"

APP_NAME="${DQ_APP_NAME:-danmo-work}"
BACKEND_PORT="${DQ_BACKEND_PORT}"
FRONTEND_PORT="${DQ_FRONTEND_PORT}"

dq_ensure_out_layout
"$SCRIPT_DIR/stop.sh" 2>/dev/null || true

echo "==> Starting $APP_NAME (dev) [${DQ_PROJECT}]"
echo "    Backend : http://127.0.0.1:${BACKEND_PORT}"
echo "    Frontend: http://localhost:${FRONTEND_PORT}/app/  (Vite HMR)"

cd "$DQ_ROOT/frontend"
if [[ ! -d node_modules ]]; then
  npm install
fi

DEV_BACKEND_BIN="$DQ_RUN_DIR/backend-bin"
echo "==> Building dev backend -> $DEV_BACKEND_BIN"
(cd "$DQ_ROOT" && go build -o "$DEV_BACKEND_BIN" ./server)

export DQ_DEV_ENV=$'WORK_AUTO_APPROVE='"${WORK_AUTO_APPROVE:-false}"$'\nWORK_ADDR=0.0.0.0:'"${BACKEND_PORT}"
dq_dev_start backend "$DQ_ROOT" "$DEV_BACKEND_BIN"
unset DQ_DEV_ENV

export DQ_DEV_ENV=$'DQ_BACKEND_PORT='"${BACKEND_PORT}"$'\nDQ_FRONTEND_PORT='"${FRONTEND_PORT}"
dq_dev_start frontend "$DQ_ROOT/frontend" npm run dev
unset DQ_DEV_ENV

echo "==> PIDs: backend=$(cat "$DQ_RUN_DIR/backend.pid") frontend=$(cat "$DQ_RUN_DIR/frontend.pid")"
echo "    Marker: $(cat "$DQ_RUN_DIR/project.marker")"
echo "    Logs: $DQ_RUN_DIR/backend.log $DQ_RUN_DIR/frontend.log"
echo "    Stop: make stop"

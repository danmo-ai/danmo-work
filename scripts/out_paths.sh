# Unified build output paths — source from scripts/*.sh
# Layout: out/frontend/dist, out/server/, out/desktop/, out/dist/, out/run/

if [[ -n "${_DQ_OUT_PATHS_LOADED:-}" ]]; then
  return 0 2>/dev/null || true
fi
_DQ_OUT_PATHS_LOADED=1

_DQ_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export DQ_ROOT="$(cd "$_DQ_SCRIPT_DIR/.." && pwd)"
export DQ_OUT="$DQ_ROOT/out"
export DQ_FRONTEND_DIST="$DQ_OUT/frontend/dist"
export DQ_SERVER_DIR="$DQ_OUT/server"
export DQ_DESKTOP_BUNDLE="$DQ_OUT/desktop/bundle"
export DQ_DESKTOP_CARGO="$DQ_OUT/desktop/cargo"
export DQ_RELEASE_DIST="$DQ_OUT/dist"
export DQ_RUN_DIR="$DQ_OUT/run"
export DQ_PROJECT="${DQ_PROJECT:-danmo-work}"
# Port convention: backend 78xx / frontend 58xx (same suffix per project)
export DQ_BACKEND_PORT="${DQ_BACKEND_PORT:-7801}"
export DQ_FRONTEND_PORT="${DQ_FRONTEND_PORT:-5801}"

dq_ensure_out_layout() {
  mkdir -p \
    "$DQ_FRONTEND_DIST" \
    "$DQ_SERVER_DIR" \
    "$DQ_DESKTOP_BUNDLE" \
    "$DQ_DESKTOP_CARGO" \
    "$DQ_RELEASE_DIST" \
    "$DQ_RUN_DIR"
}

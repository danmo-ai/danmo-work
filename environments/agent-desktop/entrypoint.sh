#!/usr/bin/env bash
# Start a minimal virtual desktop for agent computer/browser use.
set -euo pipefail

DISPLAY_NUM="${DISPLAY_NUM:-1}"
export DISPLAY=":${DISPLAY_NUM}"
WIDTH="${DISPLAY_WIDTH:-1280}"
HEIGHT="${DISPLAY_HEIGHT:-800}"
CDP_PORT="${CDP_PORT:-9222}"
NOVNC_PORT="${NOVNC_PORT:-6080}"
VNC_PORT="${VNC_PORT:-5900}"

mkdir -p /tmp/danmo-desktop /home/desktop/.config/openbox
export HOME=/home/desktop
# openbox needs a writable HOME; create if missing (root-run containers).
mkdir -p "$HOME"

cleanup() {
  jobs -p | xargs -r kill 2>/dev/null || true
}
trap cleanup EXIT

echo "==> Xvfb ${DISPLAY} ${WIDTH}x${HEIGHT}x24"
Xvfb "${DISPLAY}" -screen 0 "${WIDTH}x${HEIGHT}x24" -ac +extension GLX +render -noreset &
# Wait until the display is accepting connections.
for _ in $(seq 1 50); do
  if xdpyinfo -display "${DISPLAY}" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

echo "==> openbox"
openbox >/tmp/danmo-desktop/openbox.log 2>&1 &

echo "==> xterm (optional shell)"
xterm -geometry 100x30+20+20 >/tmp/danmo-desktop/xterm.log 2>&1 &

CHROME_BIN="${DANMO_CHROME_BIN:-}"
if [[ -z "$CHROME_BIN" ]]; then
  for c in danmo-chromium chromium chromium-browser google-chrome google-chrome-stable; do
    if command -v "$c" >/dev/null 2>&1; then
      CHROME_BIN="$(command -v "$c")"
      break
    fi
  done
fi

if [[ -n "${CHROME_BIN}" ]]; then
  echo "==> Chromium CDP :${CDP_PORT} (${CHROME_BIN})"
  # --no-sandbox: typical for root/container; --disable-gpu for Xvfb.
  "${CHROME_BIN}" \
    --no-first-run \
    --no-default-browser-check \
    --disable-gpu \
    --disable-dev-shm-usage \
    --no-sandbox \
    --window-size="${WIDTH},${HEIGHT}" \
    --remote-debugging-address=0.0.0.0 \
    --remote-debugging-port="${CDP_PORT}" \
    about:blank \
    >/tmp/danmo-desktop/chromium.log 2>&1 &
else
  echo "==> WARNING: Chromium not found; CDP unavailable" >&2
fi

echo "==> x11vnc :${VNC_PORT}"
x11vnc -display "${DISPLAY}" -forever -shared -rfbport "${VNC_PORT}" -localhost -nopw \
  >/tmp/danmo-desktop/x11vnc.log 2>&1 &

NOVNC_ROOT=""
for d in /usr/share/novnc /usr/share/novnc/utils/.. /usr/share/novnc; do
  if [[ -f "${d}/vnc.html" ]] || [[ -f "${d}/vnc_lite.html" ]]; then
    NOVNC_ROOT="$(cd "$d" && pwd)"
    break
  fi
done
if [[ -z "$NOVNC_ROOT" ]] && [[ -d /usr/share/novnc ]]; then
  NOVNC_ROOT=/usr/share/novnc
fi

if [[ -n "$NOVNC_ROOT" ]] && command -v websockify >/dev/null 2>&1; then
  echo "==> noVNC :${NOVNC_PORT} (root=${NOVNC_ROOT})"
  websockify --web="${NOVNC_ROOT}" "0.0.0.0:${NOVNC_PORT}" "127.0.0.1:${VNC_PORT}" \
    >/tmp/danmo-desktop/novnc.log 2>&1 &
else
  echo "==> WARNING: noVNC/websockify unavailable" >&2
fi

echo "==> desktop ready DISPLAY=${DISPLAY} CDP=${CDP_PORT} noVNC=${NOVNC_PORT}"
# Keep the container alive; exec_shell uses docker/podman exec against this PID 1.
exec sleep infinity

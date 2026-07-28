#!/usr/bin/env bash
# Build the bundled agent OCI image and save as a local tar (no registry push/pull).
# Output: out/env/danmo-work-env-linux-<arch>.tar
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

IMAGE_TAG="${DQ_ENV_IMAGE_TAG:-localhost/danmo-work-env:bundled}"
DOCKERFILE="${DQ_ENV_DOCKERFILE:-$DQ_ROOT/environments/agent-base/Dockerfile}"
CONTEXT="${DQ_ENV_CONTEXT:-$DQ_ROOT/environments/agent-base}"

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) ARCH="$ARCH_RAW" ;;
esac
# Allow CI cross-build override (linux/amd64, linux/arm64).
PLATFORM="${DQ_ENV_PLATFORM:-linux/${ARCH}}"
OUT_ARCH="${DQ_ENV_OUT_ARCH:-$ARCH}"

dq_ensure_out_layout
mkdir -p "$DQ_OUT/env"
OUT_TAR="${DQ_ENV_TAR_OUT:-$DQ_OUT/env/danmo-work-env-linux-${OUT_ARCH}.tar}"

ENGINE=""
if command -v podman >/dev/null 2>&1; then
  ENGINE=podman
elif command -v docker >/dev/null 2>&1; then
  ENGINE=docker
else
  echo "ERROR: need podman or docker to build env tar" >&2
  exit 1
fi

echo "==> Building $IMAGE_TAG via $ENGINE (platform=$PLATFORM)"
BUILD_ARGS=(build -t "$IMAGE_TAG" -f "$DOCKERFILE" --build-arg "TARGETARCH=${OUT_ARCH}")
if [[ "$ENGINE" == "docker" ]]; then
  BUILD_ARGS+=(--platform "$PLATFORM")
elif podman build --help 2>&1 | grep -q -- '--platform'; then
  BUILD_ARGS+=(--platform "$PLATFORM")
fi
"$ENGINE" "${BUILD_ARGS[@]}" "$CONTEXT"

echo "==> Saving image → $OUT_TAR"
rm -f "$OUT_TAR"
"$ENGINE" save -o "$OUT_TAR" "$IMAGE_TAG"
ls -lh "$OUT_TAR"
echo "==> Done (tag=$IMAGE_TAG engine=$ENGINE)"

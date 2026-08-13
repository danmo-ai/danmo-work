#!/usr/bin/env bash
# Build the agent OCI image and save as a gzipped tar (no registry push/pull).
# Output: out/env/danmo-work-env-linux-<arch>.tar.gz (release asset; runtime gunzips before load)
#
# Cross-build:
#   DQ_ENV_PLATFORM=linux/arm64 DQ_ENV_OUT_ARCH=arm64 ./scripts/build_env_tar.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

DOCKERFILE="${DQ_ENV_DOCKERFILE:-$DQ_ROOT/environments/agent-base/Dockerfile}"
CONTEXT="${DQ_ENV_CONTEXT:-$DQ_ROOT/environments/agent-base}"

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) ARCH="$ARCH_RAW" ;;
esac
PLATFORM="${DQ_ENV_PLATFORM:-linux/${ARCH}}"
OUT_ARCH="${DQ_ENV_OUT_ARCH:-$ARCH}"
# Tag uniquely per arch so parallel CI matrix jobs don't collide when saving.
IMAGE_TAG_ARCH="${DQ_ENV_IMAGE_TAG:-localhost/danmo-work-env:bundled}-${OUT_ARCH}"

dq_ensure_out_layout
mkdir -p "$DQ_OUT/env"
OUT_TAR="${DQ_ENV_TAR_OUT:-$DQ_OUT/env/danmo-work-env-linux-${OUT_ARCH}.tar.gz}"

ENGINE=""
if command -v docker >/dev/null 2>&1; then
  ENGINE=docker
elif command -v podman >/dev/null 2>&1; then
  ENGINE=podman
else
  echo "ERROR: need docker or podman to build env tar" >&2
  exit 1
fi

echo "==> Building $IMAGE_TAG_ARCH via $ENGINE (platform=$PLATFORM)"

if [[ "$ENGINE" == "docker" ]] && docker buildx version >/dev/null 2>&1; then
  docker buildx build \
    --platform "$PLATFORM" \
    -t "$IMAGE_TAG_ARCH" \
    -f "$DOCKERFILE" \
    --build-arg "TARGETARCH=${OUT_ARCH}" \
    --load \
    "$CONTEXT"
elif [[ "$ENGINE" == "podman" ]]; then
  podman build \
    --platform "$PLATFORM" \
    -t "$IMAGE_TAG_ARCH" \
    -f "$DOCKERFILE" \
    --build-arg "TARGETARCH=${OUT_ARCH}" \
    "$CONTEXT"
else
  docker build \
    --platform "$PLATFORM" \
    -t "$IMAGE_TAG_ARCH" \
    -f "$DOCKERFILE" \
    --build-arg "TARGETARCH=${OUT_ARCH}" \
    "$CONTEXT"
fi

# Also tag the canonical name for local load expectations on native arch builds.
if [[ "$OUT_ARCH" == "$ARCH" ]]; then
  "$ENGINE" tag "$IMAGE_TAG_ARCH" "${DQ_ENV_IMAGE_TAG:-localhost/danmo-work-env:bundled}" || true
fi

echo "==> Saving image → $OUT_TAR (gzipped)"
rm -f "$OUT_TAR"
"$ENGINE" save "$IMAGE_TAG_ARCH" | gzip -6 > "$OUT_TAR"
ls -lh "$OUT_TAR"
echo "==> Done (tag=$IMAGE_TAG_ARCH engine=$ENGINE arch=$OUT_ARCH)"

#!/usr/bin/env bash
# Build the agent OCI image and save as a gzipped tar (no registry push/pull).
#
# Profiles (DQ_ENV_PROFILE):
#   cli     (default) → Alpine shell image
#             tag: localhost/danmo-work-env:bundled
#             out:  out/env/danmo-work-env-linux-<arch>.tar.gz
#   desktop           → Ubuntu Xvfb desktop + Chromium CDP + noVNC
#             tag: localhost/danmo-work-env:desktop
#             out:  out/env/danmo-work-env-desktop-linux-<arch>.tar.gz
#
# Cross-build:
#   DQ_ENV_PLATFORM=linux/arm64 DQ_ENV_OUT_ARCH=arm64 ./scripts/build_env_tar.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

PROFILE="${DQ_ENV_PROFILE:-cli}"
case "$PROFILE" in
  cli|"")
    PROFILE=cli
    DEFAULT_DOCKERFILE="$DQ_ROOT/environments/agent-base/Dockerfile"
    DEFAULT_CONTEXT="$DQ_ROOT/environments/agent-base"
    DEFAULT_IMAGE_TAG="localhost/danmo-work-env:bundled"
    DEFAULT_TAR_SUFFIX="danmo-work-env-linux"
    ;;
  desktop)
    DEFAULT_DOCKERFILE="$DQ_ROOT/environments/agent-desktop/Dockerfile"
    DEFAULT_CONTEXT="$DQ_ROOT/environments/agent-desktop"
    DEFAULT_IMAGE_TAG="localhost/danmo-work-env:desktop"
    DEFAULT_TAR_SUFFIX="danmo-work-env-desktop-linux"
    ;;
  *)
    echo "ERROR: unknown DQ_ENV_PROFILE=$PROFILE (cli|desktop)" >&2
    exit 1
    ;;
esac

DOCKERFILE="${DQ_ENV_DOCKERFILE:-$DEFAULT_DOCKERFILE}"
CONTEXT="${DQ_ENV_CONTEXT:-$DEFAULT_CONTEXT}"

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) ARCH="$ARCH_RAW" ;;
esac
PLATFORM="${DQ_ENV_PLATFORM:-linux/${ARCH}}"
OUT_ARCH="${DQ_ENV_OUT_ARCH:-$ARCH}"
# Tag uniquely per arch so parallel CI matrix jobs don't collide when saving.
IMAGE_TAG_BASE="${DQ_ENV_IMAGE_TAG:-$DEFAULT_IMAGE_TAG}"
IMAGE_TAG_ARCH="${IMAGE_TAG_BASE}-${OUT_ARCH}"

dq_ensure_out_layout
mkdir -p "$DQ_OUT/env"
OUT_TAR="${DQ_ENV_TAR_OUT:-$DQ_OUT/env/${DEFAULT_TAR_SUFFIX}-${OUT_ARCH}.tar.gz}"

ENGINE=""
if command -v docker >/dev/null 2>&1; then
  ENGINE=docker
elif command -v podman >/dev/null 2>&1; then
  ENGINE=podman
else
  echo "ERROR: need docker or podman to build env tar" >&2
  exit 1
fi

echo "==> Building profile=$PROFILE $IMAGE_TAG_ARCH via $ENGINE (platform=$PLATFORM)"

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

# Retag to the canonical name so the saved tar carries the expected tag
# (runtime loads tars on machines where the canonical tag never existed).
IMAGE_TAG="$IMAGE_TAG_BASE"
if "$ENGINE" image inspect "$IMAGE_TAG" >/dev/null 2>&1; then
  SAVE_TAG="$IMAGE_TAG"
elif "$ENGINE" tag "$IMAGE_TAG_ARCH" "$IMAGE_TAG" 2>/dev/null; then
  SAVE_TAG="$IMAGE_TAG"
else
  SAVE_TAG="$IMAGE_TAG_ARCH"
fi

echo "==> Saving image → $OUT_TAR (gzipped, tag=$SAVE_TAG)"
rm -f "$OUT_TAR"
"$ENGINE" save "$SAVE_TAG" | gzip -6 > "$OUT_TAR"
ls -lh "$OUT_TAR"
echo "==> Done (profile=$PROFILE tag=$SAVE_TAG engine=$ENGINE arch=$OUT_ARCH)"

# Danmo Work bundled agent environment (ultra-light).
# Built in CI → docker/podman save → out/env/danmo-work-env-*.tar
# Runtime loads the tar locally; never pulls from a registry.
#
# Contents: bash, git, curl, ca-certs, python3, Node.js LTS (official tarball), jq.
# No project source COPY; workspace is bind-mounted at /workspace.

FROM ubuntu:24.04

ARG NODE_VERSION=22.14.0
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    PATH=/usr/local/node/bin:$PATH

RUN apt-get -o Acquire::Check-Valid-Until=false -o Acquire::Check-Date=false update -qq \
  && apt-get -o Acquire::Check-Valid-Until=false -o Acquire::Check-Date=false install -y -qq --no-install-recommends \
       bash ca-certificates curl git jq python3 python3-pip python3-venv xz-utils \
  && rm -rf /var/lib/apt/lists/*

# Official Node binary (no nvm). Map Docker TARGETARCH → Node arch names.
RUN set -euo pipefail; \
  arch="${TARGETARCH:-amd64}"; \
  case "$arch" in \
    amd64|x86_64) node_arch=x64 ;; \
    arm64|aarch64) node_arch=arm64 ;; \
    *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
  esac; \
  curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" \
    | tar -xJ -C /usr/local; \
  mv "/usr/local/node-v${NODE_VERSION}-linux-${node_arch}" /usr/local/node; \
  node -v && npm -v && git --version && python3 --version && jq --version

WORKDIR /workspace

# Keep container alive for long-lived per-project exec; overridden by runtime create.
CMD ["sleep", "infinity"]

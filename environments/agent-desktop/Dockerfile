# Danmo Work desktop agent environment (optional download).
# Minimal Ubuntu desktop for computer use + browser CDP inside the OCI sandbox.
# Built in CI → docker/podman save → out/env/danmo-work-env-desktop-linux-*.tar.gz
# Runtime loads the tar locally; never pulls from a registry.
#
# Stack (Anthropic computer-use-demo / E2B Desktop inspired, minimized):
#   Xvfb :1 → openbox → Chromium (--remote-debugging-port=9222) → x11vnc + noVNC :6080

FROM ubuntu:24.04

ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    DISPLAY=:1 \
    DISPLAY_WIDTH=1280 \
    DISPLAY_HEIGHT=800 \
    CDP_PORT=9222 \
    NOVNC_PORT=6080

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    unzip \
    git \
    jq \
    bash \
    xvfb \
    x11vnc \
    openbox \
    xdotool \
    scrot \
    imagemagick \
    x11-utils \
    xterm \
    fonts-dejavu-core \
    fonts-liberation \
    libnss3 \
    libatk-bridge2.0-0 \
    libgtk-3-0 \
    libx11-xcb1 \
    libxcb-dri3-0 \
    libdrm2 \
    libgbm1 \
    libasound2t64 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    libcups2 \
    python3 \
    python3-numpy \
    novnc \
    websockify \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

# Chrome for Testing (real Chromium builds for amd64 + arm64; not the Ubuntu snap stub).
RUN set -eux; \
  arch="${TARGETARCH:-amd64}"; \
  case "$arch" in \
    amd64|x86_64) cft_plat=linux64 ;; \
    arm64|aarch64) cft_plat=linux-arm64 ;; \
    *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
  esac; \
  ver="$(curl -fsSL https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions.json \
    | jq -r '.channels.Stable.version')"; \
  url="https://storage.googleapis.com/chrome-for-testing-public/${ver}/${cft_plat}/chrome-${cft_plat}.zip"; \
  curl -fsSL "$url" -o /tmp/chrome.zip; \
  unzip -q /tmp/chrome.zip -d /opt; \
  rm -f /tmp/chrome.zip; \
  chrome_dir="$(find /opt -maxdepth 1 -type d -name 'chrome-*' | head -1)"; \
  ln -sf "${chrome_dir}/chrome" /usr/local/bin/danmo-chromium; \
  /usr/local/bin/danmo-chromium --version

COPY entrypoint.sh /usr/local/bin/desktop-entrypoint.sh
RUN chmod +x /usr/local/bin/desktop-entrypoint.sh

WORKDIR /workspace

EXPOSE 9222 6080

ENTRYPOINT ["/usr/local/bin/desktop-entrypoint.sh"]

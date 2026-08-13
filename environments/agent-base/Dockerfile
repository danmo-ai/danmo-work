# Danmo Work bundled agent environment (ultra-light + apk).
# Built in CI → docker/podman save → out/env/danmo-work-env-*.tar.gz
# Runtime loads the tar locally; never pulls from a registry.
#
# Philosophy: tiny alpine base (busybox coreutils) + apk — do NOT bake Node/Python.
# git/curl/jq are baked (small + used constantly); heavier toolchains install
# on demand: apk add --no-cache python3 nodejs …
# No project source COPY; workspace is bind-mounted at /workspace.

FROM alpine:3.24

ENV LANG=C.UTF-8

# bash (agents write bash scripts) + CA bundle (https via curl / apk) +
# git/curl/jq baked (small, used constantly). Node/Python/Go etc.:
# apk add --no-cache <pkg> when needed.
RUN apk add --no-cache bash ca-certificates-bundle curl git jq \
  && git --version && curl --version | head -1 && jq --version

WORKDIR /workspace

# Keep container alive for long-lived per-project exec; overridden by runtime create.
CMD ["sleep", "infinity"]

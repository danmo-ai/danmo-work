# Danmo Work bundled agent environment (ultra-light + apt).
# Built in CI → docker/podman save → out/env/danmo-work-env-*.tar
# Runtime loads the tar locally; never pulls from a registry.
#
# Philosophy: small base + full Debian apt — do NOT bake Node/Python.
# Agents install toolchains on demand: apt-get update && apt-get install -y …
# No project source COPY; workspace is bind-mounted at /workspace.

FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8

RUN apt-get -o Acquire::Check-Valid-Until=false -o Acquire::Check-Date=false update -qq \
  && apt-get -o Acquire::Check-Valid-Until=false -o Acquire::Check-Date=false install -y -qq --no-install-recommends \
       bash ca-certificates curl git jq xz-utils \
  && rm -rf /var/lib/apt/lists/* \
  && git --version && curl --version | head -1 && jq --version

WORKDIR /workspace

# Keep container alive for long-lived per-project exec; overridden by runtime create.
CMD ["sleep", "infinity"]

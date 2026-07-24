APP_NAME := danmo-work
SERVER_BIN := $(CURDIR)/out/server/$(APP_NAME)
CLI_BIN := $(CURDIR)/out/server/$(APP_NAME)-cli
TUI_BIN := $(CURDIR)/out/server/$(APP_NAME)-tui
FRONTEND_DIR := $(CURDIR)/frontend
OUT_DIR := $(CURDIR)/out
FRONTEND_DIST := $(OUT_DIR)/frontend/dist
RELEASE_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -w -X 'danmo-work/server/api/v1.Version=$(RELEASE_VERSION)'

export DQ_APP_NAME := $(APP_NAME)

.PHONY: help backend dev-web dev-desktop dev-cli dev-tui stop \
	frontend-install frontend-dev frontend-build frontend-typecheck \
	check-layers test test-integration \
	build-go build-server build-cli build-tui build-sidecar build build-all clean \
	pack-prereqs pack-macos-desktop pack-linux-server pack-windows-desktop \
	eval-harbor-bin eval-harbor-base eval-harbor-sync-tb2 eval-harbor-smoke eval-harbor-suite eval-harbor-compare

EVAL_BIN_DIR := $(OUT_DIR)/eval
EVAL_CLI_BIN := $(EVAL_BIN_DIR)/danmo-work-cli
# Podman on Apple Silicon typically runs linux/arm64; Linux x86 hosts use amd64.
EVAL_GOARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
# First synced TB2 task (after `make eval-harbor-sync-tb2`); override with HARBOR_TASK=...
HARBOR_TASK ?= $(shell ls -1d $(CURDIR)/evals/dq_harbor/tasks/*/ 2>/dev/null | head -1 | sed 's:/*$$::')
HARBOR_MODEL ?= $(WORK_MODEL)
# Harbor 0.19 has no built-in "podman" env; use docker API against Podman via DOCKER_HOST.
HARBOR_ENV ?= docker
PODMAN_BIN ?= $(shell command -v podman 2>/dev/null || echo /opt/podman/bin/podman)

help:
	@echo "Danmo Work"
	@echo ""
	@echo "Dev:"
	@echo "  backend       Start backend only (:7801) — for Go debugger"
	@echo "  dev-web       Backend + Vite (browser :5801)"
	@echo "  dev-desktop   Backend + Tauri webview (set SKIP_BACKEND=1 to use external backend)"
	@echo "  dev-cli       Run CLI directly (no server needed)"
	@echo "  dev-tui       Run TUI directly (no server needed)"
	@echo "  stop          Stop all dev processes"
	@echo ""
	@echo "Frontend:  frontend-install | frontend-dev | frontend-build | frontend-typecheck"
	@echo "Test:      check-layers | test | test-integration"
	@echo "Eval:      eval-harbor-bin | eval-harbor-base | eval-harbor-sync-tb2 | eval-harbor-smoke | eval-harbor-suite | eval-harbor-compare"
	@echo "Build:     build | build-all | build-go | build-server | build-cli | build-tui | build-sidecar | clean"
	@echo "Release:   pack-macos-desktop | pack-linux-server | pack-windows-desktop"

# Backend only (for Go debugger or separate frontend)
backend:
	@chmod +x scripts/*.sh
	@./scripts/start_backend.sh

# Web browser development
dev-web:
	@chmod +x scripts/*.sh
	@./scripts/start_web.sh

# Desktop (Tauri) development
dev-desktop:
	@chmod +x scripts/*.sh
	@./scripts/start_desktop.sh

# CLI (no server needed)
dev-cli:
	go run ./cli

# TUI (no server needed)
dev-tui:
	go run ./tui

stop:
	@chmod +x scripts/*.sh
	@./scripts/stop.sh

frontend-install:
	cd $(FRONTEND_DIR) && npm install

frontend-dev: frontend-install
	cd $(FRONTEND_DIR) && npm run dev

frontend-build: frontend-install
	cd $(FRONTEND_DIR) && npm run build

frontend-typecheck: frontend-install
	cd $(FRONTEND_DIR) && npm run typecheck

check-layers:
	go run scripts/check_layers.go

test: check-layers
	go test ./... -count=1

test-integration:
	go test ./test/integration/... -count=1

build-go: build-server build-cli build-tui

build-server:
	@mkdir -p $(OUT_DIR)/server
	go build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN) ./server

build-cli:
	@mkdir -p $(OUT_DIR)/server
	go build -ldflags "$(LDFLAGS)" -o $(CLI_BIN) ./cli

build-tui:
	@mkdir -p $(OUT_DIR)/server
	go build -ldflags "$(LDFLAGS)" -o $(TUI_BIN) ./tui

build-sidecar:
	@chmod +x scripts/*.sh
	@./scripts/build_sidecar.sh

build build-all: frontend-build build-go

clean:
	rm -rf $(OUT_DIR)

pack-prereqs:
	@command -v npm >/dev/null 2>&1 || (echo "npm not found" >&2; exit 1)
	@command -v cargo >/dev/null 2>&1 || (echo "cargo not found" >&2; exit 1)
	@echo "pack-prereqs OK"

pack-macos-desktop: pack-prereqs frontend-build
	@chmod +x scripts/*.sh
	@RELEASE_VERSION=$(RELEASE_VERSION) ./scripts/pack_desktop_macos.sh

pack-linux-server: frontend-build build-go
	@chmod +x scripts/*.sh
	@RELEASE_VERSION=$(RELEASE_VERSION) ./scripts/pack_linux_server.sh

pack-windows-desktop: pack-prereqs frontend-build
	@chmod +x scripts/*.sh
	@RELEASE_VERSION=$(RELEASE_VERSION) ./scripts/pack_desktop_windows.sh

# Cross-compile CLI for Harbor task containers (linux).
eval-harbor-bin:
	@mkdir -p $(EVAL_BIN_DIR)
	GOOS=linux GOARCH=$(EVAL_GOARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(EVAL_CLI_BIN) ./cli
	@echo "built $(EVAL_CLI_BIN) (linux/$(EVAL_GOARCH))"

# Shared task image with nvm/Node/OpenCode/Python preinstalled (speeds OpenCode setup).
eval-harbor-base:
	@chmod +x $(CURDIR)/evals/dq_harbor/build_base_image.sh
	@$(CURDIR)/evals/dq_harbor/build_base_image.sh

# Download official terminal-bench@2.0 and rewrite task Dockerfiles to FROM dq-harbor-base:local.
eval-harbor-sync-tb2:
	@chmod +x $(CURDIR)/evals/dq_harbor/sync_tb2_tasks.sh
	@$(CURDIR)/evals/dq_harbor/sync_tb2_tasks.sh

# Smoke: oracle then DanQing on one synced TB2 task (HARBOR_TASK or first under tasks/).
# Requires: Podman, harbor, make eval-harbor-sync-tb2, LLM credentials.
eval-harbor-smoke: eval-harbor-sync-tb2 eval-harbor-base eval-harbor-bin
	@test -x "$(PODMAN_BIN)" || (echo "podman not found (tried $(PODMAN_BIN))" >&2; exit 1)
	@command -v harbor >/dev/null 2>&1 || (echo "harbor not found — install with: uv tool install harbor" >&2; exit 1)
	@test -n "$(HARBOR_MODEL)" || (echo "Set WORK_MODEL or HARBOR_MODEL (e.g. deepseek/deepseek-v4-flash)" >&2; exit 1)
	@test -n "$(HARBOR_TASK)" || (echo "no tasks — run make eval-harbor-sync-tb2" >&2; exit 1)
	chmod +x $(HARBOR_TASK)/tests/test.sh $(HARBOR_TASK)/solution/solve.sh
	@SOCK="$$($(PODMAN_BIN) machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || true)"; \
	  if [ -n "$$SOCK" ]; then export DOCKER_HOST="unix://$$SOCK"; fi; \
	  export PATH="$(dir $(PODMAN_BIN)):$$PATH"; \
	  echo "==> oracle smoke on $(HARBOR_TASK) (env=$(HARBOR_ENV) DOCKER_HOST=$$DOCKER_HOST)"; \
	  harbor run --path $(HARBOR_TASK) --agent oracle --env $(HARBOR_ENV) --n-concurrent 1; \
	  echo "==> DanQing agent on $(HARBOR_TASK) model=$(HARBOR_MODEL)"; \
	  PYTHONPATH=$(CURDIR)/evals \
	  DANQING_CLI_BIN=$(EVAL_CLI_BIN) \
	  harbor run --path $(HARBOR_TASK) \
		--agent dq_harbor.agent:DanmoWorkAgent \
		--model $(HARBOR_MODEL) \
		--env $(HARBOR_ENV) \
		--n-concurrent 1 \
		$(if $(WORK_API_KEY),--ae WORK_API_KEY=$(WORK_API_KEY),) \
		$(if $(WORK_BASE_URL),--ae WORK_BASE_URL=$(WORK_BASE_URL),) \
		$(if $(OPENAI_API_KEY),--ae OPENAI_API_KEY=$(OPENAI_API_KEY),) \
		$(if $(ANTHROPIC_API_KEY),--ae ANTHROPIC_API_KEY=$(ANTHROPIC_API_KEY),) \
		$(if $(OPENAI_BASE_URL),--ae OPENAI_BASE_URL=$(OPENAI_BASE_URL),)

# Full Terminal-Bench 2.0 suite (~89 tasks under evals/dq_harbor/tasks/).
eval-harbor-suite: eval-harbor-sync-tb2 eval-harbor-bin
	@test -x "$(PODMAN_BIN)" || (echo "podman not found (tried $(PODMAN_BIN))" >&2; exit 1)
	@command -v harbor >/dev/null 2>&1 || (echo "harbor not found — install with: uv tool install harbor" >&2; exit 1)
	@test -n "$(HARBOR_MODEL)" || (echo "Set WORK_MODEL or HARBOR_MODEL" >&2; exit 1)
	chmod +x $(CURDIR)/evals/dq_harbor/run_suite.sh
	@SOCK="$$($(PODMAN_BIN) machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || true)"; \
	  if [ -n "$$SOCK" ]; then export DOCKER_HOST="unix://$$SOCK"; fi; \
	  export PATH="$(dir $(PODMAN_BIN)):$$PATH"; \
	  HARBOR_ENV=$(HARBOR_ENV) HARBOR_MODEL=$(HARBOR_MODEL) \
	    $(CURDIR)/evals/dq_harbor/run_suite.sh oracle; \
	  HARBOR_ENV=$(HARBOR_ENV) HARBOR_MODEL=$(HARBOR_MODEL) \
	    DANQING_CLI_BIN=$(EVAL_CLI_BIN) \
	    $(CURDIR)/evals/dq_harbor/run_suite.sh dq_harbor.agent:DanmoWorkAgent

# Same suite for a comparison agent, e.g. make eval-harbor-compare HARBOR_COMPARE_AGENT=opencode
HARBOR_COMPARE_AGENT ?= opencode
eval-harbor-compare: eval-harbor-sync-tb2 eval-harbor-base
	@test -x "$(PODMAN_BIN)" || (echo "podman not found (tried $(PODMAN_BIN))" >&2; exit 1)
	@command -v harbor >/dev/null 2>&1 || (echo "harbor not found" >&2; exit 1)
	@test -n "$(HARBOR_MODEL)" || (echo "Set WORK_MODEL or HARBOR_MODEL" >&2; exit 1)
	chmod +x $(CURDIR)/evals/dq_harbor/run_suite.sh
	@SOCK="$$($(PODMAN_BIN) machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || true)"; \
	  if [ -n "$$SOCK" ]; then export DOCKER_HOST="unix://$$SOCK"; fi; \
	  export PATH="$(dir $(PODMAN_BIN)):$$PATH"; \
	  HARBOR_ENV=$(HARBOR_ENV) HARBOR_MODEL=$(HARBOR_MODEL) \
	    PYTHONPATH=$(CURDIR)/evals \
	    $(CURDIR)/evals/dq_harbor/run_suite.sh $(HARBOR_COMPARE_AGENT)

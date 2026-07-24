#!/usr/bin/env bash
# Full-suite compare: DanQing vs OpenCode vs OpenHands on Terminal-Bench 2.0 (~89 tasks).
# Usage (from repo root):
#   ./evals/dq_harbor/compare_agents.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

export PATH="/opt/podman/bin:/tmp:${PATH:-}"
[[ -x /tmp/docker ]] || ln -sf /opt/podman/bin/podman /tmp/docker
PODMAN_BIN="${PODMAN_BIN:-$(command -v podman 2>/dev/null || echo /opt/podman/bin/podman)}"
SOCK="$("$PODMAN_BIN" machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || true)"
[[ -n "$SOCK" ]] && export DOCKER_HOST="unix://$SOCK"

export HARBOR_ENV="${HARBOR_ENV:-docker}"
export HARBOR_N_CONCURRENT="${HARBOR_N_CONCURRENT:-1}"
# Full-suite compare: ignore a leftover HARBOR_TASKS filter from the shell.
unset HARBOR_TASKS

# Prefer DeepSeek from local Teams DB if env not set.
if [[ -z "${WORK_API_KEY:-}" && -f "$HOME/.danmo-work/work.db" ]]; then
  WORK_API_KEY="$(sqlite3 "$HOME/.danmo-work/work.db" "SELECT api_key FROM llm_configs WHERE name='deepseek' LIMIT 1;" 2>/dev/null || true)"
  export WORK_API_KEY
fi
export WORK_BASE_URL="${WORK_BASE_URL:-https://api.deepseek.com}"
export WORK_MODEL="${WORK_MODEL:-deepseek/deepseek-v4-flash}"
export HARBOR_MODEL="${HARBOR_MODEL:-$WORK_MODEL}"

# Credentials for third-party agents
export OPENAI_API_KEY="${OPENAI_API_KEY:-$WORK_API_KEY}"
# Prefer /v1 so OpenAI-compat clients hit chat.completions (not /responses).
export OPENAI_BASE_URL="${OPENAI_BASE_URL:-${WORK_BASE_URL%/}/v1}"
export DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:-$WORK_API_KEY}"
# OpenHands / LiteLLM
export LLM_API_KEY="${LLM_API_KEY:-$WORK_API_KEY}"
export LLM_BASE_URL="${LLM_BASE_URL:-$OPENAI_BASE_URL}"

# Model strings:
# - DanQing: provider/model matching its LLMConfig (deepseek/deepseek-v4-flash)
# - OpenCode: native deepseek provider (openai/* + custom baseURL hits /responses → 404)
# - OpenHands: openai/<model> + LLM_BASE_URL, or deepseek/<model>
DANQING_MODEL="${DANQING_MODEL:-$WORK_MODEL}"
COMPAT_MODEL="${COMPAT_MODEL:-deepseek/deepseek-v4-flash}"
OPENHANDS_MODEL="${OPENHANDS_MODEL:-openai/deepseek-v4-flash}"
# Prebaked Node/OpenCode image + skip-install agent (see images/base/)
OPENCODE_AGENT="${OPENCODE_AGENT:-dq_harbor.agent_opencode:OpenCodePrebuilt}"

make eval-harbor-base
make eval-harbor-sync-tb2
make eval-harbor-bin

OUT_DIR="$ROOT/evals/dq_harbor/compare_results/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$OUT_DIR"
echo "results dir: $OUT_DIR"
echo "DOCKER_HOST=${DOCKER_HOST:-}"
echo "DANQING_MODEL=$DANQING_MODEL COMPAT_MODEL=$COMPAT_MODEL"

run_one() {
  local label="$1" agent="$2" model="$3"
  local log="$OUT_DIR/${label}.log"
  echo ""
  echo "######## START $label agent=$agent model=$model ########"
  set +e
  if [[ "$agent" == *DanQing* ]]; then
    HARBOR_MODEL="$model" HARBOR_ENV="$HARBOR_ENV" \
      DANQING_CLI_BIN="$ROOT/out/eval/danmo-work-cli" \
      "$ROOT/evals/dq_harbor/run_suite.sh" "$agent" >"$log" 2>&1
  else
    # Pass OpenAI-compatible keys for third-party agents
    HARBOR_MODEL="$model" HARBOR_ENV="$HARBOR_ENV" \
      WORK_API_KEY="$WORK_API_KEY" WORK_BASE_URL="$WORK_BASE_URL" \
      OPENAI_API_KEY="$OPENAI_API_KEY" OPENAI_BASE_URL="$OPENAI_BASE_URL" \
      DEEPSEEK_API_KEY="$DEEPSEEK_API_KEY" \
      LLM_API_KEY="$LLM_API_KEY" LLM_BASE_URL="$LLM_BASE_URL" \
      LLM_MODEL="$model" \
      "$ROOT/evals/dq_harbor/run_suite.sh" "$agent" >"$log" 2>&1
  fi
  local ec=$?
  set -e
  echo "######## END $label exit=$ec ########"
  # Parse suite summary line
  rg -n "suite summary|OK |FAIL " "$log" | tail -40 || true
  echo "$ec" >"$OUT_DIR/${label}.exit"
}

run_one "danqing" "dq_harbor.agent:DanmoWorkAgent" "$DANQING_MODEL"
# Third-party agents need longer timeouts (install + run).
export HARBOR_TIMEOUT_MULT="${HARBOR_TIMEOUT_MULT:-2}"
export HARBOR_AGENT_TIMEOUT_MULT="${HARBOR_AGENT_TIMEOUT_MULT:-3}"
run_one "opencode" "$OPENCODE_AGENT" "$COMPAT_MODEL"
# Prefer SDK: full openhands-ai pip install often fails/times out in task containers.
run_one "openhands" "${OPENHANDS_AGENT:-openhands-sdk}" "$OPENHANDS_MODEL"

python3 "$ROOT/evals/dq_harbor/summarize_compare.py" "$OUT_DIR" | tee "$OUT_DIR/SUMMARY.md"
echo "Wrote $OUT_DIR/SUMMARY.md"

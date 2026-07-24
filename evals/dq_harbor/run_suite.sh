#!/usr/bin/env bash
# Run Terminal-Bench 2.0 tasks under evals/dq_harbor/tasks/ for one agent.
# Tasks are official TB2, synced via sync_tb2_tasks.sh (Dockerfiles FROM dq-harbor-base:local).
#
# Usage:
#   ./evals/dq_harbor/sync_tb2_tasks.sh
#   make eval-harbor-base
#   ./evals/dq_harbor/run_suite.sh oracle
#   ./evals/dq_harbor/run_suite.sh dq_harbor.agent:DanmoWorkAgent
#   ./evals/dq_harbor/run_suite.sh opencode
#
# Harbor 0.19 has no built-in podman env: use --env docker with DOCKER_HOST
# pointing at the Podman machine socket.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TASKS_DIR="$ROOT/evals/dq_harbor/tasks"
AGENT="${1:-}"
ENV_TYPE="${HARBOR_ENV:-docker}"
MODEL="${HARBOR_MODEL:-${WORK_MODEL:-}}"
N_CONCURRENT="${HARBOR_N_CONCURRENT:-1}"
# OpenCode/OpenHands install (nvm/npm/pip) is slow; give agents more wall time.
TIMEOUT_MULT="${HARBOR_TIMEOUT_MULT:-1}"
AGENT_TIMEOUT_MULT="${HARBOR_AGENT_TIMEOUT_MULT:-1}"
MAX_TASKS="${HARBOR_MAX_TASKS:-${HARBOR_N_TASKS:-}}"
PODMAN_BIN="${PODMAN_BIN:-$(command -v podman 2>/dev/null || true)}"
[[ -z "$PODMAN_BIN" && -x /opt/podman/bin/podman ]] && PODMAN_BIN=/opt/podman/bin/podman

if [[ -z "$AGENT" ]]; then
  echo "usage: $0 <agent>   e.g. oracle | dq_harbor.agent:DanmoWorkAgent | opencode" >&2
  exit 2
fi

# Default: use prebaked OpenCode agent (skips nvm when base image has it).
if [[ "$AGENT" == "opencode" ]]; then
  AGENT="${OPENCODE_AGENT:-dq_harbor.agent_opencode:OpenCodePrebuilt}"
fi

if [[ ! -d "$TASKS_DIR" ]] || ! compgen -G "$TASKS_DIR"/*/task.toml >/dev/null; then
  echo "no TB2 tasks under $TASKS_DIR — run: ./evals/dq_harbor/sync_tb2_tasks.sh" >&2
  exit 1
fi

if [[ "$AGENT" != "oracle" && -z "$MODEL" ]]; then
  echo "Set WORK_MODEL or HARBOR_MODEL (required for non-oracle agents)" >&2
  exit 2
fi

command -v harbor >/dev/null || { echo "harbor not found" >&2; exit 1; }
[[ -n "$PODMAN_BIN" && -x "$PODMAN_BIN" ]] || { echo "podman not found" >&2; exit 1; }
export PATH="$(dirname "$PODMAN_BIN"):$PATH"
if [[ -z "${DOCKER_HOST:-}" ]]; then
  SOCK="$("$PODMAN_BIN" machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || true)"
  if [[ -n "$SOCK" ]]; then
    export DOCKER_HOST="unix://$SOCK"
  fi
fi

AE_ARGS=()
[[ -n "${WORK_API_KEY:-}" ]] && AE_ARGS+=(--ae "WORK_API_KEY=$WORK_API_KEY")
[[ -n "${WORK_BASE_URL:-}" ]] && AE_ARGS+=(--ae "WORK_BASE_URL=$WORK_BASE_URL")
[[ -n "${OPENAI_API_KEY:-}" ]] && AE_ARGS+=(--ae "OPENAI_API_KEY=$OPENAI_API_KEY")
[[ -n "${ANTHROPIC_API_KEY:-}" ]] && AE_ARGS+=(--ae "ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY")
[[ -n "${OPENAI_BASE_URL:-}" ]] && AE_ARGS+=(--ae "OPENAI_BASE_URL=$OPENAI_BASE_URL")
# OpenCode native deepseek provider; OpenHands LiteLLM
[[ -n "${DEEPSEEK_API_KEY:-}" ]] && AE_ARGS+=(--ae "DEEPSEEK_API_KEY=$DEEPSEEK_API_KEY")
[[ -n "${LLM_API_KEY:-}" ]] && AE_ARGS+=(--ae "LLM_API_KEY=$LLM_API_KEY")
[[ -n "${LLM_BASE_URL:-}" ]] && AE_ARGS+=(--ae "LLM_BASE_URL=$LLM_BASE_URL")
[[ -n "${LLM_MODEL:-}" ]] && AE_ARGS+=(--ae "LLM_MODEL=$LLM_MODEL")

# Custom Harbor agents live under evals/dq_harbor/
if [[ "$AGENT" == dq_harbor.* ]]; then
  export PYTHONPATH="$ROOT/evals${PYTHONPATH:+:$PYTHONPATH}"
fi
if [[ "$AGENT" == "dq_harbor.agent:DanmoWorkAgent" || "$AGENT" == *DanmoWorkAgent* ]]; then
  BIN="${DANQING_CLI_BIN:-$ROOT/out/eval/danmo-work-cli}"
  if [[ ! -f "$BIN" ]]; then
    echo "missing $BIN — run: make eval-harbor-bin" >&2
    exit 1
  fi
  export DANQING_CLI_BIN="$BIN"
fi

shopt -s nullglob
if [[ -n "${HARBOR_TASKS:-}" ]]; then
  # Space-separated task directory names under tasks/
  tasks=()
  for name in $HARBOR_TASKS; do
    d="$TASKS_DIR/$name"
    [[ -d "$d" ]] || { echo "unknown task: $name" >&2; exit 2; }
    tasks+=("$d/")
  done
else
  tasks=("$TASKS_DIR"/*/)
fi
if [[ ${#tasks[@]} -eq 0 ]]; then
  echo "no tasks under $TASKS_DIR" >&2
  exit 1
fi
# Optional smoke limit (sorted for stable first-N). Bash 3.2 compatible.
if [[ -n "$MAX_TASKS" ]]; then
  limited=()
  while IFS= read -r line; do
    [[ -n "$line" ]] && limited+=("$line")
  done < <(printf '%s\n' "${tasks[@]}" | sort | head -n "$MAX_TASKS")
  tasks=("${limited[@]}")
fi

pass=0
fail=0
for task_dir in "${tasks[@]}"; do
  task_dir="${task_dir%/}"
  name="$(basename "$task_dir")"
  chmod +x "$task_dir/tests/test.sh" "$task_dir/solution/solve.sh" 2>/dev/null || true
  echo ""
  echo "======== task=$name agent=$AGENT ========"
  args=(run --path "$task_dir" --agent "$AGENT" --env "$ENV_TYPE" --n-concurrent "$N_CONCURRENT")
  if [[ "$TIMEOUT_MULT" != "1" ]]; then
    args+=(--timeout-multiplier "$TIMEOUT_MULT")
  fi
  if [[ "$AGENT_TIMEOUT_MULT" != "1" ]]; then
    args+=(--agent-timeout-multiplier "$AGENT_TIMEOUT_MULT")
  fi
  if [[ "$AGENT" != "oracle" ]]; then
    args+=(--model "$MODEL")
  fi
  # Harbor exits 0 even when reward is 0 — judge by Mean reward.
  tmp_out="$(mktemp)"
  set +e
  harbor "${args[@]}" "${AE_ARGS[@]+"${AE_ARGS[@]}"}" | tee "$tmp_out"
  harbor_ec=${PIPESTATUS[0]}
  set -e
  mean="$(rg -o 'Mean: [0-9.]+' "$tmp_out" | tail -1 | awk '{print $2}')"
  rm -f "$tmp_out"
  if [[ "$harbor_ec" -eq 0 && -n "$mean" ]] && awk "BEGIN{exit !($mean >= 1)}"; then
    pass=$((pass + 1))
    echo "OK $name (mean=$mean)"
  else
    fail=$((fail + 1))
    echo "FAIL $name (mean=${mean:-n/a} harbor_ec=$harbor_ec)" >&2
  fi
done

echo ""
echo "======== suite summary agent=$AGENT pass=$pass fail=$fail total=$((pass + fail)) ========"
echo "Analyze turn logs: python3 $ROOT/evals/dq_harbor/analyze_failures.py --failed-only"
[[ "$fail" -eq 0 ]]

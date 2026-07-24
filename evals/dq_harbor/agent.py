"""Harbor BaseInstalledAgent adapter for Danmo Work.

Usage:
  make eval-harbor-bin
  make eval-harbor-sync-tb2
  PYTHONPATH=evals harbor run --path evals/dq_harbor/tasks/<task> \\
    --agent dq_harbor.agent:DanmoWorkAgent \\
    --model deepseek/deepseek-v4-flash \\
    --n-concurrent 1 \\
    --ae WORK_API_KEY=$WORK_API_KEY \\
    --ae WORK_BASE_URL=$WORK_BASE_URL
"""

from __future__ import annotations

import os
import shlex
from pathlib import Path

try:
    from typing import override
except ImportError:  # Python < 3.12
    def override(fn):  # type: ignore[misc]
        return fn

from harbor.agents.installed.base import BaseInstalledAgent, with_prompt_template
from harbor.environments.base import BaseEnvironment
from harbor.models.agent.context import AgentContext

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_HOST_BIN = REPO_ROOT / "out" / "eval" / "danmo-work-cli"
CONTAINER_BIN = "/usr/local/bin/danmo-work-cli"
CONTAINER_CONFIG = "/installed-agent/config.yaml"
CONTAINER_DATA = "/installed-agent/data"


class DanmoWorkAgent(BaseInstalledAgent):
    """Runs danmo-work-cli headless inside the Harbor task container."""

    SUPPORTS_ATIF = False

    @staticmethod
    @override
    def name() -> str:
        return "danqing"

    @override
    def version(self) -> str | None:
        return self._version or "dev"

    def _host_bin(self) -> Path:
        override = os.environ.get("DANQING_CLI_BIN", "").strip()
        if override:
            return Path(override)
        return DEFAULT_HOST_BIN

    @override
    async def install(self, environment: BaseEnvironment) -> None:
        host_bin = self._host_bin()
        if not host_bin.is_file():
            raise RuntimeError(
                f"DanQing CLI binary not found at {host_bin}. "
                "Run `make eval-harbor-bin` first (linux cross-compile)."
            )

        await self.exec_as_root(
            environment,
            command=(
                "mkdir -p /installed-agent /usr/local/bin /logs/agent && "
                "apt-get update -qq && "
                "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ca-certificates >/dev/null"
            ),
        )
        await environment.upload_file(host_bin, CONTAINER_BIN)
        await self.exec_as_root(
            environment,
            command=f"chmod +x {shlex.quote(CONTAINER_BIN)}",
        )

        config = """\
# Generated for Harbor smoke eval — do not use as a product config.
data:
  dir: "/installed-agent/data"
  database: "/installed-agent/work.db"
  store: "sqlite"
runtime:
  auto_approve: true
  sandbox:
    enabled: true
    mode: "workspace-write"
    network: "allow"
  browser:
    enabled: false
  turn:
    max_steps_default: 30
"""
        local_cfg = self.logs_dir / "danqing-config.yaml"
        local_cfg.write_text(config, encoding="utf-8")
        await environment.upload_file(local_cfg, CONTAINER_CONFIG)

    def _run_env(self) -> dict[str, str]:
        env: dict[str, str] = {
            "WORK_CONFIG": CONTAINER_CONFIG,
            "WORK_DATA_DIR": CONTAINER_DATA,
            "WORK_DB_PATH": "/installed-agent/work.db",
            "WORK_AUTO_APPROVE": "true",
            "WORK_EVAL_MODE": "1",
            "WORK_SANDBOX_NETWORK": "allow",
        }
        # Forward common provider keys into the container.
        for key in (
            "WORK_API_KEY",
            "WORK_BASE_URL",
            "WORK_MODEL",
            "WORK_PROVIDER_TYPE",
            "OPENAI_API_KEY",
            "ANTHROPIC_API_KEY",
            "OPENAI_BASE_URL",
        ):
            val = self._get_env(key)
            if val:
                env[key] = val
                if key == "OPENAI_BASE_URL" and "WORK_BASE_URL" not in env:
                    env["WORK_BASE_URL"] = val
        if self.model_name:
            env["WORK_MODEL"] = self.model_name
        return env

    @with_prompt_template
    @override
    async def run(
        self,
        instruction: str,
        environment: BaseEnvironment,
        context: AgentContext,
    ) -> None:
        model = self.model_name or self._get_env("WORK_MODEL") or ""
        if not model:
            raise RuntimeError(
                "No model configured. Pass --model provider/model "
                "(e.g. deepseek/deepseek-chat) or set WORK_MODEL."
            )

        report_path = "/logs/agent/report.json"
        logs_dir = "/logs/agent/turnlogs"
        goal = instruction.strip()
        cmd = (
            f"{CONTAINER_BIN} run "
            f"--workdir /app "
            f"--goal {shlex.quote(goal)} "
            f"--agent default "
            f"--model {shlex.quote(model)} "
            f"--timeout 8m "
            f"--auto-approve "
            f"--eval "
            f"--config {shlex.quote(CONTAINER_CONFIG)} "
            f"--data-dir {shlex.quote(CONTAINER_DATA)} "
            f"--report {shlex.quote(report_path)} "
            f"--logs-dir {shlex.quote(logs_dir)}"
        )
        base_url = self._get_env("WORK_BASE_URL") or self._get_env("OPENAI_BASE_URL")
        if base_url:
            cmd += f" --base-url {shlex.quote(base_url)}"
        api_key = (
            self._get_env("WORK_API_KEY")
            or self._get_env("OPENAI_API_KEY")
            or self._get_env("ANTHROPIC_API_KEY")
        )
        if api_key:
            cmd += f" --api-key {shlex.quote(api_key)}"

        # Best-effort: ensure logs dir exists even if the agent process fails early.
        await self.exec_as_root(
            environment,
            command=f"mkdir -p {shlex.quote(logs_dir)}",
        )

        try:
            await self.exec_as_agent(
                environment,
                command=cmd,
                env=self._run_env(),
                cwd="/app",
                timeout_sec=540,
            )
        finally:
            # Harbor syncs /logs/agent to the host trial dir; leave a pointer file.
            await environment.exec(
                command=(
                    "set +e; "
                    f"if [ -f {shlex.quote(logs_dir)}/analysis.md ]; then "
                    f"cp -f {shlex.quote(logs_dir)}/analysis.md /logs/agent/FAILURE_ANALYSIS.md; "
                    "fi; "
                    f"ls -la {shlex.quote(logs_dir)} >/logs/agent/turnlogs_listing.txt 2>/dev/null || true"
                ),
                user="root",
            )

        context.metadata = {
            "agent": self.name(),
            "model": model,
            "report_path": report_path,
            "logs_dir": logs_dir,
        }

    @override
    def populate_context_post_run(self, context: AgentContext) -> None:
        # Optional: leave empty; Harbor verifier scores the environment, not ATIF.
        pass

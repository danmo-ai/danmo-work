# Harbor eval — Terminal-Bench 2.0

官方 [Terminal-Bench 2.0](https://www.tbench.ai/)（**89** 题），本地同步后跑在 **`dq-harbor-base:local`** 上。  
Harness：[Harbor](https://github.com/laude-institute/harbor) + Podman。  
通过标准：Mean reward ≥ 1.0。非榜单提交（DanQing `SUPPORTS_ATIF = False`）。

**89 题不进 git**：目录 `evals/dq_harbor/tasks/` 已 gitignore，每台机器需自己同步。

---

## 一键怎么跑（推荐顺序）

在仓库根目录：

```bash
# 0) 依赖
#    - Podman（macOS: podman machine start）
#    - Harbor: uv tool install --upgrade 'harbor>=0.20'
#    - LLM Key（DanQing / 对比用）

# 1) 预构建镜像（含 Node + OpenCode，避免每题重装）
make eval-harbor-base

# 2) 同步官方 89 题 → evals/dq_harbor/tasks/（gitignore）
#    若 git clone 失败，用 token 走 GitHub API：
GH_TOKEN=$(gh auth token) make eval-harbor-sync-tb2
#    强制重拉：FORCE_RESYNC=1 GH_TOKEN=$(gh auth token) make eval-harbor-sync-tb2

# 3) 交叉编译 Linux CLI（注入容器）
make eval-harbor-bin

# 4a) 冒烟：1 题 oracle + DanQing
export WORK_MODEL=deepseek/deepseek-v4-flash
export WORK_API_KEY=...          # 或从 ~/.danmo-work/work.db 读 deepseek
export WORK_BASE_URL=https://api.deepseek.com
make eval-harbor-smoke

# 4b) 全量：oracle 再 DanQing（89 题，很久）
make eval-harbor-suite

# 4c) 三方对比：DanQing / OpenCode / OpenHands
./evals/dq_harbor/compare_agents.sh
```

检查题量：

```bash
find evals/dq_harbor/tasks -mindepth 1 -maxdepth 1 -type d | wc -l   # 应为 89
cat evals/dq_harbor/tasks/.tb2-synced
```

---

## 按 agent 跑套件

```bash
# 需已 sync + base（OpenCode）+ bin（DanQing）
./evals/dq_harbor/run_suite.sh oracle
./evals/dq_harbor/run_suite.sh dq_harbor.agent:DanmoWorkAgent
./evals/dq_harbor/run_suite.sh opencode          # → OpenCodePrebuilt
./evals/dq_harbor/run_suite.sh openhands-sdk
```

### 过滤

| 变量 | 作用 | 示例 |
|------|------|------|
| `HARBOR_MAX_TASKS` | 只跑排序后前 N 题 | `HARBOR_MAX_TASKS=3 ./evals/dq_harbor/run_suite.sh oracle` |
| `HARBOR_TASKS` | 指定题名（空格分隔） | `HARBOR_TASKS="fix-git openssl-selfsigned-cert" ./evals/dq_harbor/run_suite.sh oracle` |
| `HARBOR_N_CONCURRENT` | 并发（默认 1） | `HARBOR_N_CONCURRENT=2 ./evals/dq_harbor/run_suite.sh oracle` |
| `HARBOR_MODEL` / `WORK_MODEL` | 非 oracle 必填 | `deepseek/deepseek-v4-flash` |

单题直接 Harbor：

```bash
TASK=evals/dq_harbor/tasks/openssl-selfsigned-cert
harbor run --path "$TASK" --agent oracle --env docker --n-concurrent 1
```

---

## Makefile 目标

| Target | 做什么 |
|--------|--------|
| `make eval-harbor-base` | 构建 `dq-harbor-base:local` |
| `make eval-harbor-sync-tb2` | 同步 89 题并改写 Dockerfile / 清 `docker_image` |
| `make eval-harbor-bin` | `out/eval/danmo-work-cli`（linux/`EVAL_GOARCH`） |
| `make eval-harbor-smoke` | sync + base + bin → 1 题 oracle + DanQing |
| `make eval-harbor-suite` | sync + bin → 全量 oracle 再 DanQing |
| `make eval-harbor-compare` | 对单一对比 agent 跑全套（默认 `opencode`） |

---

## 同步细节（89 题从哪来）

不能直接 `harbor run --dataset terminal-bench@2.0`：官方题自带 `FROM`，且 `task.toml` 常有 `docker_image`，Harbor 会拉上游预构建镜像，用不上我们的 base。

`sync_tb2_tasks.sh` 会：

1. 拉取 TB2（优先：`TB2_SRC_DIR` → `git clone` → `harbor datasets download` → GitHub API）
2. `environment/Dockerfile` 首行改为 `FROM dq-harbor-base:local`
3. 删除 `task.toml` 里的 `docker_image = ...`
4. 写入 `evals/dq_harbor/tasks/<name>/`

常用：

```bash
# 本地已有 clone
TB2_SRC_DIR=/path/to/terminal-bench-2 ./evals/dq_harbor/sync_tb2_tasks.sh

# git 不通时强制 API（可断点续传）
GH_TOKEN=$(gh auth token) python3 evals/dq_harbor/sync_tb2_via_api.py
```

OpenCode 用 [`OpenCodePrebuilt`](agent_opencode.py)：镜像里已有 opencode 则跳过 nvm/npm。

---

## 结果与排错

- 对比日志：`evals/dq_harbor/compare_results/<timestamp>/`（gitignore）
- 成绩摘要：[`COMPARE_RESULTS.md`](COMPARE_RESULTS.md)
- 失败分析：`python3 evals/dq_harbor/analyze_failures.py --failed-only`
- Harbor job：仓库下 `jobs/` 或 `harbor view jobs`

| 现象 | 处理 |
|------|------|
| `no TB2 tasks under …/tasks` | `make eval-harbor-sync-tb2` |
| `missing OpenCode` / 每题装 nvm | `make eval-harbor-base`，agent 用 `opencode`（Prebuilt） |
| 仍在拉 `alexgshaw/...` 镜像 | 题未清 `docker_image`，`FORCE_RESYNC=1` 再 sync |
| `exec format error` | `make eval-harbor-bin EVAL_GOARCH=amd64`（或与 Podman VM 一致） |
| sync 中途失败 | 再跑 API sync（默认续传已下完的题） |

## Out of scope

- 官方榜单 / ATIF 导出

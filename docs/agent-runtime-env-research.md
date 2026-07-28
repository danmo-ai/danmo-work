# 轻量容器 / 标准化运行环境调研

> 目标：对照开源 AI Agent 与产业实践，厘清 **进程隔离（sandbox）** 与 **标准化运行环境（runtime env）** 的边界，给出 Danmo Work 可落地的分层方案。  
> 范围：调研与架构建议；本文件不包含实现代码。  
> 现状锚点：`core/runtime/sandbox/`（Seatbelt / Landlock / bwrap / win-token）、Harbor 评测镜像 `evals/dq_harbor/images/base/Dockerfile`。

---

## 1. 结论（先看这个）

开源 Agent 在「运行系统环境」上已收敛为 **两层正交问题**：

| 层 | 要解决的问题 | 主流做法 | Danmo 现状 |
|----|--------------|----------|------------|
| **A. 隔离 / 安全** | Agent 命令不能乱写盘、乱出网 | OS 原语（bwrap / Landlock / Seatbelt）或容器 / microVM | ✅ 本地进程 sandbox 已落地，策略对齐 Codex |
| **B. 标准化环境** | 工具链、依赖、OS 可复现；云端 / 评测 / 多机一致 | OCI 镜像 + 声明式 env spec（Dockerfile / environment.json）+ Workspace 抽象 | ⚠️ 仅 Harbor 评测有镜像；产品路径仍跑宿主工具链 |

**建议方向（按威胁模型分层，不追最重方案）：**

1. **本地桌面 / CLI（默认）**：继续强化现有 OS sandbox；补齐网络 allowlist proxy（对标 Anthropic `sandbox-runtime` / Codex managed-proxy）。
2. **标准化环境（下一优先）**：引入 **Workspace / ExecutionBackend 抽象**（Local | Docker/Podman | Remote），用 OCI 镜像 + 项目级 env 声明保证「能跑、能复现」。
3. **多租户 / 云托管（远期）**：再上 gVisor 或 Firecracker（E2B 等）；本地产品路径不必为 microVM 付运维税。

一句话：**本地用轻量 OS 沙箱管安全；跨机 / 评测 / 云端用轻量 OCI 容器管环境一致性；两者通过同一 `Sandbox`/`Workspace` 接口切换。**

权限 Soft/Hard 分工与网络三态见 **[permission-model.md](./permission-model.md)**。

---

## 2. 问题拆解：别把两件事揉成一件

### 2.1 隔离（sandbox）≠ 标准化环境（runtime env）

| | 隔离 | 标准化环境 |
|--|------|------------|
| 用户感知 | 「Agent 会不会删我 `~` / 偷密钥」 | 「Agent 能不能 `go test` / `npm run build`」 |
| 失败形态 | 越权读写、外泄、逃逸 | 缺 Node、缺编译器、路径/版本漂移 |
| 典型手段 | Seatbelt、Landlock、bwrap、seccomp、gVisor、Firecracker | Dockerfile、devcontainer、snapshot、预装镜像、`install` 脚本 |
| 是否需要 daemon | OS 原语通常不需要 | Docker/Podman/containerd 需要 |

许多产品文档把两者都叫 sandbox / runtime，但实现上应分开：

- **隔离策略**：`mode` / `network` / 路径 allowlist（Danmo 已有）。
- **环境规格**：base image、`install`、工具链、密钥注入、egress allowlist（Danmo 产品路径尚未建模）。

### 2.2 威胁模型决定技术档位

参考 2026 年业界选型共识（Claude Code / Codex / E2B / Anthropic web）：

| 威胁模型 | 合适档位 | 代表 |
|----------|----------|------|
| 开发者本机；blast radius = 用户自己 | OS 原语（bwrap / Landlock / Seatbelt） | Claude Code、Codex CLI、**Danmo 现状** |
| 共享基础设施、半可信租户 | gVisor（用户态内核） | Claude web、Modal sandbox |
| 真多租户 SaaS、不可信代码 | Firecracker microVM | E2B、Vercel Sandbox、多数托管 code-exec |
| 评测 / 可复现基准 | OCI 容器 + 固定镜像 | SWE-bench / Harbor / mini-SWE-agent |

**不要**在桌面端默认上 microVM：冷启动、镜像、KVM 依赖对桌面产品过重，而威胁模型并不需要租户级内核隔离。

---

## 3. 开源 / 产业方案对照

### 3.1 总览矩阵

| 项目 | 环境抽象 | 隔离层 | 标准化手段 | 对 Danmo 的启发 |
|------|----------|--------|------------|-----------------|
| **OpenAI Codex CLI** | 本机进程 | Landlock + seccomp（默认）；bwrap 回退；网络 unshare / managed-proxy | 宿主工具链 | 策略命名与 backend 探测（Danmo 已对齐） |
| **Anthropic Claude Code** | 本机 Bash tool | macOS Seatbelt；Linux bwrap + 可选 seccomp；外置 HTTP/SOCKS proxy 做域名 allowlist | `@anthropic-ai/sandbox-runtime`（Apache-2.0）开源 | **网络 allowlist 应走代理，而非仅 `--unshare-net`** |
| **Anthropic Claude web** | 云端会话 | gVisor | 托管镜像 | 多租户时再考虑 |
| **OpenHands V1** | `Workspace`：Local / Docker / Remote API | Docker 容器（推荐）；Process（快但不安全）；Remote | Agent Server 镜像（含 VSCode/VNC/Browser）；同一 SDK 换 workspace | **接口稳定、后端可插拔**是关键架构 |
| **mini-SWE-agent / SWE-ReX** | `environment_class`：local / docker / singularity / bwrap / modal… | 容器或进程 | `docker exec` + 镜像配置；动作无状态（每次独立 exec） | 无状态 exec 极易换后端；HPC 用 Singularity |
| **Cursor Cloud Agent** | VM + `.cursor/environment.json` | 托管 VM + egress 策略 | Dockerfile **或** snapshot；`install` 脚本；密钥单独注入 | **声明式 env spec + 可验证 install** 值得抄 |
| **E2B** | SDK `Sandbox` | Firecracker microVM | 自定义模板镜像；pause/resume | 云托管 / 不可信代码时的标准答案 |
| **Daytona 等** | Workspace API | 默认 Docker（可选 Kata） | 持久工作区 | 长会话状态 vs 短生命周期 code-exec 的产品分叉 |
| **Harbor / Terminal-Bench**（Danmo 已用） | 每题容器 | Podman/Docker | `dq-harbor-base:local` 预装工具链 | 评测路径已证明「镜像标准化」有效 |

### 3.2 深度摘录：值得抄的设计点

#### A. OpenHands：Workspace 三态

```
Client / Agent 逻辑
        │  同一接口（file / shell / git）
        ▼
   BaseWorkspace
   ├── LocalWorkspace      → 本机（快，无隔离）
   ├── DockerWorkspace     → 容器内 Agent Server
   └── RemoteAPIWorkspace  → 托管 / K8s
```

要点：Agent 代码不感知后端；切换只改 workspace 构造参数。V1 默认可本地进程，容器为 opt-in——与「桌面默认本机、云端强制隔离」一致。

#### B. mini-SWE-agent：无状态 `execute` + environment_class

- 每次动作 ≈ `subprocess` / `docker exec`，不依赖长生命周期 shell session。
- 换 Docker / Singularity / bubblewrap 只需换 Environment 类。
- 配置面：`image`、`cwd`、`env`、`forward_env`、`timeout`、`run_args`。

对 Danmo：`exec_shell` 已是「命令字符串 + WorkDir + Timeout」模型，天然适合再挂一层 Docker backend，而无需先做 PTY 会话抽象。

#### C. Anthropic sandbox-runtime：容器外的网络策略

- FS：Seatbelt / bwrap。
- Network：沙箱内断网或受限，**出站经宿主机侧 proxy 做域名 allowlist**。
- 可沙箱任意进程（含 MCP server），不只是 bash。

Danmo 现状：`sandboxNetwork=deny|allow|allowlist`；deny ≈ `bwrap --unshare-net` / Seatbelt `(deny network*)`；**allowlist 已落地**为宿主机 loopback HTTP CONNECT 代理（`core/runtime/sandbox/netproxy`）+ `allowlist_domains`，经 `HTTP(S)_PROXY` / `ALL_PROXY` 注入。

#### D. Cursor environment.json：标准化环境的最小声明

概念模型（与产品细节无关的可复用部分）：

```json
{
  "build": { "dockerfile": "Dockerfile", "context": ".." },
  "install": "npm ci && go mod download",
  "terminals": []
}
```

原则：

1. **镜像管系统依赖**（编译器、git、运行时版本）。
2. **`install` 管项目依赖**（可版本化、可重跑）。
3. **不要把整仓 COPY 进镜像**；工作区由 runtime checkout / mount。
4. **密钥不进镜像**；运行时注入。
5. **setup 以一条验证命令收尾**（`make test` / `go test ./...`）。

#### E. E2B / Firecracker：何时才需要

- 冷启动约 100–200ms，每租户独立内核。
- 适合：托管 code interpreter、多租户 Agent SaaS。
- **不适合**作为 Danmo 桌面默认路径（依赖 KVM、镜像管线、运维面过大）。

---

## 4. 技术档位速查

| 档位 | 代表技术 | 启动 | Daemon | 隔离强度 | 环境可复现 | 适用 |
|------|----------|------|--------|----------|------------|------|
| L0 Host | 裸 `exec` | ~0 | 无 | 无 | 否 | danger-full-access / 调试 |
| L1 OS sandbox | bwrap / Landlock / Seatbelt / win-token | ms | 无 | 中（共享内核） | 否（用宿主工具链） | **本地默认** |
| L2 OCI 容器 | Docker / Podman / containerd | 百 ms～数 s（镜像主导） | 需要 | 中（可加 seccomp/user ns） | **强** | 标准化环境、评测、可选本地「容器模式」 |
| L3 gVisor | runsc | 亚秒～秒 | 需要 | 较强 | 强（仍基于容器镜像） | 共享 infra |
| L4 microVM | Firecracker / Kata | ~100–200ms | 需要（VMM） | 最强 | 强 | 多租户云 |

「轻量」在社区里常被混用：

- **轻量隔离** → L1（无 daemon，Danmo 已选）。
- **轻量容器** → L2 中的 rootless Podman / 精简 OCI，强调可复现而非最强安全。
- **轻量 VM** → L4 Firecracker，相对传统 VM 轻，相对 L1 仍重。

Danmo 产品叙事建议：**本地轻量沙箱 + 可选轻量容器环境**，避免把 microVM 写进近期路线图。

---

## 5. 与 Danmo Work 的差距

### 5.1 已具备

- 跨平台 sandbox manager：`core/runtime/sandbox/`
  - macOS Seatbelt、Linux Landlock / bwrap、Windows token / WSL2 探测
  - 模式：`read-only` / `workspace-write` / `danger-full-access`（Codex 命名）
  - `exec_shell` 经 `port.Sandbox` 执行；配置可热更新；`/sandbox/status` 可观测
- Harbor 评测：Podman + 预构建 `dq-harbor-base:local`（Node/OpenCode/Python）——证明「镜像标准化」在仓库内已有先例

### 5.2 缺口

| 缺口 | 说明 |
|------|------|
| **无 ExecutionBackend / Workspace 抽象** | `Sandbox` 只管「怎么隔离地跑一条命令」，不管「在哪套文件系统/工具链里跑」 |
| **无项目级环境声明** | 缺少类似 `environment.json` / `devcontainer.json` / `runtime.env.yaml` 的一等配置 |
| **网络 allowlist（Phase 1 已完成）** | `netproxy` + `allowlistDomains` + Settings UI；已知限制：忽略代理 env 的客户端可绕过 |
| **桌面用户环境漂移** | Agent 依赖用户机器上的 Node/Go/Python；换机即碎 |
| **容器 backend 未接入产品路径** | Harbor 镜像仅服务 eval，未作为 Session/Project 运行时选项 |
| **远程 / 多租户** | 无 Remote workspace；若未来做云端 Agent，需 L3/L4 |

---

## 6. 推荐架构（目标态）

```
TurnRunner / Tools (exec_shell, write, …)
        │
        ▼
  ExecutionBackend  (port)
  ├── LocalOS          → 现有 sandbox.Manager（L1）
  ├── ContainerOCI     → Podman/Docker：镜像 + bind workspace（L2）
  └── RemoteAPI        → 未来：E2B / 自建 Agent Server（L3/L4）
        │
        ├── IsolationPolicy   (mode, network, path allowlist)
        └── EnvironmentSpec   (image | dockerfile, install, env, verify)
```

### 6.1 EnvironmentSpec（建议最小字段）

```yaml
# 例：项目或全局 ~/.danmo-work/environments/default.yaml
apiVersion: danmo.work/v1
kind: EnvironmentSpec
metadata:
  name: default
spec:
  backend: local | container | remote   # local 忽略 image
  container:
    # 本地标签；镜像来自 CI 内置 tar（load），禁止 registry pull
    image: "localhost/danmo-work-env:bundled"
    tarPath: ""  # 空则自动发现 WORK_ENV_TAR / ~/.danmo-work/env / out/env
    workspaceMount: /workspace
    network: deny | allow | allowlist
    allowlistDomains: ["pypi.org", "proxy.golang.org", "registry.npmjs.org"]
  install: |
    go mod download
  verify: |
    go test ./...
  forwardEnv: ["HTTP_PROXY", "HTTPS_PROXY"]
```

原则对齐 Cursor / OpenHands：

- 镜像不 COPY 业务源码；workspace bind/mount。
- `install` / `verify` 可缓存（hash 项目锁文件）。
- 密钥走配置 / OS keychain，不进层。

### 6.2 与现有 Sandbox 的关系

- **LocalOS**：`EnvironmentSpec.backend=local` 时，继续用现有 `sandbox.Manager`；EnvironmentSpec 仅表达「期望工具」，可用 `verify` 做软检查（缺 Node 则提示），不强行容器化。
- **ContainerOCI**：命令在容器内执行；容器外仍可叠加 rootless + read-only rootfs；网络策略在容器 network namespace / 代理上实现。
- **策略复用**：`SandboxMode` / `SandboxNetwork` 枚举保持不变，避免前端与配置分裂。

---

## 7. 分阶段落地建议

### Phase 0 — 文档与接口（本调研）

- 固定术语：Sandbox（隔离）vs Environment（工具链/镜像）vs Backend（执行位置）。
- 明确桌面默认保持 L1。

### Phase 1 — 本地安全补齐（已完成）

1. ~~实现 `network=allowlist`：宿主机侧 HTTP CONNECT proxy + 域名列表。~~ → `core/runtime/sandbox/netproxy` + `Manager` 注入。
2. ~~扩展 sandbox 状态：`allowlistActive` / `allowlistProxy` / `allowlistDomains` + degraded reason。~~
3. Go 内生实现（未嵌入 `@anthropic-ai/sandbox-runtime`）；未做 SOCKS5 / 内核层强制代理。

### Phase 2 — Container backend MVP（标准化环境 / 进行中）

**镜像分发：不走 registry pull。** CI（`make build-env-tar`）构建轻量 agent 友好镜像并 `podman/docker save` 为 `out/env/danmo-work-env-linux-<arch>.tar`；随 Linux server 包与 Release 资产分发。运行时仅 `load -i`，标签 `localhost/danmo-work-env:bundled`。

1. ~~新增 `port.ExecutionBackend`~~ → `core/runtime/execution` + `core/adapter/container`
2. Podman 优先探测，Docker 兼容；**禁止 pull**
3. `runtime.environment.backend=local|container`；一项目一长生命周期容器（`danmo-work-proj-{id}`），bind-mount workdir → `/workspace`
4. `exec_shell` 经 ExecutionBackend；缺引擎/缺 tar 时 degraded → LocalOS
5. `GET /api/v1/environment/status`；Settings UI 后续补

验收：安装包内带 env tar + 本机 Podman/Docker 时，`backend=container` 下 `node -v` / `python3 -v` 不依赖宿主工具链。

### Phase 3 — 声明式 EnvironmentSpec

1. 支持 `.danmo-work/environment.yaml`（或兼容读取 `.cursor/environment.json` / `.devcontainer/devcontainer.json` 的子集）。
2. `install` + `verify` 生命周期；锁文件 hash 缓存。
3. 与 Project 绑定；切换项目自动切换 env。

### Phase 4 — Remote / 多租户（按需）

1. OpenHands 风格 Agent Server，或对接 E2B SDK。
2. 仅当出现「云端托管 Agent / 多租户」需求时启动；桌面版可不打包。

---

## 8. 方案比选（针对 Phase 2）

| 方案 | 优点 | 缺点 | 建议 |
|------|------|------|------|
| **A. 仅加强 L1 sandbox** | 已实现、无 daemon、桌面体验好 | 不解决工具链漂移与评测/云一致性 | 必要但不充分 |
| **B. 本机 L1 + 可选 OCI（推荐）** | 与 OpenHands/SWE-agent 同构；复用 Harbor 经验；用户可选 | 需 Podman/Docker；镜像维护 | **主推** |
| **C. 默认全部进容器** | 最大一致性 | 桌面摩擦大、体积与权限问题 | 不作为默认 |
| **D. 直接上 E2B/Firecracker** | 隔离最强 | 过重、依赖云或 KVM、与当前桌面定位不符 | 留作 Remote backend |
| **E. 只依赖 devcontainer CLI** | 生态现成 | 与 Go runtime 集成松；Windows/桌面路径复杂 | 可作为 EnvironmentSpec 的导入源，不作唯一后端 |

---

## 9. 风险与非目标

**风险**

- 容器内网络与「本机已登录的 gh/npm」凭证割裂 → 需明确 forwardEnv / 密钥注入模型。
- macOS 桌面 Docker/Podman 机依赖重 → Phase 2 应允许 degraded 回退 LocalOS，并在 UI 标明。
- 镜像膨胀 → 产品镜像保持「语言 runtime + 常用 CLI」，项目依赖走 `install`，勿做成巨型 kitchen-sink。
- 把「标准化环境」误做成「更强沙箱」而默认强制容器 → 伤害本地 UX。

**非目标（本阶段不做）**

- 自研 microVM 控制面
- 完整兼容 Docker Compose 多服务编排（除非项目 env 明确需要）
- 在 L1 sandbox 内再嵌一套完整容器运行时（DinD）作为桌面默认
- 用 WASM 替代通用 shell 环境（语言覆盖不足）

---

## 10. 参考链接

| 主题 | 链接 |
|------|------|
| OpenHands sandboxes overview | https://docs.openhands.dev/openhands/usage/sandboxes/overview |
| OpenHands runtime 架构 | https://docs.openhands.dev/openhands/usage/architecture/runtime |
| OpenHands Agent SDK 论文 | https://arxiv.org/abs/2511.03690 |
| mini-SWE-agent environments | https://mini-swe-agent.com/latest/reference/environments/docker/ |
| Anthropic sandbox-runtime | https://github.com/anthropic-experimental/sandbox-runtime |
| Claude Code sandboxing | https://www.anthropic.com/engineering/claude-code-sandboxing |
| Codex / Claude sandbox 对比 | https://instavm.io/blog/how-claude-code-and-codex-approach-sandboxing |
| 沙箱档位选型（2026） | https://tanayshah.dev/blog/choosing-agent-sandbox-2026/ |
| Agent sandbox landscape | https://katosh.github.io/agent_sandbox/reference/landscape-comparison/ |
| E2B | https://github.com/e2b-dev/e2b |
| Cursor environment setup | https://cursor.com/docs/cloud-agent/setup |
| Danmo Harbor 评测 | `evals/dq_harbor/README.md` |
| Danmo 现有 sandbox | `core/runtime/sandbox/`、`core/domain/sandbox.go` |

---

## 11. 下一步（若立项实现）

1. 开设计稿：`port.ExecutionBackend` + `EnvironmentSpec` 字段冻结（小 PR）。
2. Phase 1：`allowlist` 网络代理（可独立交付）。
3. Phase 2：Podman backend MVP + 产品基础镜像（从 `dq-harbor-base` 裁剪）。
4. 前端 Settings：backend 选择与 degraded 提示。
5. 文档：用户侧「本机沙箱 vs 容器环境」说明。

# Danmo Work 权限模型

> Soft Gate（审批）与 Hard Enforcement（沙箱 / 出站策略）正交。  
> 对照：OpenWorker（risk × mode）、Claude Code（Permissions vs Sandboxing）、Codex（sandbox-first）。

---

## 1. 两层模型

| 层 | 职责 | 实现 | 失败形态 |
|----|------|------|----------|
| **Soft Gate** | 是否允许 Agent *尝试* | `permission.Gate` + 审批 UI | 误点 / 疲劳 |
| **Hard Enforcement** | 尝试后 *实际能碰什么* | FS sandbox **或** OCI 容器 + net deny/allowlist proxy + 主机出站检查 | 逃逸 / 忽略 PROXY |

原则：

1. 强隔离内安全 `exec_shell` Soft=allow（少问）。隔离源见 `EffectiveIsolation`（OS sandbox **或** 健康 OCI container）。
2. 出站用 Hard 收口（`deny` / `allowlist`）；allowlist 不是审批 Reason；策略助手在 `core/runtime/egress`。
3. `runtime.auto_approve` 不抬高能力上限：不跳过 `dangerous_command` / `unsandboxed`。
4. 域名授予：`once` → turn 级 Hard；`session` → 会话 Soft + Hard；会话结束可 `RevokeSessionDomains`。

## 2. 风险面（OpenWorker 映射）

| OpenWorker | Danmo Risk | 工具 | interactive Soft |
|------------|------------|------|------------------|
| read | low | `read_file`, `web_fetch`* | allow（*仍受出站 Hard） |
| write_local | medium | `write` / `edit` / `apply_patch` | allow |
| exec | high | `exec_shell` | 强沙箱 allow；危险 Ask；弱沙箱 Ask |
| external | external | `mcp_*`, 写 HTTP | Ask |

## 3. 网络三态

| network | Soft | Hard（shell + 主机 HTTP） |
|---------|------|---------------------------|
| `deny` | 出站意图 → Ask `network`；session 可全开 | OS 断网 / 主机拒绝出站 |
| `allowlist` | 未知域名 → Ask `network_domain`（加入名单） | 正向代理 + 主机 `Match`；**禁止**用全开代替 |
| `allow` | 不出站 Ask | 仅 SSRF 防护 |

`SessionAllowNetwork`（全开）**仅**服务 `deny`。allowlist 用域名会话授予。

## 4. 必要优化状态

| ID | 项 | 状态 |
|----|----|------|
| M1 | Soft/Hard 文档 | 本文档 |
| M2 | deny 全开语义隔离 | 文案 + Gate |
| M3 | 域名会话授予 | `network_domain` + `GrantSessionDomains` / `GrantTurnDomains` |
| M4 | 主机出站合流 | web/http 工具走同一 allowlist |
| S1 | PermissionRule 接线 | `runtime.permission_rules` |
| S2 | MCP stdio 网络隔离 | inherit 沙箱网络 + proxy/unshare |
| S3 | allowlist 推荐与预设 | Settings UI：快速组合（开发依赖 / 网络搜索）+ 单生态（debian/npm/go/pypi/crates/github），可叠加追加 |
| S4 | auto_approve 天花板 | 不跳过 dangerous/unsandboxed |
| S5 | EffectiveIsolation | Gate 感知 OCI container |
| S6 | EgressPolicy 统一 | `core/runtime/egress`（proxy env / 网络映射 / CheckHost） |
| S7 | 容器生命周期 | 项目删除 Teardown + `Core.Close` |

## 5. 相关代码

- Soft：`core/runtime/permission/gate.go`（`EffectiveIsolation`）
- Hard FS/Net：`core/runtime/sandbox/`
- 共享出站：`core/runtime/egress/`
- 执行路由：`core/runtime/execution/`（`EnvironmentInspector` + `CommandRunner`）
- 主机出站：`core/runtime/tool/builtin/web_http.go`
- MCP spawn：`core/adapter/mcp/client.go`
- 审批：`core/runtime/turn_runner.go` `gateToolCall`

交叉：`docs/agent-runtime-env-research.md`（Hard 环境层）、`docs/core-design.md` §8。

可选 OCI（`runtime.environment.backend=container`）：镜像来自 CI 旁路 tar（`load`，不 pull）；每项目一容器。Soft Gate 通过 `EffectiveIsolation` 将健康容器视为强隔离；Hard 边界为容器 FS（项目目录 **同路径 bind**）+ 容器 network（deny=`none`；allowlist 用 host/default 网 + 共用 host allowlist proxy）。

# Danmo Work 权限模型

> Soft Gate（审批）与 Hard Enforcement（沙箱 / 出站策略）正交。  
> 对照：OpenWorker（risk × mode）、Claude Code（Permissions vs Sandboxing）、Codex（sandbox-first）。

---

## 1. 两层模型

| 层 | 职责 | 实现 | 失败形态 |
|----|------|------|----------|
| **Soft Gate** | 是否允许 Agent *尝试* | `permission.Gate` + 审批 UI | 误点 / 疲劳 |
| **Hard Enforcement** | 尝试后 *实际能碰什么* | FS sandbox + net deny/allowlist proxy + 主机出站检查 | 逃逸 / 忽略 PROXY |

原则：

1. 强沙箱内安全 `exec_shell` Soft=allow（少问）。
2. 出站用 Hard 收口（`deny` / `allowlist`）；allowlist 不是审批 Reason。
3. `runtime.auto_approve` 不抬高能力上限：不跳过 `dangerous_command` / `unsandboxed`。

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
| M3 | 域名会话授予 | `network_domain` + `GrantDomains` |
| M4 | 主机出站合流 | web/http 工具走同一 allowlist |
| S1 | PermissionRule 接线 | `runtime.permission_rules` |
| S2 | MCP stdio 网络隔离 | inherit 沙箱网络 + proxy/unshare |
| S3 | allowlist 推荐与预设 | Settings UI |
| S4 | auto_approve 天花板 | 不跳过 dangerous/unsandboxed |

## 5. 相关代码

- Soft：`core/runtime/permission/gate.go`
- Hard FS/Net：`core/runtime/sandbox/`
- 主机出站：`core/runtime/tool/builtin/web_http.go`
- MCP spawn：`core/adapter/mcp/client.go`
- 审批：`core/runtime/turn_runner.go` `gateToolCall`

交叉：`docs/agent-runtime-env-research.md`（Hard 环境层）、`docs/core-design.md` §8。

可选 OCI（`runtime.environment.backend=container`）：镜像来自 CI 内置 tar（`load`，不 pull）；每项目一容器。Soft Gate 不变；Hard 边界变为容器 FS（bind `/workspace`）+ 容器 network（deny=`none`，allowlist 暂用 `host` 以达 loopback 代理）。

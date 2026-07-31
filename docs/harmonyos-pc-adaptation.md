# HarmonyOS PC（鸿蒙桌面）适配评估

> 状态：评估文档（非实施计划承诺）  
> 日期：2026-07-31  
> 范围：在现有 Danmo Work「Tauri 薄壳 + Go sidecar」架构上，评估如何适配 HarmonyOS NEXT / 鸿蒙 PC（2in1）桌面版。

## 结论

**不能直接移植现有 Tauri 桌面包。** Tauri 2 仅覆盖 Win / macOS / Linux；当前打包产物（DMG / AppImage / deb / NSIS）与鸿蒙 HAP / AppGallery 体系不兼容。

**可复用面很大。** 产品核心是「本地 Go HTTP 引擎 + Vue 前端」，前端几乎不依赖原生桥（仅 dialog / updater / 写文件）。推荐路径是：

> **新建鸿蒙壳（ArkTS + ArkWeb）+ 复用 Go 核心（ohos_golang_go 编译为可执行文件或 `c-shared` .so）+ 按能力降级 agent 工具链。**

不建议第一期做完整 ArkUI 原生重写，也不建议押注「Linux 兼容层跑现有 AppImage」。

---

## 1. 现状架构（与鸿蒙相关的切面）

```
┌─────────────────────────────────────────────┐
│  Tauri 2 webview（Win / macOS / Linux）     │
│  Vue 3 dist  ──HTTP──► 127.0.0.1:7801       │
│  dialog / updater / write_file_bytes        │
└──────────────────┬──────────────────────────┘
                   │ spawn + lifecycle
                   ▼
┌─────────────────────────────────────────────┐
│  Go server sidecar（~/.danmo-work/bin/…）    │
│  SQLite work.db + store.db（pure Go）       │
│  exec_shell + OS sandbox + PTY + tools      │
└─────────────────────────────────────────────┘
```

| 层 | 技术 | 鸿蒙可复用性 |
|----|------|-------------|
| UI | Vue 3 + Vite → `out/frontend/dist` | **高**（ArkWeb 加载；需替换 Tauri 插件调用） |
| 壳 | Tauri 2.11（`desktop/src-tauri`） | **无**（需 ArkTS UIAbility 重写） |
| 引擎 | Go `server/` + `core/`，监听 `127.0.0.1:7801` | **中高**（需 ohos Go 工具链；部分 OS 特化代码要改） |
| 持久化 | `glebarez/sqlite` / `modernc.org/sqlite`（无 CGO） | **高** |
| 数据目录 | `~/.danmo-work/` | **中**（映射到应用沙箱 files 目录） |
| 分发 | GitHub Releases + Homebrew + `releases.danmo.ai` 镜像 | **部分可复用镜像思路**；上架需 AppGallery |

前端原生耦合点（均在 `frontend/src/`）：

- `utils/desktop.ts`：`isTauriRuntime`、`apiBaseUrl`、`waitForBackend`、`saveBlobAs`（dialog + `write_file_bytes`）
- `composables/useAppUpdater.ts`：Tauri updater
- `components/left/LeftRail.vue`：打开文件夹 dialog
- `views/SettingsView.vue`：桌面特性门控

其余业务走 `/api/v1/*` HTTP，与壳无关。

---

## 2. 硬阻塞与半阻塞

### 硬阻塞（不做新壳则无法上架）

1. **Tauri / 现有 packer 无鸿蒙目标**  
   `scripts/build_sidecar.sh` 仅识别 darwin/linux/windows；`pack_desktop_{macos,linux,windows}.sh` 产物不可用于 HAP。
2. **官方 Go 无 `GOOS=harmony`**  
   需 OpenHarmony-SIG 的 [`ohos_golang_go`](https://gitcode.com/openharmony-sig/ohos_golang_go)（`GOOS=openharmony GOARCH=arm64`）。基线与仓库 `go 1.26` 可能不对齐，要锁定可编译版本。
3. **进程模型与沙箱**  
   当前壳用 PID 文件、`lsof`/`netstat`、Unix process group / Windows `taskkill` 管 sidecar。鸿蒙应用进程与权限模型不同，需 Ability 生命周期管理 + 应用沙箱路径，不能照搬「旁路外部 Bin」。
4. **分发与更新链**  
   Tauri minisign updater、NSIS、DMG、Homebrew 均不适用；需 HAP 签名、AppGallery（或企业分发）及鸿蒙更新通道。

### 半阻塞（MVP 可降级，完整 Agent 体验要补）

| 能力 | 现状 | 鸿蒙风险 |
|------|------|----------|
| `exec_shell` + sandbox | Seatbelt / bwrap+Landlock / Win token | 无对应后端；需新 `sandbox_openharmony.go` 或降级为受限子进程 |
| 项目终端 PTY | `creack/pty` + bash/cmd | 可能无 POSIX PTY；终端 Tab 可能不可用 |
| Windows Coreutils | 捆绑 Microsoft Coreutils | 需确认设备是否自带 POSIX 工具 / 是否要自带 busybox 类工具 |
| `chromedp` 渲染抓取 | 依赖本机 Chromium | 设备上可能无 Chromium；需禁用或改用系统 Web 能力 |
| Git CLI | `ProjectManager` shell-out | 需设备安装 git，或改为纯 Go git 库 / 远程执行 |
| 可选 OCI agent 环境 | Podman/Docker/Apple Container | 鸿蒙 PC 容器生态未对齐；一期不做 |
| 本地端口 `7801` | 固定 loopback | 应用内绑定通常可行；需确认权限与多实例策略 |

---

## 3. 可选适配路线

### 路线 A（推荐）：ArkWeb 薄壳 + Go 引擎（镜像现有架构）

```
┌──────────────────────────────────────────────┐
│  HarmonyOS PC HAP                            │
│  UIAbility + ArkWeb 加载 Vue dist            │
│  filePicker / 写文件 / 更新 → ArkTS bridge   │
└──────────────────┬───────────────────────────┘
                   │ 启动 / 生命周期
                   ▼
┌──────────────────────────────────────────────┐
│  Go HTTP 引擎（ohos 可执行文件或内嵌服务）   │
│  127.0.0.1:<port> 或 Unix domain / 本地 IPC  │
│  SQLite + 业务 API 复用                      │
└──────────────────────────────────────────────┘
```

- **优点**：最大化复用 `core/` + `frontend/`；与现有桌面心智一致；国内已有类似案例（Tauri 去壳 → Rust/Go 编 `.so` + ArkTS，如社区对学习助手类应用的 OH 移植）。
- **缺点**：要维护鸿蒙专用壳与 CI；Go ohos 工具链非官方主线，升级有摩擦。
- **Go 嵌入形态**：
  1. **可执行 sidecar / HNP native bin**：最接近现状，Ability `onCreate` 拉起、退出时回收。
  2. **`c-shared` → Node-API**：进程内嵌，生命周期更干净，但要把「长期跑 HTTP server」嵌进 Ability 线程模型，调试成本更高。

**建议先 Spike 形态 1**；若进程拉起被商店策略限制，再切形态 2。

### 路线 B：Go 核心编为 `.so`，ArkTS 主导 UI

把关键能力通过 NAPI 暴露，UI 逐步 ArkUI 化。适合长期「原生体验」，一期成本明显高于 A，且要重做大量 Vue 页面。

### 路线 C：无本地引擎 / 远程后端

壳只开 Web；会话跑在远端服务器。可快速有「能打开的鸿蒙客户端」，但失去本地 SQLite、本地 shell/agent、隐私本地优先等产品卖点，**不作为桌面对等产品**。

### 路线 D：Linux 兼容跑现有包

依赖未承诺的兼容层，且当前 Linux 包是 **glibc x86_64 AppImage/deb**，与鸿蒙 NEXT 主流 **aarch64-ohos** 不对齐。**不推荐作为产品路径。**

---

## 4. 推荐分期

### Phase 0 — 可行性 Spike（决定是否立项）

目标：在鸿蒙 PC / 2in1 真机或模拟器上跑通「Go 监听 + 一次 `/api/v1/version`」。

1. 用 `ohos_golang_go` 交叉编译最小 Gin/`net/http` 服务（`GOOS=openharmony GOARCH=arm64`）。
2. 验证纯 Go SQLite 在应用沙箱路径读写。
3. 用 ArkWeb 加载静态页并 `fetch` 本地 API（CORS / 自定义协议 / 明文 localhost 策略一并验证）。
4. 记录：可执行文件 vs `.so`、端口绑定、后台保活、杀进程后重启行为。

**退出标准**：version API 稳定；SQLite 读写成功；ArkWeb 可调通。任一失败则重估路线 B/C。

### Phase 1 — 产品 MVP（聊天 / 会话 / 配置）

- 新建仓库目录建议：`harmony/`（或 `desktop-ohos/`）— DevEco 工程、Ability、桥接。
- 前端抽象壳适配层（避免继续散落 `isTauriRuntime`）：
  - `isDesktopRuntime()` / `DesktopBridge`：dialog、save file、open folder、updater、backend ready。
  - Tauri 与 Harmony 各一实现；Web 保持现状。
- Go：`WORK_*` 环境变量指向应用 files 目录；新增 `openharmony` build tag 的路径/进程辅助。
- **明确降级**：无 PTY 终端、无 OS sandbox、无 chromedp、无 OCI env、无自动 updater（或仅跳转 AppGallery）。
- 打包：DevEco 出 HAP；CI 可先手工，后加 Linux runner + ohos SDK。

### Phase 2 — Agent 工具链对齐

按优先级补：

1. 受限 `exec_shell`（应用可见目录 + 白名单命令）  
2. Git（系统 git 或 go-git）  
3. 文件类工具与项目工作区权限  
4. 知识库 / skills 目录扫描路径  
5. 若系统提供终端/PTY 能力再开终端 Tab  
6. 渲染抓取：无 Chromium 则保持 HTTP-only `web_fetch`

### Phase 3 — 分发与体验

- 签名、隐私声明、权限清单（网络、文件、后台）  
- AppGallery 上架；国内已有 `releases.danmo.ai` 镜像经验可作补充下载渠道，但不能替代商店更新  
- 窗口：自定义标题栏、2in1 触控/键鼠、深浅色跟随  
- 与 macOS/Windows/Linux 版本号与 API 兼容策略对齐

---

## 5. 代码层改动清单（落地时）

### 建议先做的结构性改动（也利于长期多壳）

| 改动 | 说明 |
|------|------|
| 前端 `DesktopBridge` | 收敛 Tauri 专用调用，鸿蒙只实现同一接口 |
| Go `paths` | 支持显式 `WORK_HOME` / 沙箱 root，避免写死 `~/` 语义 |
| sandbox 接口 | 已有多 OS 文件；增加 `openharmony` 实现或 `nop`+策略拒绝 |
| sidecar 构建脚本 | `build_sidecar.sh` 增加 ohos 目标（依赖 ohos Go + SDK） |
| CI | 新 job：交叉编译 ohos 产物（真机签包可仍在本机 DevEco） |

### 不宜一期改动的部分

- 重写 Vue → ArkUI  
- 容器化 agent 环境  
- 完整等价 Seatbelt/bwrap 沙箱  
- Homebrew / Tauri updater 兼容鸿蒙

---

## 6. 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| `ohos_golang_go` 与仓库 Go 1.26 版本差 | 编不过或标准库行为差 | Spike 锁定工具链版本；必要时引擎用独立 go.mod 降级 |
| 应用商店限制任意 native 子进程 | sidecar 不可行 | 改为进程内 `.so` + NAPI |
| 无可用 shell/PTY | Agent「写代码改文件」体验弱 | MVP 以会话+文件 API 为主；shell 标为实验 |
| modernc sqlite / 依赖在 ohos 的汇编/syscall | 运行崩溃 | Spike 强制跑 DB 迁移与并发读写 |
| 维护第四平台成本 | 发布节奏变慢 | 功能门控 + 共享 HTTP API 契约测试 |
| 真机资源（鸿蒙 PC / 2in1） | 无法验证 | 立项前确保至少一台目标设备或云真机 |

---

## 7. 与现有中国区基础设施的关系

已有能力对鸿蒙**有帮助但不够**：

- `releases.danmo.ai` 更新镜像、微信/企微/飞书/QQ 渠道 → 可作下载与运营触达  
- 产品本身已是「本地优先 + 中文文档/Gatekeeper 说明」  

仍缺：

- HAP 工程与签名流水线  
- AppGallery 元数据与合规  
- 鸿蒙专用帮助文档（权限、安装、数据目录）

---

## 8. 决策建议

| 问题 | 建议 |
|------|------|
| 要不要做？ | 若目标用户含鸿蒙 PC / 国产桌面办公场景，**值得做 Spike**；完整对等桌面是多期工程，不是「加一个 pack target」。 |
| 第一期目标？ | **能聊、能管会话/项目元数据、能配模型**；Agent shell/终端标为降级。 |
| 架构？ | **路线 A**：ArkWeb + Go 引擎，镜像 Tauri 薄壳模型。 |
| 和现有三端关系？ | 保持 Win/macOS/Linux Tauri 主线；鸿蒙为并行壳，共享 `core/` + `frontend/`。 |
| 下一步唯一动作？ | **Phase 0 Spike**（ohos Go 编译 + SQLite + ArkWeb fetch），用结果决定是否开 `harmony/` 工程与 CI。 |

---

## 9. 参考

- 本仓库：`desktop/src-tauri/`、`scripts/build_sidecar.sh`、`frontend/src/utils/desktop.ts`、`docs/updater-signing.md`、`docs/agent-runtime-env-research.md`
- OpenHarmony-SIG Go：<https://gitcode.com/openharmony-sig/ohos_golang_go>（`GOOS=openharmony`，runtime 报告为 linux 兼容层语义）
- 社区先例：Tauri 应用去壳后以 native `.so` + ArkTS 跑在鸿蒙 2in1；Electron 兼容层项目存在但维护重，不适合本产品首选

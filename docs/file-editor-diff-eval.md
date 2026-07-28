# 程序文件编辑器 & Git Diff 评估

> 背景：Files / Document Stage 目前只对少数「工作产物」格式提供编辑面；`.go` / `.ts` / `.py` 等程序文件落入 `preview`，Changes 面板仅列变更路径、无 diff。  
> 问题：是否需要引入强大的程序文件编辑器（Monaco 等），以及文件级 git diff 差异对比？  
> 结论先行：**不要做完整 IDE 级编辑器；应做轻量代码阅读/轻改 + Changes→Diff 审阅。** 与产品定位「不是 IDE」一致，同时补上 Agent 写码场景下人机共审的关键缺口。

**落地状态（已实现）**：`kind: code` / `kind: diff` Document Stage、`GET .../git-diff`、Changes 打开 Diff、CodeSurface 选区批注（含行号）→ Composer。编辑器为 **CodeMirror 6**（`@codemirror/language-data` 按扩展名语法高亮；暗色 one-dark）。

---

## 1. 现状盘点

### 1.1 Document Stage 路由（`frontend/src/utils/office-route.ts`）

| Kind | 格式 | Surface | 能力 |
|------|------|---------|------|
| `doc` | `.md` / `.markdown`（非 slides） | TipTap | 富文本编辑 ↔ Markdown |
| `slides` | slides MD / `*-slides.html` | textarea + Present | 编辑 MD；程序同步 HTML |
| `sheet` | `.csv` / `.danmo-sheet.json` | 网格 | 表格编辑 |
| `preview` | **其余一切**（含源码、json、yaml、通用 html、图片…） | iframe / img | **只读预览**，无语法高亮编辑 |

程序语言文件（`.go` / `.ts` / `.py` / `.rs` / `.json` / `.yaml` / `.toml` / `.sh` …）全部走 `preview`：浏览器按 MIME 当纯文本或下载，**无高亮、无行号、不可编辑保存**。

### 1.2 Changes 面板（`ChangesPanel.vue`）

已有：

- `GET .../git-changes`（`git status --porcelain`）
- 分支列表 / checkout
- staged / unstaged 分组与过滤

缺口（代码已留 stub）：

```ts
function jumpToFile(file: string) {
  // Expose selection for future file open; copy path for now
  void navigator.clipboard?.writeText(file).catch(() => {})
}
```

- 点击变更 → 仅复制路径，**不打开 Stage、不展示 diff**
- 后端**无** `git-diff` API（只有 status / branches / checkout）

### 1.3 依赖现状

`frontend/package.json`：**无** Monaco / CodeMirror / diff2html / shiki。唯一富编辑器是 TipTap（文档专用）。

### 1.4 Agent 侧已具备的能力

Agent 已通过 `read_file` / `write` / `edit` / `apply_patch` / `grep` / `glob` 读写任意文本文件；turnlog 有 `file_changes.jsonl`（工具突变轨迹，非 UI diff）。

**人的缺口**：Agent 改完代码后，用户在 UI 上无法舒适地「看懂改了什么 / 随手修一行」。

---

## 2. 产品约束（决策边界）

摘自 `docs/core-design.md`：

> **不是 Agent 工具，不是 IDE，是「人机共思」的实时协作操作系统。**

| 应坚持 | 应避免 |
|--------|--------|
| 中心画布仍是 **Document Stage**（按 kind 换 surface） | 把 Stage 变成 VS Code / Cursor 克隆 |
| 代码编辑是「审阅与轻改」，主编辑权在 Agent Loop | LSP、多标签 IDE、调试器、重构、Git 完整客户端 |
| Changes = **人机共审交付物** | 与 JetBrains / VS Code 功能对标的 source control |

Code 场景在架构上是同一 Agent Loop 的参数配置（浅调用、硬超时），**不是**第二套 IDE 产品线。

---

## 3. 需求强度评估

### 3.1 程序文件编辑器

| 用户场景 | 频率（推断） | 无编辑器时的体验 | 痛点等级 |
|----------|--------------|------------------|----------|
| Agent 写/改源码后，用户打开文件核对 | 高（有 Coding 能力时） | iframe 纯文本或无法阅读 | **高** |
| 用户改一个 typo / 常量 / 注释 | 中 | 只能回 Composer 让 Agent 改，或外开编辑器 | **中** |
| 用户长时间手写几百行新代码 | 低（与定位冲突） | 应用外部 IDE | 低（不应产品内解决） |
| 跳转定义 / 补全 / 重构 | 低→产品外 | — | **不做** |

**判断**：需要的是 **Code Surface（阅读优先 + 轻量编辑）**，不是「强大程序文件编辑器」。

### 3.2 Git Diff

| 用户场景 | 频率 | 现状 | 痛点等级 |
|----------|------|------|----------|
| Agent 一轮改了 N 个文件，扫一眼 diff 再继续 | **很高** | 只有路径列表 | **很高** |
| 点开某个变更文件看 unified / side-by-side | 高 | 无 API、无 UI | **高** |
| stage / commit / push / PR 全流程 | 中 | 无；终端 / 外开 Git 可用 | 中（可后置） |
| merge conflict 可视化解决 | 低 | 无 | 低（本期不做） |

**判断**：Diff 审阅是 Changes 面板的**自然下一跳**，投入产出比高于全面代码编辑器；且 stub 已明示「future file open」。

---

## 4. 方案对比

### 4.1 程序文件：不做 / 轻量 / 重型

| 方案 | 内容 | 包体 / 复杂度 | 与定位契合 | 建议 |
|------|------|---------------|------------|------|
| **A. 维持 preview** | 继续 iframe | 0 | 编码体验差 | 否 |
| **B. Code Surface（推荐）** | 新 `kind: code`；Monaco 或 CodeMirror 6；语法高亮 + 行号 + 保存；可选只读默认 / 显式 Edit | Monaco ~包体积大；CM6 更轻 | 高 | **做** |
| **C. 完整 IDE 壳** | 多文件 tabs、LSP、终端联动调试、搜索替换工作区 | 极高 | 低（变 IDE） | **不做** |

编辑器选型（若做 B）：

| 库 | 优点 | 缺点 | 倾向 |
|----|------|------|------|
| **CodeMirror 6** | 体积可控、Vue 友好、够用高亮/编辑 | 生态比 Monaco 窄 | **首选 MVP** |
| **Monaco** | 体验接近 VS Code、自带 diff editor 可复用 | 体积大、worker 配置重 | Diff 面板可单独用 monaco-diff，或统一用 Monaco 一处搞定 |
| TipTap | 已有 | 不适合代码 | 否 |

推荐默认：**只读高亮打开**；工具栏「编辑」后可写回 `PUT .../files/content`（已有 UTF-8 写入 API，1 MiB 读上限需在大文件时提示）。

路由扩展建议：

```
routeOfficeFile:
  已知文本源码扩展名 → kind: code, mode: view（默认）
  其余未知二进制 → 维持 preview
```

不必为每种语言单独 Surface；一种 `CodeSurface` + language-from-extension 即可。

### 4.2 Git Diff：列表 / 只读 Diff / 完整 Git UI

| 方案 | 内容 | 建议 |
|------|------|------|
| **A. 维持列表** | 仅路径 | 否（缺口过大） |
| **B. 只读 Diff 审阅（推荐）** | `GET .../git-diff?path=&staged=` → unified；Changes 点文件打开 Diff pane（可嵌在 Stage 或 Changes 下半区）；可选「在编辑器打开」 | **做** |
| **C. 完整 Git 客户端** | stage/unstage、commit、push、stash、conflict | 后置；终端已可覆盖 |

后端最小 API：

```
GET /api/v1/projects/:id/git-diff?path=<rel>&staged=0|1
→ { path, staged, patch, truncated?, ... }

可选后续：
GET .../git-diff?commit=HEAD   # 工作区相对 HEAD 总览（慎：大仓库）
```

实现：`git diff` / `git diff --cached`，路径校验对齐现有 `ensureGitRepo` + project 目录限制；硬顶 patch 大小（如 512KiB～1MiB）避免拖垮前端。

前端：优先 **unified diff 渲染**（自研轻量或 `diff2html` / Monaco DiffEditor）。MVP 不必 side-by-side。

### 4.3 与 Agent file_changes 的关系

| 来源 | 含义 | UI |
|------|------|-----|
| `git status` / `git diff` | 工作区相对 index/HEAD | Changes Tab（人审交付） |
| turnlog `file_changes` | 本回合工具写了哪些文件 | Stream / 回合摘要（已有或可增强链接） |

两者互补：回合结束可深链到 Changes 中对应 path 的 diff，**不要**用 turnlog 替代 git diff。

---

## 5. 推荐结论

| 议题 | 结论 | 理由 |
|------|------|------|
| **强大程序文件编辑器（IDE 级）** | **不需要** | 与「不是 IDE」冲突；主编辑路径应是 Agent + 外置 IDE |
| **轻量 Code Surface** | **需要（P1）** | 源码目前无法可读可改；成本可控，补齐 Coding 场景的人审 |
| **Git Diff 差异对比** | **需要（P0）** | Changes 面板半成品；Agent 改码后最高频人机共审动作 |
| **完整 Git 工作流 UI** | **暂缓** | 终端 + 外置工具足够；避免范围膨胀 |

优先级：**P0 Diff 审阅 → P1 Code Surface → P2 Changes 点文件打开 Code / Diff 联动 → 明确不做 LSP/多标签 IDE。**

---

## 6. 建议落地范围（若立项）

### 6.1 MVP（建议一次 PR 可切两刀，或先 Diff 后 Code）

1. **Backend**：`GetGitDiff(projectID, path, staged)` + API 路由；路径必须落在项目目录；输出截断。
2. **ChangesPanel**：点击文件 → 打开 Diff 视图（Stage 新 mode 或面板内 split）；删除「仅复制路径」作为主行为（可保留次级「复制路径」）。
3. **CodeSurface（可紧随其后）**：
   - `OfficeKind` 增加 `code`
   - `routeOfficeFile` 扩展常见源码扩展名
   - CodeMirror 6（或 Monaco）只读默认 + Edit/Save
   - 大文件 / 二进制仍走 preview 或拒绝编辑

### 6.2 非目标（写进 Stage 非目标列表）

- Language Server / IntelliSense / 跳转定义
- 工作区全局搜索替换 UI
- Debug、断点、测试运行器
- Git commit / push / PR / merge conflict 可视化
- 把 TipTap 文档编辑替换为代码编辑器
- OOXML / 协同（仍见 `core-design.md` §13.3）

### 6.3 成功标准

- 用户能在 Changes 中对任意文本变更文件看到可读 unified diff（含 staged/unstaged）
- 从 Files 打开 `.ts` / `.go` / `.py` 等可语法高亮阅读，并可保存小改
- 包体与首屏：Code/Diff 相关 chunk **按需加载**，不影响 doc/slides/sheet 主路径
- 文档与定位表述不出现「内置 IDE」承诺

---

## 7. 风险与缓解

| 风险 | 缓解 |
|------|------|
| Monaco 打包膨胀、桌面端体积 | 优先 CM6；或 Monaco 动态 `import()`；diff 与 editor 同库则选 Monaco 一次到位 |
| 大 diff / 大文件卡死 UI | 后端截断 + 前端虚拟列表 / 「仅显示前 N 行」 |
| 用户期待「越用越像 Cursor」 | 产品文案与非目标写死；编辑器默认只读 |
| 与 Document Stage AI 工具栏混淆 | Code Surface **不**默认挂润色/改稿 office-edit；改码继续走 Composer / Agent |
| 二进制 / 锁文件误开编辑 | 扩展名白名单 + `ReadFileContent` binary 检测 |

---

## 8. 对 `core-design.md` 的建议修订（立项后）

在 §13 Document Stage 路由表增加一行：

| `code` | 常见源码 / 配置文本 | CodeSurface（高亮；默认 view） | view |

在 §13.3 非目标追加：LSP / IDE 壳 / 完整 Git 客户端。

另增简短 §：Changes Diff 审阅为右侧 Changes 的一等能力，API `git-diff`。

---

## 9. 一句话决策

**需要 Diff，需要「能读能轻改」的代码面；不需要强大的程序 IDE。**  
先补齐人机共审（git diff），再用轻量 Code Surface 填上 Files 对源码的空白——两者都服务于 Agent 写码，而不是把 Danmo Work 做成编辑器产品。

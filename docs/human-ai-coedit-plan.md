# 人与 AI 协同编辑增强方案

> 目标：坚持 **人机共审、文本 SoT、Turn 可恢复** 的路线，深度参考开源实现的交互与工具模式，把 Document Stage 从「整回合写盘 → 整页 reload」提升到 **可预览、可分块接受、可中途决策** 的协同编辑体验。  
> 边界对齐 [`core-design.md`](./core-design.md) §13 / §13.3：**不做 Yjs 多人文档、不做 OOXML 套件、不做 IDE 壳**。  
> 相关已落地：[`file-editor-diff-eval.md`](./file-editor-diff-eval.md)（Code / Diff Stage）、`ask_user` / permission、office-edit 工具栏。

**落地状态（进行中）**

| 阶段 | 状态 |
|------|------|
| **P0** 快照 + AI Diff banner + Keep/Revert | **已实现** |
| **P1** hunk 级 Accept（Diff Stage） | **已实现**（真 `propose_only` 写 shadow 仍后续） |
| **P2** Sheet 选区/填充/公式展示；Slides 主题+布局 | **已实现首批**；虚拟滚动 / HyperFormula / 按页 Accept 见 §7 P2 |

---

## 1. 结论先行

| 决策 | 说明 |
|------|------|
| **协同对象 = 人 ↔ AI，不是人 ↔ 人** | 产品壁垒是 Agent Loop + Turn Log + Stage；不是石墨式实时共编 |
| **保持文本 SoT** | Doc/Slides = Markdown；Sheet = CSV / `.danmo-sheet.json`；Agent 仍走 `read_file` / `edit` / `apply_patch` / `write` |
| **增强点 = 审阅层，不是 CRDT 层** | 借 Cursor / TipTap AI Toolkit / Liveblocks 的 **propose → review → accept/reject**；拒绝 Electric「AI 作为 Yjs peer」作为主路径 |
| **表格 / 幻灯片继续自研** | 深度参考 MIT 产品的能力点与交互，不嵌入 Univer / Marp-core / Reveal 整引擎 |
| **分三期落地** | P0 提案审阅；P1 流式可见 + 分块接受；P2 Surface 能力与 office-edit 深化 |

原则：

1. **人始终可介入**：AI 改稿默认进「提案」或「可回滚写盘」，人决定留下什么。
2. **轨迹可审计**：提案、接受、拒绝都进 Turn Log / `file_changes`，IM / Web / 桌面同一语义。
3. **编辑器可换、契约不变**：Stage surface 可变厚；Agent 工具契约与 SoT 格式不因 UI 而变成 OOXML/CRDT。

---

## 2. 现状（基线）

当前人机共编是 **回合制、写盘中介、结束后 reload**：

```
人：Office AI 工具栏 / Composer 批注 / ask_user 回答
  → ensureSaved（dirty → 写盘）
  → Turn 进行中：Stage 只读
  → Agent：read_file + edit/write/apply_patch → 磁盘
  → Turn 结束：Stage 整页 reload + 恢复 scroll / pageIndex
```

| 已有能力 | 缺口 |
|----------|------|
| `[office-edit]` → session turn（非独立 `/office/ai`） | AI 写入前无 inline 提案预览 |
| Doc 选区 / 全文；Slides 当前页；Sheet 整表 Markdown | 无分块 Accept / Reject |
| Code / Preview 选区 → Composer 附件 | 改完只有 reload，无 before/after 对照 |
| `ask_user` + permission 卡（Stream + Composer） | Turn 中用户不能边看边改同一文件 |
| Git Diff Stage（人审工作区变更） | **AI 提案 Diff** 与 Git Diff 未打通 |
| `file_changes.jsonl` 记录工具突变 | UI 未从该轨迹驱动 Stage 审阅 |
| 表格 / 幻灯片自研 MVP | 公式/主题布局弱；与协同审阅未耦合 |

非目标（继续有效，见 §13.3）：

- Yjs / OT 多端实时共编  
- OOXML 作为 SoT、OfficeCLI 导出为主路径  
- 独立 Office AI HTTP 端点  
- LSP / 完整 IDE / merge-conflict 工作台  

---

## 3. 开源参考：取什么、不取什么

### 3.1 深度参考（模式层）

| 来源 | 可借鉴 | 落地到 Danmo 的形态 |
|------|--------|---------------------|
| **Cursor / Copilot 交互**（产品参考，非依赖） | Inline diff、Accept / Reject / Accept file、改完可继续聊 | Stage「AI Diff」层 + Composer 续写 |
| **TipTap AI Toolkit**（商业组件，只借模式） | Schema-aware 编辑；suggestion 节点；Compare before/after | Doc：TipTap decoration / suggestion marks（自研轻量，不引入 Enterprise SDK） |
| **Liveblocks AI copilot 文** | 私有 review 视图；Accept 才发布；流式时隐藏尾部假删除 | 「提案缓冲」：磁盘或内存 shadow，Accept 才合并进 SoT |
| **Aider / Continue apply** | Patch 先展示再应用；失败可重试 | 复用 `apply_patch` / unified diff；UI 挂到 Stage |
| **本仓已有轨迹** | `apply_patch`、`file_changes.jsonl`、DiffSurface、ask_user | **优先接线**，少造新抽象 |

### 3.2 明确不采用（或降级为远期实验）

| 来源 | 为何不作为主路径 |
|------|------------------|
| **Electric：AI as Yjs peer** | 与 §13.3「不做 Yjs」冲突；IM / 无头 Turn / 多入口无法要求「文档打开在浏览器」 |
| **Univer / FortuneSheet 整引擎** | Sheet SoT 变 workbook 运行时，Agent 文本编辑失真；包体与 Stage 契约成本高 |
| **Marp-core / Slidev / Reveal 默认 Present** | 破坏「MD SoT + Stage 派生 HTML、Agent 不手写 playable HTML」 |
| **OnlyOffice / Collabora** | AGPL/MPL 与「嵌入套件」路线；不是人机共审增强 |

### 3.3 表格 / 幻灯片：参考 MIT，增强自研

| Surface | 参考对象（MIT / Apache） | 借鉴内容 | 仍自研 |
|---------|--------------------------|----------|--------|
| **Sheet** | Univer OSS（交互）、HyperFormula（公式引擎可选）、FortuneSheet（选区/填充 UX） | 虚拟滚动、选区、填充柄、公式以**单元格字符串**入库 | `.danmo-sheet.json` / CSV SoT、office-edit、提案 Diff |
| **Slides** | Marp 方言与主题约定 | `theme` CSS、`_class` / 分栏等少量布局 | `slides-render.ts`、Present HTML、按页提案 |

---

## 4. 目标体验（人机协同环）

统一为 **四拍环**（所有 kind 共用语义，Surface 实现不同）：

```
1. Intent     人指定范围（选区 / 页 / 表 / 附件批注）+ 指令
2. Propose    AI 产出提案（patch / 全文替换 / 流式预览），不默认真相覆盖
3. Review     人在 Stage 看 Diff / 分块，可改指令续跑或 ask_user 作答
4. Commit     Accept → 写入 SoT + 记轨迹；Reject → 丢弃提案，SoT 不变
```

与现网兼容：

- **Auto-commit 模式**（默认可保留）：权限信任 / 小改 / CLI·IM 无 Stage 时，Propose 后直接 Commit（今日行为）。  
- **Review 模式**（桌面 / Web Stage 打开该文件时默认）：先提案，等人点 Accept。

IM 通道：无 Stage 时走 Auto-commit + `ask_user`；若用户稍后在桌面打开文件，Changes / AI Diff 仍可从 `file_changes` 回看。

---

## 5. 架构方案

### 5.1 新增概念：Edit Proposal（提案）

不新增「Office AI 服务」，仍走 Session Turn。增加 **提案产物**（会话目录或 Stage 本地）：

```
~/.danmo-work/data/projects/<id>/sessions/<sid>/
  file_changes.jsonl          # 已有：已提交到磁盘的工具写
  proposals/<proposalId>.json # 新增：未接受的 AI 提案元数据
  proposals/<proposalId>.diff # 或 .patch / before-after 快照
```

`proposals/*.json` 最小字段：

| 字段 | 含义 |
|------|------|
| `id` / `turnId` / `callId` | 关联 Turn / tool |
| `path` / `kind` | Stage 路由 |
| `mode` | `patch` \| `replace` \| `cell-range` \| `slide-pages` |
| `status` | `pending` \| `accepted` \| `rejected` \| `superseded` |
| `baseHash` | 提案基于的文件内容哈希（防脏写） |
| `scopes` | 选区 / pageIndex / sheet 名等 |

**提交规则**：Accept 时若 `baseHash` ≠ 当前文件 → 冲突卡（提示 Reload / Overwrite / 重新生成），**不做**三路 merge UI（非目标）。

### 5.2 Agent 工具侧（渐进）

| 阶段 | 行为 |
|------|------|
| **P0** | 保持 `edit` / `write` / `apply_patch` 写盘；Turn 结束后用 **before 快照（turn 开始时 ensureSaved 的副本）vs after** 生成提案 Diff，供 UI 审阅；提供「Revert file to pre-turn」= Reject 整文件 |
| **P1** | 新增或扩展工具标志 `propose_only` / 会话策略 `edit.applyMode=propose`：工具写到 `proposals/` shadow，不碰 SoT；Accept 时再 `apply_patch` |
| **P2** | Doc：可选结构化 ops（段落级）；Sheet：`cell-range` patch；Slides：多页 patch；仍落文本 |

优先 **P0 不改工具契约**（最快增强体验），P1 再切「默认不写 SoT」。

### 5.3 前端 Stage：AI Diff 层

在现有 `DiffSurface`（Git）旁增加 **同源组件能力**：

| UI | 行为 |
|----|------|
| **AI Diff banner** | Turn 结束后若该 path 有 pending / 刚提交变更 → 条带：Accept all / Reject（revert）/ 在 Diff 中查看 |
| **分块操作** | unified hunk → Accept hunk / Reject hunk（P1；P0 可先整文件） |
| **流式预览（P1）** | Doc：TipTap 只读 + decoration 显示插入/删除；Sheet：变更单元格高亮；Slides：页缩略图角标 + 页内 diff |
| **与 Stream 联动** | 点击 `file_changes` / tool 行 → 打开对应 path 的 AI Diff（补齐 file-editor-diff-eval 的「软未来」） |

Turn 进行中：

- **P0**：维持只读（简单、无冲突）。  
- **P1**：允许人编辑 **未 overlapping 区域** 仅作可选实验；默认仍只读 + 流式提案层，避免 OT。

### 5.4 与 ask_user / permission 的关系

| 机制 | 职责 |
|------|------|
| `permission` | 高风险工具（shell / 外网 / MCP external）放行 |
| `ask_user` | 语义澄清、选项、表单（思维流内决策） |
| **Edit Proposal Review** | **内容层**放行：改稿是否进 SoT |

三者并列，不互相替代。Office 工具栏「修改」在 Review 模式下可在 prompt 约束中写明：`prefer apply_patch；桌面将展示提案供用户接受`。

### 5.5 office-edit 契约增强（不改入口形态）

继续 `buildOfficeEditPrompt` + Document Agent skills；增量字段：

```
[office-edit]
action: …
path: …
kind: …
scope: selection|document|slide|sheet
review: propose|commit    # 新增；UI 模式注入
base_hash: …              # 可选；与提案校验一致
```

Skills（`document-writing` / `playable-slides` / `sheet-writing`）补充：

- `review: propose` 时优先最小 diff / `apply_patch`；禁止无关文件。  
- Slides：仍禁止手写 playable HTML。  
- Sheet：输出仍为 CSV/JSON 文本；公式以字符串单元格表示（若启用）。

---

## 6. 分 Surface 增强（自研 + 开源参考）

### 6.1 Doc（TipTap）— 协同编辑主战场

| 项 | 做法 | 参考 |
|----|------|------|
| 选区 polish/modify | 已有；提案 Diff 高亮选区映射 | TipTap suggestion / Liveblocks review |
| 段落级 Accept | ProseMirror decoration 映射 unified diff 行 → 块 | TipTap AI Toolkit「every edit is a suggestion」 |
| Continue | 已有 action；Commit 后光标处续写 | Cursor continue |
| 不引入 | Tiptap Enterprise AI Toolkit 依赖 | 模式自研 |

### 6.2 Code（CodeMirror 6）— 对齐「写码共审」

| 项 | 做法 |
|----|------|
| 批注 → Composer | 已有 |
| AI 改码审阅 | Turn 后 AI Diff（可复用 DiffSurface + hunk accept） |
| 不挂 Office AI 工具栏 | 保持；改码走 Composer / Agent |

### 6.3 Slides（自研渲染）

| 项 | 做法 | 参考 |
|----|------|------|
| 按页提案 | `pageIndex` 范围 diff；Accept 只合并指定 `---` 页 | Marp 分页模型 |
| 主题 / 布局 | `slides-render.ts` 增加 2–3 theme + 少量 `_class` | Marp 方言 |
| Present | 仍 Stage 派生 HTML；提案未 Accept 不进 Present 源 | 现契约 |
| 不引入 | Marp-core 默认引擎 | — |

### 6.4 Sheet（自研网格）

| 项 | 做法 | 参考 |
|----|------|------|
| 提案可视化 | 变更单元格底色 + 行级 Accept（P1） | FortuneSheet 选区反馈 |
| 能力增强（并行轨） | 虚拟滚动 → 选区/填充 → 可选 HyperFormula | Univer OSS / HyperFormula |
| office-edit | 从「整表 MD」逐步支持 `sheet+range` scope | — |
| 不引入 | Univer 整包替换 SoT | — |

---

## 7. 分期路线

### P0 — 可见、可回滚（优先，契约零破）

**目标**：人能看懂 AI 刚改了什么，并能一键撤销。

1. Turn 开始：对 office-edit / 显式附件涉及的 path 做 **pre-turn 快照**（内容哈希 + 可选全文，大文件只哈希+diff 用）。  
2. Turn 结束：若 path 变更 → Stage 顶栏 **AI 变更条** + 打开 AI Diff（复用 / 抽取 DiffSurface）。  
3. **Reject = 恢复快照**；**Accept = 关闭条带**（文件已在盘上）。  
4. Stream / Changes：从 `file_changes.jsonl` 跳到对应 Diff。  
5. 设置项：`stage.aiReview.afterTurn` = `banner` \| `off`（默认 banner）。

**验收**：Doc/Slides/Sheet/Code 任一 AI 改稿后，用户无需 git 即可看 diff 并 revert。

### P1 — 真提案 + 分块接受

**目标**：默认不直接覆盖信任文件；人按 hunk / 页 / 行接受。

1. 会话或项目策略 `edit.applyMode=propose`（Web/Desktop Stage 聚焦文件时默认 on）。  
2. 工具写 shadow proposal；Accept 写 SoT；Reject 删提案。  
3. Hunk / slide-page / sheet-row 级 Accept。  
4. 流式：Doc 只读 decoration 跟随最后一次 `apply_patch` 预览（可先非 token 级，按 tool 结束刷新）。  
5. `baseHash` 冲突卡。

**验收**：润色长文时可只接受部分段落；幻灯片可只接受第 3 页。

### P2 — Surface 能力与协同深度（加深）

**目标**：表格/幻灯片在「Agent 可写文本 SoT」前提下够用；审阅环与 Surface 能力对齐。  
**原则**：P2 加厚交互与方言，**不**引入 Univer / Marp-core / HyperFormula 为默认依赖；公式引擎与虚拟滚动按需再开。

#### P2.1 Sheet（已部分落地 / 后续）

| 项 | 状态 | 说明 |
|----|------|------|
| 选区 + Shift 扩展 | **已做** | `selectionRange`；office-edit selection 带 `range: A1:B3` |
| 向下填充 | **已做** | 选区顶行复制到下方 |
| 公式字符串展示 | **已做** | `=` 前缀等宽 + accent（**不求值**；SoT 仍是字符串） |
| 虚拟滚动 | 后续 | 行数 > ~500 时再上；当前 DOM 表足够 Agent 产物 |
| HyperFormula 求值 | 后续可选 | 仅当用户明确要「算表」；默认不依赖，避免 SoT/导出语义分叉 |
| 单元格级 AI Diff 高亮 | 后续 | 在 AI Diff / Stage 上标变更格；依赖稳定 JSON 行序列 |

**Agent 契约**：`sheet-writing` 继续输出 CSV / `.danmo-sheet.json`；公式以单元格文本 `=SUM(...)` 保存；不要默认转 `.xlsx`。

#### P2.2 Slides（已部分落地 / 后续）

| 项 | 状态 | 说明 |
|----|------|------|
| 主题 `default` / `light` / `academic` | **已做** | frontmatter `theme:` → Present CSS |
| 布局 `lead` / `columns` | **已做** | `<!-- _class: lead -->` / `columns`（Marp 方言子集） |
| 按页 AI Diff | 后续 | 用 `---` 分页对 snapshot/current 做页级 Accept（复用 hunk 或页 merge） |
| 更多布局（invert / invert-list） | 后续 | 仍在 `slides-render.ts` 扩展，不绑 Marp-core |

**Agent 契约**：只改 MD；Present HTML 仍由 Stage 派生；布局用 HTML comment 指令，勿手写 playable HTML。

#### P2.3 协同深度（后续）

1. Doc：选区映射稳定的 block-id（可选，写入 HTML comment / MD 属性，谨慎）。  
2. Composer：「基于当前提案继续改」无需重读全文（带上 pending proposal id）。  
3. **明确不做**：Yjs 多人或「AI as CRDT peer」——除非修订 §13.3。

---

## 8. 模块改动清单（实施时）

| 层 | 改动 |
|----|------|
| `core/store/turnlog` | proposal store；可选 pre-turn snapshot API |
| `core/runtime/tool/builtin` | P1：`propose_only` / applyMode；返回 diff 摘要给 SSE |
| `core/service` + `server/api/v1` | `GET/POST .../proposals`：list / accept / reject / revert-pre-turn |
| `frontend` DocumentStage / DiffSurface | AI Diff banner、hunk actions、与 reload 策略协调 |
| `office-route` + skills + `document.md` | `review` 字段与提示词 |
| `docs/core-design.md` §13 | 落地后回写「提案审阅」为正式架构段落 |

---

## 9. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 大文件全文快照膨胀 | 阈值阈值；超限只存 hash + 依赖 git / 仅支持 Reject via git checkout |
| 提案与用户手改冲突 | `baseHash`；冲突时拒绝静默覆盖 |
| IM 无 Stage 却开启 propose | IM / CLI 强制 `commit`；桌面可事后 Diff |
| 流式装饰性能 | P1 按 tool 边界刷新，不做每 token ProseMirror 事务 |
| 误用商业 SDK | TipTap AI Toolkit 仅作文案级参考，不引入依赖 |
| 表格公式复杂度 | 公式列 P2 可选；默认仍纯文本格 |

---

## 10. 成功标准

| 指标 | 含义 |
|------|------|
| **可解释** | 任意 office-edit 后 1 步内看到「改了什么」 |
| **可逆** | Reject / Revert 不依赖用户会用 git |
| **可选择** | P1：至少 Doc hunk + Slides page 级 Accept |
| **不偏航** | 无 Yjs 主路径、无 OOXML SoT、Agent 仍文本工具 |
| **多入口一致** | 同一 Turn 在 Web / 桌面审阅语义一致；IM 降级 Auto-commit |

---

## 11. 建议默认策略

| 场景 | applyMode |
|------|-----------|
| Web / Desktop，Stage 打开且 kind ∈ doc/slides/sheet/code | `propose`（P1 起） |
| P0 过渡期 | `commit` + after-turn banner + revert |
| CLI / TUI / IM | `commit` |
| 用户 Settings 覆盖 | `stage.aiReview.applyMode` |

---

## 12. 总结

> **人与 AI 协同编辑 = 提案审阅环 + 文本 SoT + 既有 Agent Loop**，不是多人 CRDT Office。

深度参考开源的是 **Cursor / TipTap AI / Liveblocks 的审阅与建议模式**，以及 **Marp / Univer OSS / HyperFormula 的能力点**；自研继续扛 Stage、SoT、表格与幻灯片渲染。  
落地顺序：**先让人看见并撤销 AI 的改动（P0），再让人分块接受（P1），最后加厚 Sheet/Slides 能力（P2）**。

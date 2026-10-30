# 开源 Web UI 自动化路线与选型

Danmo 沙箱是 **Linux 桌面虚拟机**，工具在机内执行。本专家只做 **Web**；不测 Windows 原生软件。

## 两条主线

| 路线 | 代表 | 优点 | 成本 |
|------|------|------|------|
| 确定性脚本 | Playwright、Cypress、Selenium | CI 稳定、选择器生态成熟 | 选择器易碎；需另装 runner/浏览器矩阵 |
| Agent 驱动 | Midscene、Stagehand、Drik、AgenTeX、OpenCheck | 自然语言步骤；弱选择器；适合快速探索与回归叙述 | 需明确 Oracle；模型波动；不如纯脚本便宜 |

## 本仓库已有能力

- **`browser_*`（chromedp）**：粘性标签页、无障碍树 snapshot + ref 点击/输入，适合结构化 Web。
- **`computer`**：X11 桌面真实窗口（headed Chromium 等），see→act→verify，适合视觉/canvas/无语义 DOM。

缺口历来是：**用例设计规范、多场景执行、PASS/FAIL 报告**——不是再造浏览器驱动。

## Danmo `ui-test` 选型（v1）

- 用例：项目内 **Markdown specs**（对齐 Drik / AgenTeX 风格步骤动词）。
- 执行：**优先 `browser_*`，必要时 `computer` 开 Linux 桌面浏览器**。
- **不**在 v1 引入 Playwright/Midscene/Stagehand 工程或依赖。
- 若团队日后需要纯 CI 脚本，可将稳定场景再人工沉淀为 Playwright；那是后续阶段，不是本专家默认产出。

## 何时仍应考虑脚本框架（知识层提示）

- 需要无模型、毫秒级重放的大规模 CI 矩阵。
- 断言必须是 deterministic DOM API，且团队已有 Playwright 基建。

此类需求交给实现/CI 专家路线，而不是本专家默认行为。

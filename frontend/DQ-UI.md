# Danmo UI in Danmo Work

与 [DanQing-Studio](../DanQing-Studio/frontend/DQ-UI.md) 一致，使用共享包 [`../dq-ui`](../dq-ui)。

## 栈

| 层 | 包 |
|----|-----|
| Tokens | `@danqing/dq-tokens`（含 `dq-agent.css`） |
| 组件 | `@danqing/dq-ui`（`Dq*`） |
| Shell | `@danqing/dq-shell`（反馈、图标、`DqToolCard` / `DqPillTabs`） |

**禁止 Element Plus**：模板仅使用 `Dq*` 组件。

## 约定

- **主题切换**：使用 `applyDqTheme` / `THEME_OPTIONS`（见 `@danqing/dq-tokens`），经 `stores/theme.ts` 持久化；不要维护私有主题 class 列表。
- **间距 / 半径**：优先 `--dq-space-*`、`--dq-radius-*`；产品语义层仅保留仍在用的 `--teams-*`（glass / surface / radius）。
- **Size**：紧凑控件只用 `size="sm"`（禁止 `small` / `mini`）。
- **Select**：Composer / 工具栏用 `size="sm" variant="ghost"`。
- **Agent UI**：`main.ts` 引入 `@danqing/dq-tokens/dq-agent.css`；对话正文用 `.dq-prose`；工具行用 `DqToolCard` + `.dq-status-dot`；右栏 tabs 用 `DqPillTabs`。
- **焦点 / 悬停**：`--dq-focus-ring`、`.dq-hoverable`；禁止自造 `0 0 0 2px` 环。
- **管理页**：统一 `WorkspaceShell` + `DqSelect` / `DqInput` / `DqSegmented`（或 `DqSectionTabs`）。
- **浮层玻璃**：Composer / Dialog / Popover 使用 `.dq-glass--*` / `--dq-glass-blur*`（含 `.dq-glass--composer`）；主壳为靠边扁平布局。
- **禁止**全局 `html * { transition: ... }`；主题切换只过渡 `html` / `body`。

## 本地开发

```bash
cd ../dq-ui && pnpm install && npm run build
cd ../DanQing-Teams/frontend && npm install && npm run dev
```

修改 `dq-ui` 后需重启 Vite；tokens / ui / shell 变更后在 `dq-ui` 执行 `npm run build`（或 `make check`）。

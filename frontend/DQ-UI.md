# Danmo UI in Danmo Work

与 [DanQing-Studio](../DanQing-Studio/frontend/DQ-UI.md) 一致，使用共享包 [`../dq-ui`](../dq-ui)。

## 栈

| 层 | 包 |
|----|-----|
| Tokens | `@danqing/dq-tokens`（含 `dq-agent.css`、`dq-recipe-*.css`） |
| 组件 | `@danqing/dq-ui`（`Dq*`） |
| Shell | `@danqing/dq-shell`（反馈、图标、`DqToolCard` / `DqPillTabs`） |

**禁止 Element Plus**：模板仅使用 `Dq*` 组件。

## 主题（精选 5）

全部为 **macOS 26 Liquid Glass**（半透壳层 + 真玻璃浮层）；色相 / accent 不同。

| Slug | 明暗 | 说明 |
|------|------|------|
| `mac` | 暗 | 默认；系统蓝 |
| `mac-light` | 亮 | 系统蓝亮色 |
| `tokyo-night` | 暗 | 冷蓝海军 |
| `nord-dark` | 暗 | 北极霜色 |
| `minimal-light` | 亮 | 极简纸白 |

`main.ts` 需引入对应 palette + `dq-recipe-dark.css` / `dq-recipe-light.css` + `dq-glass.css`。已删除主题的 localStorage slug 经 `resolveDqThemeSlug` 回落到 `mac` / `mac-light`。

## 约定

- **主题切换**：使用 `applyDqTheme` / `THEME_OPTIONS` / `resolveDqThemeSlug`（见 `@danqing/dq-tokens`），经 `stores/theme.ts` 持久化；不要维护私有主题 class 列表。
- **字号（跨主题统一）**：`caption` 12 / `nav` 13（仅 shell 侧栏导航行）/ `body`=`prose` 14 / `title` 16。主题 CSS 禁止覆盖 `--dq-font-size-*`，也禁止在 `html` 上设 `font-size`（rem 会二次缩放）。一页只用一个 `title`；区块与对话用 `body`；侧栏列表用 `nav`；提示用 `caption`；层级靠字重/颜色。侧栏禁止使用 550/650/700 等非标准字重。LeftRail、设置页中间栏、resource rail 列表项均属 shell nav，统一 `--dq-font-size-nav` + `--dq-sidebar-*` 字色 token，不用 `--dq-label-primary`。
- **间距 / 半径**：优先 `--dq-space-*`、`--dq-radius-*`；产品语义层仅保留仍在用的 `--work-*`（glass / surface / radius）。
- **Size**：紧凑控件只用 `size="sm"`（禁止 `small` / `mini`）。
- **Select**：Composer / 工具栏用 `size="sm" variant="ghost"`。
- **Agent UI**：`main.ts` 引入 `@danqing/dq-tokens/dq-agent.css`；对话正文用 `.dq-prose`；工具行用 `DqToolCard` + `.dq-status-dot`；右工作区用会话顶栏图标 + `DqDrawer`（玻璃浮层）。
- **焦点 / 悬停**：`--dq-focus-ring`、`.dq-hoverable`；禁止自造 `0 0 0 2px` 环。
- **管理页**：统一 `WorkspaceShell` + `DqSelect` / `DqInput` / `DqSegmented`（或 `DqSectionTabs`）。
- **壳层与浮层**：主壳靠边扁平（`radius/gap=0`）；暗色材质只由 `dq-recipe-dark.css` 定义（palette 不重写 shell/glass/composer）。Composer 是**一整块** `.dq-glass--composer` 胶囊，底托为内部细分割线，禁止第二层渐变/实色板。
- **禁止**全局 `html * { transition: ... }`；主题切换只过渡 `html` / `body`。

## 本地开发

```bash
cd ../dq-ui && pnpm install && npm run build
cd ../DanQing-Teams/frontend && npm install && npm run dev
```

修改 `dq-ui` 后需重启 Vite；tokens / ui / shell 变更后在 `dq-ui` 执行 `npm run build`（或 `make check`）。

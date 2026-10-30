# Oracle：如何判定 PASS / FAIL

Agent 跑 UI 时最大的风险是「自我打分」。Oracle 必须落在**可核对证据**上。

## 合格断言

| 类型 | 合格例子 | 不合格例子 |
|------|----------|------------|
| 文案 / 角色名 | snapshot 含 `heading "Dashboard"` | 「页面看起来正常」 |
| URL | 当前 URL 为 `/app/home` 或包含 `?ok=1` | 「应该跳转成功了」 |
| 禁止态 | snapshot **不含** `role: alert` / 文案 `Payment failed` | 「好像没有报错」 |
| 视觉（computer / screenshot） | 截图中可见绿色成功条、登录表单已消失 | 「布局挺好看」 |

## 规则

1. **每个 `verify` 必须引用证据**：最新 snapshot 片段、URL、或截图路径。
2. **`verify not` 同样要证据**：说明检查了哪些节点/文案后确认不存在。
3. **不要用模型心情代替检查**：禁止仅凭「我觉得过了」标 PASS。
4. **a11y 优先于像素**：能用 snapshot/URL 就不要只用截图当 Oracle（截图可作附件证据）。
5. **失败即 FAIL**：期望未出现或禁止态出现 → FAIL；工具不可用 → BLOCKER（不是假 PASS）。
6. **敏感操作**：未授权的真实登录/支付/删除不得标 PASS；先 `ask_user` 或标 BLOCKER。

## 与社区实践的对齐

- Agentic browser testing：accessibility tree 作主信号，像素作兜底。
- Drik / AgenTeX：步骤级 verify + 每场景证据落盘。
- Midscene `aiAssert`：自然语言断言仍需运行时失败抛错——本专家用显式 PASS/FAIL 表代替框架异常。

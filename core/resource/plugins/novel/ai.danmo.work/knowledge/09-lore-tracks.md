# 连载轨（作者 / 读者账本）

长篇连载要把「作者手里的底牌」和「读者已经知道的事」分开，否则模型会提前揭底。

## 两轨

| 轨 | 路径 | 谁可读 | 来源 |
|----|------|--------|------|
| **author-lore** | `canon/author-lore.md` | 作者 / 规划 / 改纲 | 立项时写入；含终局细节、隐藏规则、角色私密 |
| **ledger** | `continuity/ledger.md` | 写正文、审稿、续写 | **只从已 Commit 章节重建**：Public facts + Tracking + Open loops + Volume summaries + 当前卷章摘要 |

`candidate` vs `canon` 管人物卡能不能进正文。两轨管 **信息对读者是否已公开**。

## ledger 体积治理

ledger 只留**当前卷**章摘要明细；卷收束（人工确认）后该卷明细归档到 `continuity/summaries/vNN.md`，ledger 留 `### vNN 卷总结`（500–800 字）。体积 ≈ O(当前卷章数 + 卷数×800字)。gate `postcommit` 对归档章节照常有效。

## 写正文加载

允许：gate preflight 打印的 `### CONTEXT`（**风格指纹** / 上章接钩 / 点名角色 Cast snapshot 行 + 三锚点 + **在场角色间关系行** / 开放债务 ≤8 条 / 单元功能）+ 本章纲 + 上场 `canon/cast` 公开段。**模型不读 ledger 全文**——脚本抽取，模型只消费抽取结果。人物卡全文不进上下文：只有三锚点行与关系行（仅 `对方 ∈ 本章在场名单` 的行）被注入，伤口/弧光等半剧透字段不常驻。

**风格指纹**随 preflight CONTEXT 注入：`canon/style-fingerprint.md`（无则退回 bible `## Style card`）由脚本压缩 ≤480 字拼进 CONTEXT 顶部，每章写前自动对齐 POV/语域/句式/禁语/章末钩。上下文被裁剪导致本轮未见指纹时，按专家规则 `read_file` 补读该文件（不要扫树）。**角色卡不全量进 CONTEXT**：只按章点名注入三锚点，避免伤口/弧光等半剧透字段常驻与 POV 泄露。

禁止：`author-lore.md`、圣经「终局储备」细节栏、未 Commit 草稿里的剧透、`continuity/summaries/` 归档全量通读.

**章纲阶段例外**：写章纲前允许读 ledger 的 `### Cast snapshot` 小节（只读涉及角色的行）与点名人物卡的「关系」段，用于对齐 `state_deltas` 起点；仍不读 ledger 全文、不读 author-lore。

## Legacy

旧书可能仍有 `public-lore.md` / `tracking.md` / `chapter_summaries.md` / `foreshadow-tracker.md`。冷启动合并进 `ledger.md` 后移入 `_archive/`.

## Commit

每章定稿必须刷新 ledger（只加本章真正公开的 + 状态差 + 摘要块）。Gate 脚本 `--action postcommit` 检查 ledger（或 legacy 文件对）.

## Checklist

- [ ] 正文未使用 author-lore 中未到解锁卷的底牌
- [ ] ledger 没有作者侧真相
- [ ] Open loops ≤5
- [ ] 卷收束后旧卷明细已归档 `continuity/summaries/vNN.md`，ledger 只留卷总结
- [ ] 写前 preflight 未把 author-lore 列入读取回执

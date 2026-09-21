# 连载轨（作者 / 读者账本）

长篇连载要把「作者手里的底牌」和「读者已经知道的事」分开，否则模型会提前揭底。

## 三轨

| 轨 | 路径 | 谁可读 | 来源 |
|----|------|--------|------|
| **author-lore** | `canon/author-lore.md` | 作者 / 规划 / 改纲 | 立项时写入；含终局细节、隐藏规则、角色私密 |
| **facts（读者账）** | `continuity/facts.md` | 写正文、审稿、续写 | **只从已 Commit 章节重建**：Public facts + Tracking + Open loops + Volume summaries + 当前卷章摘要 |
| **commits（执行日志）** | `continuity/commits/vNN-U#.md` | 作者 / 回溯 | 每单元一个：gate 结果 / 四计数 / 锁词扫描 / 字数实测 / 扩写技术 / 偏离登记。**不进 facts**，避免日志污染事实层 |
| **locked-terms** | `canon/locked-terms.yaml` | gate 自动扫描 | 本卷之前不得出现的真名/真相词；`locked_until` 按卷解锁 + `compliance` 永久红线 |

`candidate` vs `canon` 管人物卡能不能进正文。三轨管 **信息对读者是否已公开** + **锁词是否提前泄漏**。

## facts 体积治理

facts 只留**当前卷**章摘要明细；卷收束（人工确认）后该卷明细归档到 `continuity/summaries/vNN.md`，facts 留 `### vNN 卷总结`（500–800 字）。体积 ≈ O(当前卷章数 + 卷数×800字)。gate `postcommit` 对归档章节照常有效。

**执行日志不进 facts。** gate 结果、四计数、锁词扫描、字数实测、扩写技术、偏离登记 → 全部写 `continuity/commits/vNN-U#.md`。facts 只回答"读者现在知道什么"，commits 回答"我们这一轮做了什么"。

## 写正文加载

允许：gate preflight 打印的 `### CONTEXT`（**风格指纹** / 上章接钩 / 点名角色 Cast snapshot 行 + 三锚点 + **在场角色间关系行** / 开放债务 ≤8 条 / 单元功能 / **本单元锁词清单**）+ 本单元细纲 + 上场 `canon/cast` 公开段。**模型不读 facts 全文**——脚本抽取，模型只消费抽取结果。人物卡全文不进上下文：只有三锚点行与关系行（仅 `对方 ∈ 本章在场名单` 的行）被注入，伤口/弧光等半剧透字段不常驻。

**风格指纹**随 preflight CONTEXT 注入：`canon/style-fingerprint.md`（无则退回 bible `## Style card`）由脚本压缩 ≤480 字拼进 CONTEXT 顶部。上下文被裁剪导致本轮未见指纹时，按专家规则 `read_file` 补读该文件（不要扫树）。**角色卡不全量进 CONTEXT**：只按章点名注入三锚点。

禁止：`author-lore.md`、圣经「终局储备」细节栏、未 Commit 草稿里的剧透、`continuity/summaries/` 归档全量通读。

**单元细纲阶段例外**：写细纲前允许读 facts 的 `### Cast snapshot` 小节（只读涉及角色的行）与点名人物卡的「关系」段，用于对齐 `state_deltas` 起点；仍不读 facts 全文、不读 author-lore。

## Legacy

旧书可能仍有 `continuity/ledger.md` / `public-lore.md` / `tracking.md` / `chapter_summaries.md` / `foreshadow-tracker.md`。冷启动合并进 `facts.md`（事实）+ `commits/`（执行日志分文件）后移入 `_archive/`. gate `ledger_path()` 自动 fallback 到 `ledger.md`，兼容旧书。

## Commit

每单元定稿必须：
1. 刷新 `facts.md`（只加本单元真正公开的 + 状态差 + 摘要块 + Open loops）。
2. 写 `continuity/commits/vNN-U#.md`（执行日志，按 `commit-log.md` 模板）。
3. Gate `--action postcommit --unit vNN-U#` exit 0（锁词/字数 floor/单元规模在 **precommit** 已硬检，postcommit 二次校验）。

## 伏笔表

伏笔是前段悄悄留下、回头看合得上的信号。分四类登记，不要只记「以后有反转」：

| 类 | 记什么 |
|----|--------|
| 主线 | 核心阴谋、最终真相、主线任务 |
| 人物 | 身份、秘密、立场会变 |
| 道具 | 信物、令牌、功法、武器 |
| 世界观 | 隐藏规则、力量体系、历史真相 |

每一条写进 facts 的 Open loops：**FS-id / 内容 / 状态（planted/advanced/paid）/ 埋点章 / 计划回收卷 / 实际回收章**。藏得住、收得回、回头看合理。坑比能填的多，就先停手。作者侧真相仍只在 author-lore，不要写进这张读者账。**总纲 `book_outline.md` 只列计划层（FS-id/内容/埋点卷/计划回收卷），运行时状态在 facts 维护**。

## Checklist

- [ ] 正文未使用 author-lore 中未到解锁卷的底牌
- [ ] facts 没有作者侧真相
- [ ] Open loops ≤5，无 dangling
- [ ] 锁词无泄漏（gate precommit 自动扫描 `canon/locked-terms.yaml`）
- [ ] 卷收束后旧卷明细已归档 `continuity/summaries/vNN.md`，facts 只留卷总结
- [ ] 执行日志在 `commits/vNN-U#.md`，未污染 facts
- [ ] 写前 preflight 未把 author-lore 列入读取回执

## 来源

伏笔设计｜挖坑一时爽，填坑火葬场？一张伏笔表全搞定。

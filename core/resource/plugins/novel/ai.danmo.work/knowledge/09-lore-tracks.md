# 连载轨（作者 / 读者账本）

长篇连载要把「作者手里的底牌」和「读者已经知道的事」分开，否则模型会提前揭底。

## 三轨

| 轨 | 路径 | 谁可读 | 来源 |
|----|------|--------|------|
| **author-lore** | `canon/author-lore.md` | 作者 / 规划 / 改纲 | 立项时写入；含终局细节、隐藏规则、角色私密 |
| **facts（读者账）** | `continuity/facts.md` | 写正文、审稿、续写 | **只从已 Commit 章节重建**：Public facts + Tracking（cursor / Cast snapshot）+ Open loops + 每卷一行 summaries 索引。**不存章摘要** |
| **summaries（章摘要）** | `continuity/summaries/vNN.md` | Commit 写入；续写只看上一章块 | 每章一个 `## chNNN` 块（五要素）；卷收束在末尾追加 `### vNN 卷总结`。按卷拆存只为缩小文件，不增加注入 |
| **commits（执行日志）** | `continuity/commits/vNN-U#.md` | 作者 / 回溯 | 每单元一个：gate 结果 / 四计数 / 锁词扫描 / 字数实测 / 扩写技术 / 偏离登记。**不进 facts**，避免日志污染事实层 |
| **locked-terms** | `canon/locked-terms.yaml` | gate 自动扫描 | 本卷之前不得出现的真名/真相词；`locked_until` 按卷解锁 + `compliance` 永久红线 |

`candidate` vs `canon` 管人物卡能不能进正文。三轨管 **信息对读者是否已公开** + **锁词是否提前泄漏**。

## 账本体积治理

章摘要从 Commit 起**直接写** `continuity/summaries/vNN.md`；facts 只留公开事实、cursor、Cast snapshot、未关闭 Open loops、每卷一行索引（`vNN → continuity/summaries/vNN.md`）。facts 体积 ≈ O(事实 + 人物 + 伏笔)，不随章数增长。snapshot 与 open loops 只此一份，不按卷复制。gate `postcommit` 到 `summaries/vNN.md` 核对每章块；旧书 facts 里的 `## chNNN` 块用 `novel_gate.py --action migrate` 拆出。

**执行日志不进 facts。** gate 结果、四计数、锁词扫描、字数实测、扩写技术、偏离登记 → 全部写 `continuity/commits/vNN-U#.md`。facts 只回答"读者现在知道什么"，commits 回答"我们这一轮做了什么"。

## 写正文加载

允许：**只有** gate preflight 打印的 `### CONTEXT`。它依次含：风格指纹 / 题材专有文全文（`genre`；`crime-human` 再接「刑侦人味文风」）/ 卷纲索引行 / 渲染后的单元卡 / 上一单元钩子 / `on_stage` 人物（snapshot 行 + 三锚点 + 1 条台词；`pov` 加「不知」一行；双方在场的关系行）/ 开放债务 ≤8 条 / 锁词清单。**模型不读 facts 全文、不读人物卡、不再通读细纲 YAML**——脚本抽取，模型只消费抽取结果。从账本抽出的只有三块：`on_stage` 人物的 snapshot 行、≤8 条 open loops、上一单元钩子（优先上一单元 YAML 的 `next_hook.out`，缺了才取该卷 summaries 上一章的「钩子」行）。伤口 / 弧光 / 关系编年史 / 章摘要 / 卷总结 / 共性 KB 不进。`on_stage` 为空 → CONTEXT 写「未列上场人物」并 warning（不再按名字扫场面文本猜上场，也不灌 snapshot 前几行）。

**风格指纹**随 preflight CONTEXT 注入：`canon/style-fingerprint.md`（无则退回 bible `## Style card`）由脚本压缩 ≤480 字拼进 CONTEXT 顶部。上下文被裁剪导致本轮未见指纹时，按专家规则 `read_file` 补读该文件（不要扫树）。**角色卡不全量进 CONTEXT**：只按细纲 `on_stage` 注入三锚点 + 1 条台词。

禁止：`author-lore.md`、圣经「终局储备」细节栏、未 Commit 草稿里的剧透、`continuity/summaries/` 归档全量通读。

**单元细纲阶段例外**：写细纲前允许读 facts 的 `### Cast snapshot` 小节（只读涉及角色的行）与点名人物卡的「关系」段，用于对齐 `state_deltas` 起点；仍不读 facts 全文、不读 author-lore。

## Legacy

旧书可能仍有 `continuity/ledger.md` / `public-lore.md` / `tracking.md` / `chapter_summaries.md` / `foreshadow-tracker.md`。冷启动合并进 `facts.md`（事实）+ `commits/`（执行日志分文件）后移入 `_archive/`. gate `ledger_path()` 自动 fallback 到 `ledger.md`，兼容旧书。

## Commit

每单元定稿必须：
1. `## chNNN` 摘要块写 `continuity/summaries/vNN.md`；刷新 `facts.md`（只加本单元真正公开的 + 状态差 + Open loops）；关系质态变了回写两张人物卡两列。
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
- [ ] facts 无 `## chNNN` 块；章摘要在 `continuity/summaries/vNN.md`，卷总结在该文件末尾
- [ ] 执行日志在 `commits/vNN-U#.md`，未污染 facts
- [ ] 写前 preflight 未把 author-lore 列入读取回执

## 来源

伏笔设计｜挖坑一时爽，填坑火葬场？一张伏笔表全搞定。

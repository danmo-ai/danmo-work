# 连载三轨（作者 / 读者 / 追踪）

长篇连载要把「作者手里的底牌」和「读者已经知道的事」分开，否则模型会提前揭底。

## 三轨

| 轨 | 路径 | 谁可读 | 来源 |
|----|------|--------|------|
| **author-lore** | `canon/author-lore.md` | 作者 / 规划 / 改纲 | 立项时写入；含终局细节、隐藏规则、角色私密 |
| **public-lore** | `continuity/public-lore.md` | 写正文、审稿 | **只从已 Commit 章节重建** |
| **tracking** | `continuity/tracking.md` | 写正文、审稿、续写 | 已 Commit 章节的当前状态（位置/知情/开放钩） |

`candidate` vs `canon` 管人物卡能不能进正文。三轨管 **信息对读者是否已公开**。Canon 人物卡仍可含伤口；未揭的真相应下沉到 author-lore，正文只写 public-lore 允许的信息。

## 写正文加载

允许：`public-lore.md` + `tracking.md`（短）+ 本章合同 + 上场 `canon/cast` 的公开段 + 近 3 章摘要。

禁止：`author-lore.md`、圣经「终局储备」细节栏、未 Commit 草稿里的剧透。

章合同 `forbidden` 从卷纲「禁止提前释放」下推，作为写章红线；不要为了核对 forbidden 而把真名读进上下文。

## 事实类型（public-lore）

| Kind | 含义 |
|------|------|
| shown_fact | 正文明确展示或确认 |
| reader_inference | 读者可推断，正文未确认 |
| character_claim | 角色说过或相信 |
| rumor | 流言 |
| misdirection | 有意误导 |

## Commit

每章定稿必须刷新 public-lore（只加本章真正公开的）和 tracking（状态差）。Gate 脚本 `--action postcommit` 检查这两份文件在盘上。

## Checklist

- [ ] 正文未使用 author-lore 中未到解锁卷的底牌
- [ ] public-lore 没有作者侧真相
- [ ] tracking 开放债务 ≤5
- [ ] 写前 gate 脚本 preflight 未把 author-lore 列入读取回执

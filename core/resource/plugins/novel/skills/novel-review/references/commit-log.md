# Commit log — vNN-U#（模板）

**本文件是本单元的执行日志，不写读者事实。** 读者事实在 `continuity/facts.md`。
每单元一个文件，命名 `continuity/commits/vNN-U#.md`。Commit 时新建，不追加到 facts。

---

# Commit log — vNN-U# {{单元名}}

- 单元 ID: vNN-U#
- 章范围: chNNN–chNNN
- Commit 时间: YYYY-MM-DD
- 执笔模型: <首稿模型> / 评审模型: <评审模型>（如分模）
- 细纲 status: reviewed

## Gate 结果

| 阶段 | VERDICT | 备注 |
|------|---------|------|
| preflight | PASS / FAIL | （失败时记原因，不重跑日志） |
| scan-deslop | PASS / FAIL | |
| precommit | PASS / FAIL | |
| postcommit | PASS / FAIL | |

## 反 AI 四计数（来自 gate ### COUNTS）

- em_dash_count:
- ai_vocab_count:
- english_leak_count:
- simile_count:
- （阈值以 `novel_gate.py` 常量为准；超阈值 FAIL）

## 锁词扫描（来自 precommit）

- locked_until v{NN} 命中数: 0（命中即 FAIL）
- compliance 命中数: 0
- 命中词（若有）: （位置 + 词）

## 字数实测

- 单元 runes: （vs word_floor / word_ceiling）
- 各章 runes: chNNN= / chNNN= / chNNN= / …（vs word_share 2000–3500）

## 扩写技术（若字数不足用了扩写）

- 使用技术（≤3 种/章）：
  - [ ] 场景细节充实
  - [ ] 非视觉感官（嗅/听/触/味）
  - [ ] 对白潜台词
  - [ ] 内心独白（POV 内）
- 各章扩写前后 runes: chNNN: → / chNNN: →

## Deslop 处置（若跑了 scan-deslop）

- 删除的 AI 味句式:
- 替换的比喻:
- 残留 WARN（可接受）:

## 偏离登记（正文与细纲不一致时）

| 细纲原计划 | 正文实际 | 处置 |
|------------|----------|------|
| | | 回写 yaml / 升级用户裁决 / 登记 facts.cannot_rewind |

**规则：**
- 小偏离（章位微调、场面增减）：同一 patch 回写 yaml，不留登记。
- 大偏离（情节走向反转、角色关系质变）：登记本表 + 升级用户裁决，不擅自改细纲。
- 禁止"只登记不处置"无限挂账——每偏离必须指派下一单元或卷末收束时处理。

## 下一单元指引（一句话）

vNN-U# 末章 next_hook.out 是「{{具体事件}}」，下一单元 U#+1 的 entry 应承接此钩；plan 层需注意 {{注意点}}。

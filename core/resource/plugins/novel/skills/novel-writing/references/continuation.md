# Continuation engine（续写 / 接手 / 卡文）

用户要续写、接手外稿、卡文救援、旧坑回填时加载本文件。

## CP1 — 反向解析（DNA）

1. 扫描已有正文、大纲、`canon/`、`continuity/`、`reviews/`（若有）。
2. `write` `canon/style-fingerprint.md`（模板 `style-fingerprint.md`）：句式、禁语、POV、对话占比、章末习惯。
3. 提取作品 DNA：读者承诺、主冲突、金手指边界、活跃伏笔、人物状态快照。
4. `novel-state.yaml` 设 `continuation_mode: true`；`stage: continuation`。

## CP2 — 卡点诊断

给出 **3 条可执行路线**（换钩、降维、回收伏笔、插支线等），`ask_user` 选一条。

## CP3 — Frozen_Canon 确认

1. `write` `continuity/frozen-canon.md`：不可改动的设定、已发生事实、文风指纹引用。
2. **未经 `ask_user` 确认 Frozen_Canon → 禁止进入批次细纲与正文**。
3. 确认后 `memory_update` project checkpoint。

## 试写

首次续写：写 **500–1000 字试写**，用户确认文风后再批量。

## 后续流程

复用 `batch-freeze.md` → `chapter-contract.md` → `chapter-write.md` → `review-gates.md` → `continuity-commit.md`。

## Human stops

- Frozen_Canon 未确认
- 试写未确认
- 发现外稿与 canon 硬冲突未裁决

## Done when

- `style-fingerprint.md` + `frozen-canon.md` 存在（续写项目）
- `continuation_mode` 与 `stage` 已更新
- 用户已确认试写或明确跳过

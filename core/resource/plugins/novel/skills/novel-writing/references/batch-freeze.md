# Batch freeze（批次细纲冻结）

卷纲完成后，**批量写正文前**必须冻结一批章（通常 3–8 章）。

## 何时触发

- 用户要连续写 >1 章
- `novel-state.yaml` 无 `frozen_batch` 且 `artifacts.batch_freeze` ≠ `frozen`
- 工作台 stage = `batch_freeze`

## 产物

1. `continuity/batch-freeze.yaml`（模板 `assets/templates/batch-freeze.yaml`）
2. 可选：`outline/batch-chNNN-N.md` 细纲（每章：功能、阻碍、状态变化、伏笔动作、钩）

## 流程

1. 读当前卷纲 `outline/volumes/vNN.md` 章纲表 + `continuity/foreshadow-tracker.md`。
2. 生成批次范围 `from`–`to` 章细纲。
3. **硬逻辑审核**（时间、空间、人物、规则、资源、伏笔）— 硬矛盾必须关闭。
4. `ask_user` 裁决 → 写入 `frozen_chapters` 列表，`status: frozen`。
5. 更新 `novel-state.yaml`：

```yaml
frozen_batch:
  from: 1
  to: 8
  frozen_at: "YYYY-MM-DD"
artifacts:
  batch_freeze: frozen
stage: chapter_loop
```

## 门控

- `status: frozen` 前：**禁止** `chapter-write.md` 批量正文。
- 单章模式：用户明确只写一章时可 bypass（`ask_user` 留痕）。

## 解冻

大纲/细纲结构性修改 → 评估影响 → 解冻受影响批次 → 重新走本流程。

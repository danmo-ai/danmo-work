# Batch freeze（批次冻结）

卷纲完成后，**批量写正文前**必须冻结一批已确认的章合同（通常 3–8 章）。

## 何时触发

- 用户要连续写 >1 章
- `novel-state.yaml` 无 `frozen_batch` 且 `artifacts.batch_freeze` ≠ `frozen`
- 工作台准备批量写正文

## 产物

1. 该批每章一份 `chapters/chNNN-contract.yaml`（`status=accepted`）
2. `continuity/batch-freeze.yaml`（模板 `assets/templates/batch-freeze.yaml`）— 只记范围与状态，**不重复** purpose / 爽点 / 钩子

禁止再写 `outline/batch-*.md` 或把章合同字段抄进冻结文件。

## 流程

1. 读当前卷纲 `outline/volumes/vNN.md`（剧情单元卡 + 节奏锚点 + 反转）+ `continuity/foreshadow-tracker.md`。无单元卡则先补卷纲。
2. 为 `from`–`to` 写或补齐章合同（`chapter-contract.md`）；每章 `unit_id` 必须指向本卷一个单元，且按该卡节拍定位本章角色。
3. **连续 3 章 `pleasure_point` 为空必须重排**后再冻结。任一章 `unit_id` 空或对不上 → 先补卷纲。
4. **硬逻辑审核**（时间、空间、人物、规则、资源、伏笔）— 硬矛盾必须关闭。
5. `ask_user` 裁决 → 写入 `frozen_chapters` 列表，`status: frozen`。
6. 更新 `novel-state.yaml`：

```yaml
frozen_batch:
  from: 1
  to: 8
  frozen_at: "YYYY-MM-DD"
artifacts:
  batch_freeze: frozen
stage: writing
```

## 门控

- `status: frozen` 前：**禁止** `chapter-write.md` 批量正文。
- 单章模式：用户明确只写一章时可 bypass（`ask_user` 留痕），仍须该章合同 `accepted`。

## 解冻

总纲/卷纲或已冻结章合同结构性修改 → 评估影响 → 解冻受影响批次 → 重新走本流程。

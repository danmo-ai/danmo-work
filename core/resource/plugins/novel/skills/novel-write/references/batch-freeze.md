# Batch freeze（批次冻结）

卷纲完成后，**批量写正文前**必须冻结一批已确认的章合同（通常 3–8 章）。

## 何时触发

- 用户要连续写 >1 章
- `novel-state.yaml` 无有效 `frozen_batch` 且 `artifacts.batch_freeze` ≠ `frozen`

## 产物

1. 该批每章一份 `chapters/chNNN-contract.yaml`（`status=accepted`）
2. **仅**更新 `novel-state.yaml` 的 `frozen_batch` + `artifacts.batch_freeze` — **不再**写 `continuity/batch-freeze.yaml`

## 流程

1. 读当前卷纲 `outline/volumes/vNN.md` + `continuity/ledger.md` Open loops. 无单元卡则先补卷纲.
2. 为 `from`–`to` 写或补齐章合同；每章 `unit_id` 必须指向本卷一个单元.
3. **连续 3 章 `pleasure_point` 为空必须重排**.
4. 硬逻辑审核 — 硬矛盾必须关闭.
5. `ask_user` 裁决 → 写入 state:

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

- `artifacts.batch_freeze: frozen` 前：**禁止**批量正文.
- 单章模式：用户明确只写一章时可 bypass（`ask_user` 留痕），仍须该章合同 `accepted`.

## 解冻

总纲/卷纲或已冻结章合同结构性修改 → 评估影响 → 清/改 `frozen_batch` → 重新走本流程.

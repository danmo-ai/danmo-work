# Preflight（写前预检 + 读取回执）

每章写/审/Commit 前执行。日更只加载最小集。

**先跑 gate 脚本 `--action preflight --chapter N`（`novel-setup/references/gate.md`）。exit ≠ 0 → 停止，不要写正文。** 接手旧书或冷启动另跑 `--action doctor`。

## 日更最小加载

**必读（上限：这些文件本身，禁止扩读）：**

1. `novel-state.yaml` — stage、`frozen_batch`、`gates`、`blockers`、`qc_profile`、`last_preflight`.
2. 本章 `chapters/chNNN-contract.yaml`（写正文时须 `accepted`；`unit_id` 非空且能在本卷纲剧情单元卡对上）.
3. `continuity/ledger.md` — 尾部：Public facts + Tracking + Open loops + 近 3 章 `## chNNN` 块（不要整本）.  
   Legacy：若无 ledger，读 `public-lore.md` + `tracking.md` + `chapter_summaries.md` 近 3 章.

**按需：**

4. 本章 `beats` 点名的 `canon/cast/*.md`（asset_gate）。未上场不读.
5. `search_kb` 本阶段 **至多 1** 个主题（knowledge_gate）. **ch001–ch003** 用「节奏与结构」并 `read_skill` `opening-chapters.md`.

**禁止本轮加载：** `canon/author-lore.md`、整本 `book-bible.md` 终局细节、全卷纲正文、全书 `canon/` 通读、已提交章全文（除非合同 `continuity_risks` 点名）.

**不做：** 为走流程强制 `table_query` / `table_upsert`；文件为权威.

## 读取回执

写入 `novel-state.yaml` 的 `last_preflight` 一行（**不**另开 `preflight-log.md`）：

```text
[YYYY-MM-DD chNNN] state:writing | contract:accepted | ledger | cast:3 | gate:PASS
```

## 更新 novel-state gates

```yaml
gates:
  knowledge: pass|fail|unknown
  asset: pass|fail|unknown
  qc: unknown
blockers: []
last_preflight: "[...]"
```

## 阻断

Gate 脚本 preflight FAIL、缺少必读文件、合同非 `accepted`、`unit_id` 空或对不上卷纲单元、Frozen_Canon 空（续写）、批次未冻结（批量写）→ **停止**.

# Agent Table Store 落地方案

> 目标：为 Agent / Automation / Skill 提供 **轻量 schema-free 业务表**，支撑「每日邮件摘要」等可查询流水数据；与 Memory / Files / Knowledge 严格分家；物理隔离避免拖慢引擎核心。  
> 参考：LangGraph Store（namespace + key + JSON）、SQLite JSON1 document store、OpenClaw control/data plane 分库。  
> 范围：Core tool + 独立 `store.db` + 配额；MVP 不做自由 SQL、向量检索、独立 UI 大盘。

---

## 1. 结论与原则

| 决策 | 说明 |
|------|------|
| **做 schema-free table store** | `table → row_key → JSON`；Agent 不碰 DDL / SQL |
| **与 Memory 分家** | Memory = 偏好/约定；Table = 业务流水/可过滤记录 |
| **物理分库** | `store.db` 独立于 `work.db`，避免 automation 灌库拖慢 turn/SSE |
| **硬配额** | 单次写入、单表行数、单行大小、query limit 一律硬顶 |
| **工具返回摘要** | upsert/query 不把整表灌回 LLM context |

原则：

1. **对 Agent schema-free，对系统可索引**：value 自由；热字段后续用 `json_extract` VIRTUAL 列，不改 tool 契约。
2. **控制面 / 数据面分离**：引擎热路径（session、stream_events、turns）永不与大批量 store 写同库同事务。
3. **默认拒绝爆炸写入**：超配额返回明确错误，引导 Agent 分页或落文件。
4. **scope 由 runtime 注入**：禁止伪造其他 project/agent 的 `scope_id`（对齐 memory）。

---

## 2. 边界：放什么 / 不放什么

| 存储 | 用途 | 示例 |
|------|------|------|
| **Memory** | 长期偏好、约定、稳定事实 | 「用户偏好中文」「API 用 REST」 |
| **Files** | 长文产物、可版本管理文档 | 完整研究报告、导出 Markdown |
| **Knowledge** | 人工维护的检索文档 | 产品手册 |
| **Table Store（本方案）** | 可查询业务流水 | 每日邮件摘要、运行计数、去重游标 |
| **Turn log** | 执行轨迹 | tool_call 历史（Agent 不直接写） |

**勿存进 Table Store**：密钥、大文件二进制、可再读的仓库源码、应进 Memory 的偏好、应进文件的长文报告全文（可存摘要 + 文件路径）。

---

## 3. 数据模型

### 3.1 逻辑模型（对齐 LangGraph Store）

```text
(scope, scope_id, table, key) → JSON data
```

| 字段 | 含义 |
|------|------|
| `scope` | `user` \| `project` \| `agent` |
| `scope_id` | runtime 注入：`default` / `ProjectID` / `Agent.ID` |
| `table` | 逻辑表名，如 `email_digests`（`[a-z][a-z0-9_]{0,63}`） |
| `key` | 行键，如 `2026-07-26`（稳定、可幂等 upsert） |
| `data` | JSON object（schema-free） |
| `created_at` / `updated_at` | RFC3339 UTC |

邮件摘要示例：

```json
{
  "scope": "project",
  "table": "email_digests",
  "key": "2026-07-26",
  "data": {
    "date": "2026-07-26",
    "count": 42,
    "summary": "...",
    "highlights": ["..."],
    "sources": ["imap://..."]
  }
}
```

### 3.2 物理存储（独立库）

路径：`~/.danmo-work/store.db`（env：`WORK_STORE_DB_PATH`，与 `WORK_DB_PATH` 并列）。

```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;

CREATE TABLE store_rows (
  scope      TEXT NOT NULL,
  scope_id   TEXT NOT NULL,
  table_name TEXT NOT NULL,
  row_key    TEXT NOT NULL,
  data       TEXT NOT NULL CHECK (json_valid(data) AND json_type(data) = 'object'),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (scope, scope_id, table_name, row_key)
);

CREATE INDEX idx_store_rows_list
  ON store_rows (scope, scope_id, table_name, updated_at DESC);

-- P1：按需加热字段（不改 Agent API）
-- ALTER TABLE store_rows ADD COLUMN date TEXT
--   GENERATED ALWAYS AS (json_extract(data, '$.date')) VIRTUAL;
-- CREATE INDEX idx_store_rows_date ON store_rows (scope, scope_id, table_name, date);
```

**禁止**把 `store_rows` 放进 `work.db`。

---

## 4. Agent Tool 契约

Layer：**Core**（始终挂载，与 `memory_*` 同级）。Risk：`low`（本地数据面；无外发）。

| Tool | 作用 |
|------|------|
| `table_upsert` | 按 key 写入/覆盖一行；可选 `mode=merge` 浅合并顶层字段 |
| `table_get` | 精确取一行 |
| `table_query` | 列表/过滤；强制 limit |
| `table_delete` | 删一行；可选 `table` 清空需二次确认参数（MVP 可只支持按 key 删） |
| `table_list` | 列出当前可见 scope 下的 table 名及行数（轻量） |

### 4.1 Schema（概念）

```text
table_upsert(scope, table, key, data, mode?=replace|merge)
table_get(scope, table, key)
table_query(scope, table, filter?, order?=updated_at_desc, limit?=50, offset?=0)
table_delete(scope, table, key)
table_list(scope?)
```

`filter`（MVP）：仅支持顶层字段 **equality**，如 `{"date":"2026-07-26"}` → `json_extract(data,'$.date') = ?`。不做嵌套路径、不做 range（P1 再加 `gte`/`lte` 白名单）。

### 4.2 Tool 返回（防 context 膨胀）

```json
// upsert
{ "ok": true, "table": "email_digests", "key": "2026-07-26", "bytes": 512, "mode": "replace" }

// query
{ "table": "email_digests", "count": 7, "truncated": false, "rows": [ { "key": "...", "data": {}, "updated_at": "..." } ] }
```

`table_query` 的 `rows[].data` 受 `runtime.table.max_row_chars` 截断；超限字段替换为 `"...(truncated)"`。整次 tool result 仍走现有 `runtime.tools.max_output_chars`。

### 4.3 Policy 文案（system / tool description）

固定块 `<table-store-policy>`（类比 `<memory-policy>`）：

- 用 Table 存流水、日报、游标、计数；用 Memory 存偏好；用 Files 存长文
- 使用稳定 `key` 做幂等（日期、外部 ID）
- 大批量：分页 upsert；单 turn 勿灌入上千行
- 勿存 secrets

---

## 5. 隔离与配额

### 5.1 存储隔离

| 路径 | 职责 |
|------|------|
| `work.db` | 控制面：agents、sessions、turns、stream_events、memories、automations… |
| `store.db` | 数据面：仅 `store_rows`（及后续 meta） |

实现要点：

- 独立 `gorm.Open` / 独立连接；`SetMaxOpenConns(1)` 或小连接池（SQLite 习惯）
- WAL + `busy_timeout`
- Store 写入 **永不** 与 `stream_events` / turn 更新同事务
- Bootstrap：`NewStoreDB` + `port.TableStoreRepo` 注入 runtime tools

### 5.2 硬配额（config 可调，默认如下）

```yaml
runtime:
  table:
    max_rows_per_upsert: 50        # 单次 tool call；MVP 每次 1 行也可，预留 batch
    max_rows_per_turn: 200
    max_rows_per_table: 50000      # 超限拒绝新写入（可配 archive 策略于 P1）
    max_row_bytes: 65536           # 单行 JSON 序列化后
    max_tables_per_scope: 100
    query_default_limit: 50
    query_max_limit: 200
    max_row_chars: 8000            # 返回给 LLM 时单行展示上限
```

超限错误格式稳定、可机读：

```text
table_store_quota_exceeded: max_rows_per_table=50000 table=email_digests
```

### 5.3 运行时隔离

| 机制 | 说明 |
|------|------|
| Turn 计数器 | `TurnContext` 累计本 turn `table_upsert` 行数 |
| 摘要返回 | 禁止把 query 全量当「顺便 echo」 |
| Automation | 可与交互 session 并行；争用只发生在 `store.db` 写锁，不影响 `work.db` 热路径 |
| 大导入 | MVP 不提供；P1 `table_import` 读项目内 JSONL，后台批次写入 |

---

## 6. 代码落点（对齐现有分层）

| 层 | 路径 | 工作 |
|----|------|------|
| Domain | `core/domain/table_store.go` | `TableRow`、`TableQuery`、`TableScope` |
| Config | `core/domain/config.go` | `ConfigTableSection` + defaults |
| Paths | `core/paths/home.go` | `StoreDatabaseFile()` / `WORK_STORE_DB_PATH` |
| Port | `core/port/store.go` | `TableStoreRepo` |
| Store | `core/store/tablestore/sqlite.go` | 独立 DB 实现（**不要**塞进 `core/store/sqlite/store.go` 的 AutoMigrate） |
| Tools | `core/runtime/tool/builtin/table_store.go` | 五个 tool handler |
| Prompt | `core/runtime/prompt_builder.go` | `<table-store-policy>` |
| Bootstrap | `core/bootstrap/bootstrap.go` | 打开 store.db、注册 Core tools |
| Session | `core/runtime/session_runner.go` | Core registry 挂载；注入 scope_id；turn 配额 |
| API（可选 MVP） | `server/api/v1/` | `GET/DELETE /api/v1/table/{table}/rows` 便于调试 |
| UI（P1） | `frontend` 右侧 Tab 或 Memory 旁「Tables」 | 浏览/删除；非 MVP 阻塞项 |
| Docs | `docs/core-design.md` §7.x | 实现后补一节，指向本文 |

测试：

- `core/store/tablestore/*_test.go`：upsert/get/query/filter/quota/isolation path
- `core/runtime/tool/builtin/table_store_test.go`：scope 注入、伪造 scope_id 拒绝、turn 配额
- 集成：automation 写 1k 行时，并行 session 的 `stream_events` 写入延迟不明显（基准测试可选）

---

## 7. 与 Automation / Skill 的用法约定

### 7.1 每日邮件摘要（标杆场景）

1. Schedule automation 每日触发 session turn  
2. Agent 拉邮件 → 总结  
3. `table_upsert(scope=project, table=email_digests, key=YYYY-MM-DD, data=...)`  
4. 用户问「上周摘要」→ `table_query` + filter/limit  
5. 需要对外分享 → `write` 导出 Markdown（文件），Table 只留结构化行

### 7.2 Skill 指引（可选内置片段）

在相关 skill 中写明：

- 幂等 key 规范
- 勿把全文塞进一行；长文写文件，table 存指针
- 查询先 `table_list` / 小 limit，再按需 `table_get`

---

## 8. 分阶段交付

### MVP（本方案必须完成）

- [ ] `store.db` + `TableStoreRepo`
- [ ] Core tools：`table_upsert` / `table_get` / `table_query` / `table_delete` / `table_list`
- [ ] scope 注入 + 配额 + policy 文案
- [ ] 单元测试覆盖 CRUD / quota / scope
- [ ] `docs/core-design.md` 增加简短索引节
- [ ] 调试 API：list/get/delete rows（可无前端）

### P1

- [ ] `filter` 支持白名单 range（`date_gte` / `date_lte`）或 generated column `date`
- [ ] `table_upsert` batch（单 call 多行，仍受 per-call 上限）
- [ ] 表满时 `table_archive` → 项目目录 JSONL，并删热点行
- [ ] 右侧工作区 Tables 面板
- [ ] 异步 `table_import`（JSONL）

### 明确不做（除非新需求）

- 自由 SQL / DDL tool
- 多表 join、聚合 DSL
- 向量 / FTS（走 Memory 或 Knowledge）
- 把 Table 自动注入 system prompt
- 云端同步多租户数据库

---

## 9. 风险与对策

| 风险 | 对策 |
|------|------|
| 与 Memory 职责混淆 | policy + tool description 强约束；UI/文档对照表 |
| Agent 写爆单表 | `max_rows_per_table` 硬拒绝；P1 归档 |
| query 撑爆 context | `query_max_limit` + `max_row_chars` + 全局 tool output cap |
| SQLite 锁拖慢聊天 | **分库**；验证项：压测 store 写入时交互 turn 延迟 |
| 恶意/错误 scope | runtime 覆盖 `scope_id`，与 memory 同一模式 |
| merge 语义不清 | MVP 默认 `replace`；`merge` 仅顶层浅合并并写进 schema |

---

## 10. 验收标准

1. Automation 连续写入 ≥1000 行邮件摘要后，`table_query` 能按日期取回；交互 session 发送消息到首条 stream event 的延迟相对基线无明显恶化（同机、WAL 开启）。  
2. 超过 `max_rows_per_table` 时 tool 返回配额错误，不写入。  
3. Agent 无法写入其他 `project` / `agent` 的 scope_id。  
4. Memory 回归：现有 `memory_*` 行为与测试不变。  
5. `make test` 通过；新增 table store 单测绿色。

---

## 11. 实现顺序（建议工程切片）

```text
1. paths + config defaults + domain/port
2. core/store/tablestore (独立 DB) + 单测
3. builtin tools + turn quota + prompt policy
4. bootstrap 注册 Core tools
5. 调试 HTTP API
6. core-design 索引 + 示例 skill 片段（可选）
7. （P1）UI / archive / batch
```

切片 1–5 为可合并的一个 PR；UI 与 archive 另开 PR。

---

*方案版本：v1.0（2026-07-26）*

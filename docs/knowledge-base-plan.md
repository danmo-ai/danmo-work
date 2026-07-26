# Knowledge Base 落地方案

> 目标：打通人工知识库的 **Markdown 真相源 + 管理 UI + Agent 绑定**，并以 **章节切分 + SQLite FTS5（BM25）** 支撑 `search_kb` / 自动 top-K 注入。  
> 参考：tiersum 冷路径章节切分与混合检索算法；产品边界对齐 Memory / Table Store / Files（见 [agent-table-store-plan.md](./agent-table-store-plan.md)）。  
> 范围：P0 作者面 + P1 章级 BM25；P2 向量混合默认关闭。

---

## 1. 结论与原则

| 决策 | 说明 |
|------|------|
| **MD 文件为文档 SoT** | `~/.danmo-work/knowledge/<kbId>/<docId>.md`；元数据在 `work.db` |
| **索引单元 = 章** | 借 tiersum 标题树切分；非整篇 `Contains` |
| **P1 检索 = FTS5 BM25** | 零新原生依赖；中文用 CJK bigram 增强 |
| **P2 向量混合默认关** | 可选简单确定性 embedding + 与 BM25 0.5/0.5 融合 |
| **与 Memory / Table 分家** | Knowledge = 人工文档；不自动进 Memory |

原则：

1. **可检视**：用户可在 UI 或磁盘编辑 Markdown；Agent 检索结果带章节 path。
2. **写后即索引**：文档保存/删除同步重建该 doc 的章节行。
3. **轻依赖**：不做 Bleve / HNSW / ONNX；不做 tiersum hot 路径。

---

## 2. 边界

| 存储 | 用途 |
|------|------|
| **Knowledge** | 人工维护手册/规范，绑定 Agent，`search_kb` |
| **Memory** | Agent 显式写入的偏好/事实 |
| **Table Store** | 业务流水 |
| **Files / Document Stage** | 项目内产物文档（非 KB 目录） |

**非目标**：云端向量库、多租户 RAG、LLM 章节分析、冷热 promote。

---

## 3. 数据模型

### 3.1 逻辑

```text
KnowledgeBase { id, name, description, documentCount, createdAt, updatedAt }
KnowledgeDoc  { id, kbId, title, path, content?, createdAt, updatedAt }
KnowledgeChapter { path, kbId, docId, title, content, embedding? }
```

Agent 绑定：`Agent.KnowledgeIDs` = KB id 列表。

### 3.2 物理

- 文件：`~/.danmo-work/knowledge/<kbId>/<docId>.md`（可选 YAML frontmatter `title`）
- 表：`knowledge_bases`、`knowledge_docs`（`rel_path`，正文不灌控制面）
- FTS：`knowledge_chapters_fts`（FTS5）；旁路表 `knowledge_chapters` 存 path/kb/doc + 可选 embedding JSON

---

## 4. API

| Method | Path | 说明 |
|--------|------|------|
| GET/POST | `/api/v1/knowledge/bases` | 列表 / 创建 |
| GET/PUT/DELETE | `/api/v1/knowledge/bases/:id` | 读 / 更新 / 删（级联文档+文件+索引） |
| GET/POST | `/api/v1/knowledge/bases/:id/docs` | 列表 / 新建文档 |
| GET/PUT/DELETE | `/api/v1/knowledge/docs/:docId` | 读（含 content）/ 更新 / 删 |

---

## 5. 检索

1. Markdown → ChapterSplitter（ATX 标题 + 中文编号；排除 fenced code；token 预算合并；超大叶滑窗）
2. 章行写入 FTS5；可选写入向量
3. `search_kb`：BM25（及可选向量）→ 章级 snippet，按分排序截断
4. 自动 top-K 注入：章片段，配置 `runtime.knowledge.search_top_k`
5. `list_kb_docs` / `get_kb_doc`：整篇走文件

配置：

```yaml
runtime:
  knowledge:
    search_top_k: 3
    chapter_max_tokens: 512
    vector_hybrid: false   # P2，默认关
```

---

## 6. 代码落点

| 层 | 路径 |
|----|------|
| Domain | `core/domain/knowledge.go` |
| Port / Store | `core/port/store.go`、`core/store/sqlite/knowledge.go` |
| Splitter / embed | `core/runtime/knowledge/` |
| Service | `core/service/knowledge_manager.go` |
| Tools | `core/runtime/tool/builtin/knowledge.go` |
| API | `server/api/v1/knowledge.go` |
| FE | `frontend/src/stores/knowledge.ts`、`KnowledgeBaseManagement.vue` |

---

## 7. 验收

1. UI 创建 KB / 文档后，磁盘出现 `.md`，`work.db` 有元数据。
2. Agent 绑定 KB 后 `search_kb` 返回相关**章节**（非整篇乱序截断）。
3. 自动注入为章片段；`list_kb_docs` / `get_kb_doc` 可用。
4. `vector_hybrid: false` 时无向量分支；开启后 hybrid 融合可用。
5. `make test` 通过；切分与检索单测绿色。

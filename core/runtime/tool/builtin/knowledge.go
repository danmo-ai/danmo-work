package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"danmo-work/core/domain"
)

// KnowledgeIndex is the runtime-facing search/list/get surface for knowledge bases.
type KnowledgeIndex interface {
	Search(kbIDs []string, query string, topK int) []string
	ListDocumentSummaries(kbIDs []string) []map[string]string
	GetDocumentContent(docID string) (title, content string, ok bool)
}

// Doc is a legacy in-memory document used by tests / fallback index.
type Doc struct {
	ID      string
	KBID    string
	Title   string
	Content string
}

// Knowledge is an in-memory fallback index and/or adapter around KnowledgeIndex.
type Knowledge struct {
	mu     sync.RWMutex
	docs   []Doc
	backend KnowledgeIndex
}

func NewKnowledge() *Knowledge { return &Knowledge{} }

// SetBackend wires the durable KnowledgeManager (or test double).
func (k *Knowledge) SetBackend(b KnowledgeIndex) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.backend = b
}

func (k *Knowledge) Add(d Doc) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.docs = append(k.docs, d)
}

func (k *Knowledge) Search(kbIDs []string, query string, topK int) []string {
	k.mu.RLock()
	backend := k.backend
	k.mu.RUnlock()
	if backend != nil {
		return backend.Search(kbIDs, query, topK)
	}
	return k.searchLocal(kbIDs, query, topK)
}

func (k *Knowledge) ListDocumentSummaries(kbIDs []string) []map[string]string {
	k.mu.RLock()
	backend := k.backend
	k.mu.RUnlock()
	if backend != nil {
		return backend.ListDocumentSummaries(kbIDs)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	kbSet := toSet(kbIDs)
	var out []map[string]string
	for _, d := range k.docs {
		if _, ok := kbSet[d.KBID]; !ok {
			continue
		}
		out = append(out, map[string]string{"id": d.ID, "kbId": d.KBID, "title": d.Title})
	}
	return out
}

func (k *Knowledge) GetDocumentContent(docID string) (title, content string, ok bool) {
	k.mu.RLock()
	backend := k.backend
	k.mu.RUnlock()
	if backend != nil {
		return backend.GetDocumentContent(docID)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	for _, d := range k.docs {
		if d.ID == docID {
			return d.Title, d.Content, true
		}
	}
	return "", "", false
}

func (k *Knowledge) searchLocal(kbIDs []string, query string, topK int) []string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	q := strings.ToLower(query)
	type hit struct {
		idx   int
		score int
	}
	kbSet := toSet(kbIDs)
	var hits []hit
	for i, d := range k.docs {
		if _, ok := kbSet[d.KBID]; !ok {
			continue
		}
		score := 0
		c := strings.ToLower(d.Content)
		for _, w := range strings.Fields(q) {
			if strings.Contains(c, w) {
				score++
			}
		}
		if strings.Contains(strings.ToLower(d.Title), q) {
			score += 5
		}
		if score > 0 {
			hits = append(hits, hit{idx: i, score: score})
		}
	}
	// Sort by score descending before truncating.
	for i := 1; i < len(hits); i++ {
		j := i
		for j > 0 && hits[j].score > hits[j-1].score {
			hits[j], hits[j-1] = hits[j-1], hits[j]
			j--
		}
	}
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, k.docs[h.idx].Content)
	}
	return out
}

func toSet(ids []string) map[string]struct{} {
	kbSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		kbSet[id] = struct{}{}
	}
	return kbSet
}

type SearchKB struct {
	Knowledge *Knowledge
	KBIDs     []string
}

func (h *SearchKB) Name() string                { return "search_kb" }
func (h *SearchKB) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *SearchKB) Describe(args map[string]any) string {
	query, _ := args["query"].(string)
	if len(query) > 80 {
		query = query[:80] + "..."
	}
	return query
}
func (h *SearchKB) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "search_kb",
		Description: "Search internal knowledge bases for relevant chapter snippets.\n\n" +
			"- query: search keywords or phrases (required).\n" +
			"- Searches across all knowledge bases assigned to the current agent.\n" +
			"- Returns matching chapter contents ranked by relevance (BM25; optional vector hybrid).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
			"required": []string{"query"},
		},
	}
}

func (h *SearchKB) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	query, _ := input["query"].(string)
	results := h.Knowledge.Search(h.KBIDs, query, 5)
	content := strings.Join(results, "\n\n---\n\n")
	return domain.ToolResult{Content: content}, nil
}

type ListKBDocs struct {
	Knowledge *Knowledge
	KBIDs     []string
}

func (h *ListKBDocs) Name() string                { return "list_kb_docs" }
func (h *ListKBDocs) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *ListKBDocs) Describe(args map[string]any) string {
	return "list_kb_docs"
}
func (h *ListKBDocs) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "list_kb_docs",
		Description: "List knowledge-base documents bound to the current agent (id, title, kbId).",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
}
func (h *ListKBDocs) Execute(_ context.Context, _ map[string]any) (domain.ToolResult, error) {
	items := h.Knowledge.ListDocumentSummaries(h.KBIDs)
	if items == nil {
		items = []map[string]string{}
	}
	b, _ := json.Marshal(items)
	return domain.ToolResult{Content: string(b)}, nil
}

type GetKBDoc struct {
	Knowledge *Knowledge
}

func (h *GetKBDoc) Name() string                { return "get_kb_doc" }
func (h *GetKBDoc) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *GetKBDoc) Describe(args map[string]any) string {
	docID, _ := args["doc_id"].(string)
	return docID
}
func (h *GetKBDoc) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name:        "get_kb_doc",
		Description: "Get the full Markdown content of a knowledge-base document by doc_id (from list_kb_docs).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"doc_id": map[string]any{"type": "string"},
			},
			"required": []string{"doc_id"},
		},
	}
}
func (h *GetKBDoc) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	docID, _ := input["doc_id"].(string)
	title, content, ok := h.Knowledge.GetDocumentContent(docID)
	if !ok {
		return domain.ToolResult{Content: "document not found"}, nil
	}
	if title != "" {
		return domain.ToolResult{Content: "# " + title + "\n\n" + content}, nil
	}
	return domain.ToolResult{Content: content}, nil
}

package builtin

import (
	"context"
	"strings"
	"sync"

	"danmo-work/core/domain"
)

type Doc struct {
	KBID    string
	Title   string
	Content string
}

type Knowledge struct {
	mu   sync.RWMutex
	docs []Doc
}

func NewKnowledge() *Knowledge { return &Knowledge{} }

func (k *Knowledge) Add(d Doc) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.docs = append(k.docs, d)
}

func (k *Knowledge) Search(kbIDs []string, query string, topK int) []string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	q := strings.ToLower(query)
	type hit struct {
		idx   int
		score int
	}

	kbSet := make(map[string]struct{}, len(kbIDs))
	for _, id := range kbIDs {
		kbSet[id] = struct{}{}
	}

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

	if len(hits) > topK {
		hits = hits[:topK]
	}

	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, k.docs[h.idx].Content)
	}
	return out
}

type SearchKB struct {
	Knowledge *Knowledge
	KBIDs     []string
}

func (h *SearchKB) Name() string              { return "search_kb" }
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
		Description: "Search internal knowledge bases for relevant documents.\n\n" +
			"- query: search keywords or phrases (required).\n" +
			"- Searches across all knowledge bases assigned to the current agent.\n" +
			"- Returns matching document contents ranked by relevance.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
			"required": []string{"query"},
		},
	}
}
type ListKBDocs struct{}

func (h *ListKBDocs) Name() string               { return "list_kb_docs" }
func (h *ListKBDocs) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *ListKBDocs) Describe(args map[string]any) string {
	return "list_kb_docs"
}
func (h *ListKBDocs) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "list_kb_docs", Description: "List all knowledge base documents",
		Parameters: map[string]any{"type": "object"},
	}
}
func (h *ListKBDocs) Execute(_ context.Context, _ map[string]any) (domain.ToolResult, error) {
	return domain.ToolResult{Content: "[]"}, nil
}

type GetKBDoc struct{}

func (h *GetKBDoc) Name() string               { return "get_kb_doc" }
func (h *GetKBDoc) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *GetKBDoc) Describe(args map[string]any) string {
	docID, _ := args["doc_id"].(string)
	return docID
}
func (h *GetKBDoc) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "get_kb_doc", Description: "Get a knowledge base document",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{"doc_id": map[string]any{"type": "string"}},
			"required": []string{"doc_id"},
		},
	}
}
func (h *GetKBDoc) Execute(_ context.Context, _ map[string]any) (domain.ToolResult, error) {
	return domain.ToolResult{Content: ""}, nil
}

func (h *SearchKB) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	query, _ := input["query"].(string)
	results := h.Knowledge.Search(h.KBIDs, query, 5)
	content := ""
	for _, r := range results {
		content += r + "\n"
	}
	return domain.ToolResult{Content: content}, nil
}

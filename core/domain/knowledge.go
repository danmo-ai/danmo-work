package domain

import "strings"

// KnowledgeBase is a human-curated document collection bound to agents via KnowledgeIDs.
type KnowledgeBase struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	DocumentCount int    `json:"documentCount"`
	CreatedAt     string `json:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt"`
}

// KnowledgeDoc is a single Markdown document inside a knowledge base.
// Content is the Markdown body (loaded from disk SoT when requested).
type KnowledgeDoc struct {
	ID        string `json:"id"`
	KBID      string `json:"kbId"`
	Title     string `json:"title"`
	Path      string `json:"path,omitempty"` // relative path under knowledge dir
	Content   string `json:"content,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// KnowledgeChapter is a bottom-up extracted logical chapter stored in knowledge_chapters.
type KnowledgeChapter struct {
	ID      string `json:"id"`
	KBID    string `json:"kbId"`
	DocID   string `json:"docId"`
	Title   string `json:"title"`
	Content string `json:"content"` // full chapter Markdown
}

// KnowledgeChunkHit is a ranked chunk result from BM25 / vector search.
// ID encodes the chapter path (chunkID = chapterID/01, chapterID/02, ...).
type KnowledgeChunkHit struct {
	ID     string  `json:"id"`
	Score  float64 `json:"score"`
	Source string  `json:"source,omitempty"` // bm25 | vector | hybrid
}

// ChapterIDFromChunkID extracts the logical chapter id from a chunk id.
func ChapterIDFromChunkID(chunkID string) string {
	if idx := strings.LastIndex(chunkID, "/"); idx >= 0 {
		return chunkID[:idx]
	}
	return chunkID
}

// KnowledgeChapterHit is a ranked chapter content returned by search_kb.
type KnowledgeChapterHit struct {
	ID      string  `json:"id"`
	KBID    string  `json:"kbId"`
	DocID   string  `json:"docId"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Source  string  `json:"source,omitempty"` // bm25 | vector | hybrid
}

// CreateKnowledgeBaseRequest creates an empty knowledge base.
type CreateKnowledgeBaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateKnowledgeBaseRequest updates base metadata.
type UpdateKnowledgeBaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpsertKnowledgeDocRequest creates or updates a document body.
type UpsertKnowledgeDocRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

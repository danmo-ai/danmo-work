package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"
	kb "danmo-work/core/runtime/knowledge"
)

// KnowledgeManager owns KB CRUD, Markdown SoT on disk, and chapter search.
type KnowledgeManager struct {
	bases   port.KnowledgeBaseRepo
	docs    port.KnowledgeDocRepo
	index   port.KnowledgeIndexRepo
	rootDir string
	cfg     func() domain.ConfigKnowledgeSection

	mu sync.RWMutex
}

func NewKnowledgeManager(
	bases port.KnowledgeBaseRepo,
	docs port.KnowledgeDocRepo,
	index port.KnowledgeIndexRepo,
	rootDir string,
	cfg func() domain.ConfigKnowledgeSection,
) *KnowledgeManager {
	if rootDir == "" {
		rootDir = paths.KnowledgeDir()
	}
	if cfg == nil {
		cfg = func() domain.ConfigKnowledgeSection {
			return domain.ConfigKnowledgeSection{SearchTopK: 3, ChapterMaxTokens: 512}
		}
	}
	m := &KnowledgeManager{bases: bases, docs: docs, index: index, rootDir: rootDir, cfg: cfg}
	_ = os.MkdirAll(rootDir, 0o755)
	return m
}

// ReindexAll rebuilds chapter indexes from disk (bootstrap / recovery).
func (m *KnowledgeManager) ReindexAll(ctx context.Context) error {
	docs, err := m.docs.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, d := range docs {
		content, err := m.readContent(ctx, d)
		if err != nil {
			continue
		}
		if err := m.indexDoc(ctx, d, content); err != nil {
			return err
		}
	}
	return nil
}

// DefaultKnowledgeBaseID is the stable id for the auto-created default KB.
const DefaultKnowledgeBaseID = "kb-default"

// NovelCraftKnowledgeBaseID is the stable id for the builtin novel craft KB.
const NovelCraftKnowledgeBaseID = "kb-novel-craft"

// NovelCraftSeedDoc is a Markdown document seeded into kb-novel-craft.
type NovelCraftSeedDoc struct {
	SeedKey string
	Title   string
	Content string
}

// EnsureNovelCraftKnowledge creates kb-novel-craft (if missing) and seeds documents
// that are not yet present. Existing docs are never overwritten (seed-if-missing).
func (m *KnowledgeManager) EnsureNovelCraftKnowledge(ctx context.Context, seeds []NovelCraftSeedDoc) (domain.KnowledgeBase, error) {
	b, err := m.ensureBaseWithID(ctx, NovelCraftKnowledgeBaseID, "小说创作技法", "跨书可复用的小说/网文创作技法（节奏、爽点、人设、世界观、去AI味等）")
	if err != nil {
		return domain.KnowledgeBase{}, err
	}
	existing, err := m.docs.ListByKB(ctx, b.ID)
	if err != nil {
		return domain.KnowledgeBase{}, err
	}
	byID := make(map[string]domain.KnowledgeDoc, len(existing))
	byTitle := make(map[string]domain.KnowledgeDoc, len(existing))
	for _, d := range existing {
		byID[d.ID] = d
		byTitle[strings.TrimSpace(d.Title)] = d
	}
	for _, seed := range seeds {
		seedKey := strings.TrimSpace(seed.SeedKey)
		title := strings.TrimSpace(seed.Title)
		if seedKey == "" || title == "" {
			continue
		}
		docID := "doc-novel-craft-" + seedKey
		if _, ok := byID[docID]; ok {
			continue
		}
		if _, ok := byTitle[title]; ok {
			continue
		}
		if _, err := m.createDocWithID(ctx, b.ID, docID, domain.UpsertKnowledgeDocRequest{
			Title:   title,
			Content: seed.Content,
		}); err != nil {
			return domain.KnowledgeBase{}, err
		}
	}
	b.DocumentCount, _ = m.docs.CountByKB(ctx, b.ID)
	return b, nil
}

func (m *KnowledgeManager) ensureBaseWithID(ctx context.Context, id, name, description string) (domain.KnowledgeBase, error) {
	if existing, err := m.bases.Get(ctx, id); err == nil {
		existing.DocumentCount, _ = m.docs.CountByKB(ctx, id)
		return existing, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	b := domain.KnowledgeBase{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := os.MkdirAll(filepath.Join(m.rootDir, b.ID), 0o755); err != nil {
		return domain.KnowledgeBase{}, err
	}
	if err := m.bases.Upsert(ctx, b); err != nil {
		return domain.KnowledgeBase{}, err
	}
	return b, nil
}

func (m *KnowledgeManager) createDocWithID(ctx context.Context, kbID, docID string, req domain.UpsertKnowledgeDocRequest) (domain.KnowledgeDoc, error) {
	if _, err := m.bases.Get(ctx, kbID); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return domain.KnowledgeDoc{}, fmt.Errorf("title required")
	}
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return domain.KnowledgeDoc{}, fmt.Errorf("doc id required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rel := filepath.ToSlash(filepath.Join(kbID, docID+".md"))
	d := domain.KnowledgeDoc{
		ID: docID, KBID: kbID, Title: title, Path: rel,
		Content: req.Content, CreatedAt: now, UpdatedAt: now,
	}
	if err := m.writeFile(d, req.Content); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	meta := d
	meta.Content = ""
	if err := m.docs.Upsert(ctx, meta); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	if err := m.indexDoc(ctx, d, req.Content); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	_ = m.touchBase(ctx, kbID)
	return d, nil
}

// EnsureDefaultBase creates the default knowledge base when none exist.
// If bases already exist, returns the default id when present, otherwise the first base.
func (m *KnowledgeManager) EnsureDefaultBase(ctx context.Context) (domain.KnowledgeBase, error) {
	list, err := m.bases.List(ctx)
	if err != nil {
		return domain.KnowledgeBase{}, err
	}
	if len(list) > 0 {
		for _, b := range list {
			if b.ID == DefaultKnowledgeBaseID {
				b.DocumentCount, _ = m.docs.CountByKB(ctx, b.ID)
				return b, nil
			}
		}
		b := list[0]
		b.DocumentCount, _ = m.docs.CountByKB(ctx, b.ID)
		return b, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	b := domain.KnowledgeBase{
		ID:          DefaultKnowledgeBaseID,
		Name:        "默认知识库",
		Description: "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := os.MkdirAll(filepath.Join(m.rootDir, b.ID), 0o755); err != nil {
		return domain.KnowledgeBase{}, err
	}
	if err := m.bases.Upsert(ctx, b); err != nil {
		return domain.KnowledgeBase{}, err
	}
	return b, nil
}

func (m *KnowledgeManager) ListBases(ctx context.Context) ([]domain.KnowledgeBase, error) {
	return m.bases.List(ctx)
}

func (m *KnowledgeManager) GetBase(ctx context.Context, id string) (domain.KnowledgeBase, error) {
	return m.bases.Get(ctx, id)
}

func (m *KnowledgeManager) CreateBase(ctx context.Context, req domain.CreateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.KnowledgeBase{}, fmt.Errorf("name required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	b := domain.KnowledgeBase{
		ID:          fmt.Sprintf("kb-%d", time.Now().UnixNano()),
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := os.MkdirAll(filepath.Join(m.rootDir, b.ID), 0o755); err != nil {
		return domain.KnowledgeBase{}, err
	}
	if err := m.bases.Upsert(ctx, b); err != nil {
		return domain.KnowledgeBase{}, err
	}
	return b, nil
}

func (m *KnowledgeManager) UpdateBase(ctx context.Context, id string, req domain.UpdateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	b, err := m.bases.Get(ctx, id)
	if err != nil {
		return domain.KnowledgeBase{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.KnowledgeBase{}, fmt.Errorf("name required")
	}
	b.Name = name
	b.Description = strings.TrimSpace(req.Description)
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.bases.Upsert(ctx, b); err != nil {
		return domain.KnowledgeBase{}, err
	}
	b.DocumentCount, _ = m.docs.CountByKB(ctx, id)
	return b, nil
}

func (m *KnowledgeManager) DeleteBase(ctx context.Context, id string) error {
	docs, err := m.docs.ListByKB(ctx, id)
	if err != nil {
		return err
	}
	for _, d := range docs {
		_ = m.index.DeleteByDoc(ctx, d.ID)
		_ = m.removeFile(d)
	}
	if err := m.docs.DeleteByKB(ctx, id); err != nil {
		return err
	}
	_ = m.index.DeleteByKB(ctx, id)
	_ = os.RemoveAll(filepath.Join(m.rootDir, id))
	if err := m.bases.Delete(ctx, id); err != nil {
		return err
	}
	_, _ = m.EnsureDefaultBase(ctx)
	return nil
}

func (m *KnowledgeManager) ListDocs(ctx context.Context, kbID string) ([]domain.KnowledgeDoc, error) {
	return m.docs.ListByKB(ctx, kbID)
}

func (m *KnowledgeManager) GetDoc(ctx context.Context, id string) (domain.KnowledgeDoc, error) {
	d, err := m.docs.Get(ctx, id)
	if err != nil {
		return domain.KnowledgeDoc{}, err
	}
	content, err := m.readContent(ctx, d)
	if err != nil {
		return domain.KnowledgeDoc{}, err
	}
	d.Content = content
	return d, nil
}

func (m *KnowledgeManager) CreateDoc(ctx context.Context, kbID string, req domain.UpsertKnowledgeDocRequest) (domain.KnowledgeDoc, error) {
	if _, err := m.bases.Get(ctx, kbID); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return domain.KnowledgeDoc{}, fmt.Errorf("title required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("doc-%d", time.Now().UnixNano())
	rel := filepath.ToSlash(filepath.Join(kbID, id+".md"))
	d := domain.KnowledgeDoc{
		ID: id, KBID: kbID, Title: title, Path: rel,
		Content: req.Content, CreatedAt: now, UpdatedAt: now,
	}
	if err := m.writeFile(d, req.Content); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	meta := d
	meta.Content = ""
	if err := m.docs.Upsert(ctx, meta); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	if err := m.indexDoc(ctx, d, req.Content); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	_ = m.touchBase(ctx, kbID)
	return d, nil
}

func (m *KnowledgeManager) UpdateDoc(ctx context.Context, id string, req domain.UpsertKnowledgeDocRequest) (domain.KnowledgeDoc, error) {
	d, err := m.docs.Get(ctx, id)
	if err != nil {
		return domain.KnowledgeDoc{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return domain.KnowledgeDoc{}, fmt.Errorf("title required")
	}
	d.Title = title
	d.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if d.Path == "" {
		d.Path = filepath.ToSlash(filepath.Join(d.KBID, d.ID+".md"))
	}
	if err := m.writeFile(d, req.Content); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	meta := d
	meta.Content = ""
	if err := m.docs.Upsert(ctx, meta); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	if err := m.indexDoc(ctx, d, req.Content); err != nil {
		return domain.KnowledgeDoc{}, err
	}
	_ = m.touchBase(ctx, d.KBID)
	d.Content = req.Content
	return d, nil
}

func (m *KnowledgeManager) DeleteDoc(ctx context.Context, id string) error {
	d, err := m.docs.Get(ctx, id)
	if err != nil {
		return err
	}
	_ = m.index.DeleteByDoc(ctx, id)
	_ = m.removeFile(d)
	if err := m.docs.Delete(ctx, id); err != nil {
		return err
	}
	return m.touchBase(ctx, d.KBID)
}

func (m *KnowledgeManager) touchBase(ctx context.Context, kbID string) error {
	b, err := m.bases.Get(ctx, kbID)
	if err != nil {
		return err
	}
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return m.bases.Upsert(ctx, b)
}

func (m *KnowledgeManager) absPath(d domain.KnowledgeDoc) string {
	rel := d.Path
	if rel == "" {
		rel = filepath.ToSlash(filepath.Join(d.KBID, d.ID+".md"))
	}
	return filepath.Join(m.rootDir, filepath.FromSlash(rel))
}

func (m *KnowledgeManager) writeFile(d domain.KnowledgeDoc, content string) error {
	path := m.absPath(d)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := content
	if !strings.HasPrefix(strings.TrimSpace(body), "---") {
		title := strings.TrimSpace(d.Title)
		if title != "" {
			body = "---\ntitle: " + title + "\n---\n\n" + strings.TrimLeft(content, "\n")
		}
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func (m *KnowledgeManager) removeFile(d domain.KnowledgeDoc) error {
	return os.Remove(m.absPath(d))
}

func (m *KnowledgeManager) readContent(ctx context.Context, d domain.KnowledgeDoc) (string, error) {
	_ = ctx
	path := m.absPath(d)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read knowledge doc %s: %w", d.ID, err)
	}
	return string(b), nil
}

func (m *KnowledgeManager) indexDoc(ctx context.Context, d domain.KnowledgeDoc, content string) error {
	cfg := m.cfg()
	maxTok := cfg.ChapterMaxTokens
	if maxTok <= 0 {
		maxTok = 512
	}
	chapters := kb.SplitMarkdown(d.ID, d.Title, content, maxTok)
	rows := make([]port.KnowledgeChapter, 0, len(chapters))
	for _, ch := range chapters {
		row := port.KnowledgeChapter{
			Path: ch.Path, KBID: d.KBID, DocID: d.ID,
			Title: ch.Title, Content: ch.Text,
		}
		if cfg.VectorHybrid {
			row.Embedding = kb.SimpleEmbed(ch.Title + "\n" + ch.Text)
		}
		rows = append(rows, row)
	}
	return m.index.ReplaceDocChapters(ctx, d.ID, rows)
}

// Search returns ranked chapter snippet strings for prompt injection / search_kb.
func (m *KnowledgeManager) Search(kbIDs []string, query string, topK int) []string {
	hits := m.SearchHits(context.Background(), kbIDs, query, topK)
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		snippet := h.Content
		if len(snippet) > 4000 {
			snippet = snippet[:4000] + "…"
		}
		out = append(out, fmt.Sprintf("[%s] %s\n%s", h.Path, h.Title, snippet))
	}
	return out
}

// SearchHits runs BM25 (+ optional vector hybrid) over bound knowledge bases.
func (m *KnowledgeManager) SearchHits(ctx context.Context, kbIDs []string, query string, topK int) []domain.KnowledgeChapterHit {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if topK <= 0 {
		topK = 3
	}
	query = strings.TrimSpace(query)
	if query == "" || len(kbIDs) == 0 {
		return nil
	}
	cfg := m.cfg()
	branchLimit := topK * 3
	if branchLimit < 10 {
		branchLimit = 10
	}
	if branchLimit > 50 {
		branchLimit = 50
	}

	bm25Hits, err := m.index.SearchBM25(ctx, kbIDs, query, branchLimit)
	if err != nil {
		bm25Hits = nil
	}
	if !cfg.VectorHybrid {
		if len(bm25Hits) > topK {
			bm25Hits = bm25Hits[:topK]
		}
		return bm25Hits
	}

	qVec := kb.SimpleEmbed(query)
	vecHits, err := m.index.SearchVector(ctx, kbIDs, qVec, branchLimit)
	if err != nil {
		vecHits = nil
	}
	return mergeHybridHits(bm25Hits, vecHits, topK)
}

func mergeHybridHits(bm25, vector []domain.KnowledgeChapterHit, topK int) []domain.KnowledgeChapterHit {
	type agg struct {
		hit   domain.KnowledgeChapterHit
		score float64
	}
	byPath := map[string]*agg{}
	add := func(h domain.KnowledgeChapterHit, weight float64) {
		s := h.Score * weight
		if cur, ok := byPath[h.Path]; ok {
			cur.score += s
			if h.Source != "" && cur.hit.Source != "" && h.Source != cur.hit.Source {
				cur.hit.Source = "hybrid"
			}
			return
		}
		cp := h
		cp.Source = h.Source
		byPath[h.Path] = &agg{hit: cp, score: s}
	}
	for _, h := range bm25 {
		add(h, 0.5)
	}
	for _, h := range vector {
		add(h, 0.5)
	}
	out := make([]domain.KnowledgeChapterHit, 0, len(byPath))
	for _, a := range byPath {
		h := a.hit
		h.Score = a.score
		if h.Source == "" {
			h.Source = "hybrid"
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > topK {
		out = out[:topK]
	}
	return out
}

// ListDocumentSummaries lists docs for bound KBs (list_kb_docs).
func (m *KnowledgeManager) ListDocumentSummaries(kbIDs []string) []map[string]string {
	ctx := context.Background()
	var out []map[string]string
	for _, kbID := range kbIDs {
		docs, err := m.docs.ListByKB(ctx, kbID)
		if err != nil {
			continue
		}
		for _, d := range docs {
			out = append(out, map[string]string{
				"id": d.ID, "kbId": d.KBID, "title": d.Title, "updatedAt": d.UpdatedAt,
			})
		}
	}
	return out
}

// GetDocumentContent returns full Markdown for get_kb_doc.
func (m *KnowledgeManager) GetDocumentContent(docID string) (title, content string, ok bool) {
	d, err := m.GetDoc(context.Background(), docID)
	if err != nil {
		return "", "", false
	}
	return d.Title, d.Content, true
}

// MigrateLegacyContents writes any leftover DB content columns to disk.
func (m *KnowledgeManager) MigrateLegacyContents(ctx context.Context, legacy func(ctx context.Context, id string) (string, error), clear func(ctx context.Context, id string) error) error {
	docs, err := m.docs.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, d := range docs {
		path := m.absPath(d)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		c, err := legacy(ctx, d.ID)
		if err != nil || strings.TrimSpace(c) == "" {
			continue
		}
		if d.Path == "" {
			d.Path = filepath.ToSlash(filepath.Join(d.KBID, d.ID+".md"))
			_ = m.docs.Upsert(ctx, d)
		}
		if err := m.writeFile(d, c); err != nil {
			return err
		}
		if clear != nil {
			_ = clear(ctx, d.ID)
		}
		_ = m.indexDoc(ctx, d, c)
	}
	return nil
}

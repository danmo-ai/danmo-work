package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	pluginKBRoots []string

	indexingDone chan struct{}
	indexingOnce sync.Once
	mu           sync.RWMutex
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
	m := &KnowledgeManager{
		bases:        bases,
		docs:         docs,
		index:        index,
		rootDir:      rootDir,
		cfg:          cfg,
		indexingDone: make(chan struct{}),
	}
	_ = os.MkdirAll(rootDir, 0o755)
	return m
}

// ReindexAll rebuilds chapter indexes from disk (sync, used for backward compat and async goroutine).
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

// StartAsyncIndex kicks off async full reindex in a goroutine. Does not block.
func (m *KnowledgeManager) StartAsyncIndex(ctx context.Context) {
	m.indexingOnce.Do(func() {
		go func() {
			defer close(m.indexingDone)
			if err := m.ReindexAll(ctx); err != nil {
				log.Printf("[knowledge] async reindex error: %v", err)
			}
		}()
	})
}

// AwaitIndexReady returns a channel that closes when first async index completes.
func (m *KnowledgeManager) AwaitIndexReady() <-chan struct{} {
	return m.indexingDone
}

// IndexReady is deprecated; the index is always populated incrementally via CreateDoc/UpdateDoc.
func (m *KnowledgeManager) IndexReady() bool {
	select {
	case <-m.indexingDone:
		return true
	default:
		return false
	}
}

// SetPluginKBRoots replaces plugin knowledge root directories (batch set, called by PluginManager.Init).
func (m *KnowledgeManager) SetPluginKBRoots(roots []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginKBRoots = append([]string{}, roots...)
}

// RegisterPluginKB appends a single plugin knowledge root directory.
func (m *KnowledgeManager) RegisterPluginKB(root string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.pluginKBRoots {
		if existing == root {
			return
		}
	}
	m.pluginKBRoots = append(m.pluginKBRoots, root)
}

// UnregisterPluginKB removes a plugin knowledge root (unloads its KB from SQLite).
// The files themselves remain in the plugin directory (removed separately by PluginManager).
func (m *KnowledgeManager) UnregisterPluginKB(ctx context.Context, pluginRoot string) error {
	m.mu.Lock()
	filtered := m.pluginKBRoots[:0]
	for _, d := range m.pluginKBRoots {
		if d != pluginRoot {
			filtered = append(filtered, d)
		}
	}
	m.pluginKBRoots = filtered
	m.mu.Unlock()

	pluginName := pluginNameFromKBPath(pluginRoot)
	if pluginName == "" {
		return nil
	}
	_ = m.DeleteBase(ctx, pluginName)
	return nil
}

// ScanPluginBases discovers plugin knowledge directories and registers them as KBs in SQLite.
// One plugin = one knowledge base; kb-id is the plugin name.
// Call this before StartAsyncIndex so plugin KBs/docs are included in the reindex.
func (m *KnowledgeManager) ScanPluginBases(ctx context.Context) error {
	m.mu.RLock()
	roots := append([]string{}, m.pluginKBRoots...)
	m.mu.RUnlock()

	for _, root := range roots {
		pluginName := pluginNameFromKBPath(root)
		if pluginName == "" {
			continue
		}

		meta := readKBmeta(filepath.Join(root, "_meta.json"))
		name := meta["name"]
		if name == "" {
			name = pluginName
		}
		desc := meta["description"]

		if _, err := m.bases.Get(ctx, pluginName); err == nil {
			continue
		}

		if _, err := m.createBaseWithID(ctx, pluginName, name, desc); err != nil {
			log.Printf("[knowledge] plugin kb %q create: %v", pluginName, err)
			continue
		}

		files, err := filepath.Glob(filepath.Join(root, "*.md"))
		if err != nil {
			continue
		}
		for _, f := range files {
			fname := filepath.Base(f)
			if fname == "_meta.json" || strings.HasPrefix(fname, ".") {
				continue
			}
			docID := strings.TrimSuffix(fname, ".md")
			if _, err := m.docs.Get(ctx, docID); err == nil {
				continue
			}
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			title := extractMarkdownTitle(string(data))
			if title == "" {
				title = docID
			}
			rel := filepath.ToSlash(filepath.Join(pluginName, fname))
			now := time.Now().UTC().Format(time.RFC3339)
			d := domain.KnowledgeDoc{
				ID: docID, KBID: pluginName, Title: title, Path: rel,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := m.docs.Upsert(ctx, d); err != nil {
				log.Printf("[knowledge] plugin doc %q upsert: %v", docID, err)
			}
		}
	}
	return nil
}

func pluginNameFromKBPath(root string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(root)), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "plugins" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func readKBmeta(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func extractMarkdownTitle(content string) string {
	lines := strings.SplitN(content, "\n", 5)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func (m *KnowledgeManager) createBaseWithID(ctx context.Context, id, name, description string) (domain.KnowledgeBase, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	b := domain.KnowledgeBase{
		ID: id, Name: name, Description: description,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := m.bases.Upsert(ctx, b); err != nil {
		return domain.KnowledgeBase{}, err
	}
	return b, nil
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
	if err == nil {
		return string(b), nil
	}
	for _, root := range m.pluginKBRoots {
		pluginPath := filepath.Join(root, d.ID+".md")
		b, err := os.ReadFile(pluginPath)
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("read knowledge doc %s: %w", d.ID, err)
}

func (m *KnowledgeManager) indexDoc(ctx context.Context, d domain.KnowledgeDoc, content string) error {
	cfg := m.cfg()
	maxTok := cfg.ChapterMaxTokens
	if maxTok <= 0 {
		maxTok = 512
	}
	chapters := kb.SplitMarkdown(d.ID, d.Title, content, maxTok)

	chapterRows := make([]port.KnowledgeChapter, 0, len(chapters))
	var chunkEntries []port.KnowledgeChunkEntry
	chunkVectors := make(map[string][]float32)

	for _, ch := range chapters {
		chapterRows = append(chapterRows, port.KnowledgeChapter{
			ID: ch.Path, KBID: d.KBID, DocID: d.ID,
			Title: ch.Title, Content: ch.Text,
		})

		cks := kb.SplitChunks(ch.Path, ch.Title, ch.Text, maxTok)
		for _, ck := range cks {
			chunkEntries = append(chunkEntries, port.KnowledgeChunkEntry{
				ID: ck.ID, KBID: d.KBID, DocID: d.ID, Text: ck.Content,
			})
			if cfg.VectorHybrid {
				chunkVectors[ck.ID] = kb.SimpleEmbed(ck.Title + "\n" + ck.Content)
			}
		}
	}

	return m.index.ReplaceDocChapters(ctx, d.ID, chapterRows, chunkEntries, chunkVectors)
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
		out = append(out, fmt.Sprintf("[%s] %s\n%s", h.ID, h.Title, snippet))
	}
	return out
}

// SearchHits runs BM25 (+ optional vector hybrid) over bound knowledge bases.
// Searches at chunk level, resolves to logical chapters, and fetches full content.
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

	var vecHits []domain.KnowledgeChunkHit
	if cfg.VectorHybrid {
		qVec := kb.SimpleEmbed(query)
		vecHits, _ = m.index.SearchVector(ctx, kbIDs, qVec, branchLimit)
	}

	// Aggregate chunk hits by chapter ID, weighted equally.
	type agg struct {
		bm25   float64
		vector float64
	}
	chapterScores := make(map[string]*agg) // chapter ID → agg scores
	add := func(h domain.KnowledgeChunkHit, weight float64, isBM25 bool) {
		cid := domain.ChapterIDFromChunkID(h.ID)
		a, ok := chapterScores[cid]
		if !ok {
			a = &agg{}
			chapterScores[cid] = a
		}
		if isBM25 {
			a.bm25 = h.Score * weight
		} else {
			a.vector = h.Score * weight
		}
	}
	for _, h := range bm25Hits {
		add(h, 0.5, true)
	}
	for _, h := range vecHits {
		add(h, 0.5, false)
	}

	// Sort chapters by combined score.
	type scored struct {
		cid    string
		score  float64
		source string
	}
	var sorted []scored
	for cid, a := range chapterScores {
		source := "bm25"
		if cfg.VectorHybrid {
			if a.bm25 > 0 && a.vector > 0 {
				source = "hybrid"
			} else if a.vector > 0 {
				source = "vector"
			}
		}
		sorted = append(sorted, scored{cid: cid, score: a.bm25 + a.vector, source: source})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].score > sorted[j].score })
	if len(sorted) > topK {
		sorted = sorted[:topK]
	}

	// Batch query chapters from the persistent table.
	cids := make([]string, len(sorted))
	for i, s := range sorted {
		cids[i] = s.cid
	}
	chapters, err := m.index.GetChaptersByIDs(ctx, cids)
	if err != nil {
		return nil
	}
	chapterMap := make(map[string]domain.KnowledgeChapter, len(chapters))
	for _, ch := range chapters {
		chapterMap[ch.ID] = ch
	}

	out := make([]domain.KnowledgeChapterHit, 0, len(sorted))
	for _, s := range sorted {
		ch, ok := chapterMap[s.cid]
		if !ok {
			continue
		}
		out = append(out, domain.KnowledgeChapterHit{
			ID: ch.ID, KBID: ch.KBID, DocID: ch.DocID,
			Title: ch.Title, Content: ch.Content,
			Score: s.score, Source: s.source,
		})
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

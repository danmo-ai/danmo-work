package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
)

func TestKnowledgeManagerCRUDAndSearch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "work.db")
	st, err := sqlitestore.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "knowledge")
	mgr := service.NewKnowledgeManager(
		st.KnowledgeBases(),
		st.KnowledgeDocs(),
		st.KnowledgeIndex(),
		root,
		func() domain.ConfigKnowledgeSection {
			return domain.ConfigKnowledgeSection{
				SearchTopK:       3,
				ChapterMaxTokens: 512,
				VectorHybrid:     false,
			}
		},
	)
	ctx := context.Background()

	base, err := mgr.CreateBase(ctx, domain.CreateKnowledgeBaseRequest{
		Name: "手册", Description: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := mgr.CreateDoc(ctx, base.ID, domain.UpsertKnowledgeDocRequest{
		Title: "产品指南",
		Content: `# 产品指南

## 安装

使用 make install 安装 Danmo Work。

## 知识库

知识库支持章节检索与 Markdown 编辑。
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path == "" {
		t.Fatal("expected rel path")
	}
	abs := filepath.Join(root, filepath.FromSlash(doc.Path))
	if _, err := filepath.Abs(abs); err != nil {
		t.Fatal(err)
	}

	hits := mgr.Search([]string{base.ID}, "安装", 5)
	if len(hits) == 0 {
		t.Fatal("expected BM25 hits for 安装")
	}
	joined := strings.Join(hits, "\n")
	if !strings.Contains(joined, "安装") && !strings.Contains(joined, "make install") {
		t.Fatalf("unexpected hits: %s", joined)
	}

	// Vector hybrid path
	mgrHybrid := service.NewKnowledgeManager(
		st.KnowledgeBases(),
		st.KnowledgeDocs(),
		st.KnowledgeIndex(),
		root,
		func() domain.ConfigKnowledgeSection {
			return domain.ConfigKnowledgeSection{
				SearchTopK: 3, ChapterMaxTokens: 512, VectorHybrid: true,
			}
		},
	)
	if err := mgrHybrid.ReindexAll(ctx); err != nil {
		t.Fatal(err)
	}
	hyHits := mgrHybrid.SearchHits(ctx, []string{base.ID}, "知识库 Markdown", 5)
	if len(hyHits) == 0 {
		t.Fatal("expected hybrid hits")
	}

	got, err := mgr.GetDoc(ctx, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "make install") {
		t.Fatalf("content missing: %s", got.Content)
	}

	summaries := mgr.ListDocumentSummaries([]string{base.ID})
	if len(summaries) != 1 {
		t.Fatalf("summaries=%d", len(summaries))
	}

	if err := mgr.DeleteDoc(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	if err := mgr.DeleteBase(ctx, base.ID); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureDefaultBase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "work.db")
	st, err := sqlitestore.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	mgr := service.NewKnowledgeManager(
		st.KnowledgeBases(),
		st.KnowledgeDocs(),
		st.KnowledgeIndex(),
		filepath.Join(dir, "knowledge"),
		nil,
	)
	ctx := context.Background()

	b, err := mgr.EnsureDefaultBase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != service.DefaultKnowledgeBaseID {
		t.Fatalf("id=%s", b.ID)
	}
	again, err := mgr.EnsureDefaultBase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != b.ID {
		t.Fatalf("expected same default, got %s", again.ID)
	}
	list, err := mgr.ListBases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("bases=%d", len(list))
	}

	if err := mgr.DeleteBase(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	list, err = mgr.ListBases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != service.DefaultKnowledgeBaseID {
		t.Fatalf("expected default recreated, got %#v", list)
	}
}

func TestScanBuiltinKnowledgeDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "work.db")
	st, err := sqlitestore.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "knowledge")
	mgr := service.NewKnowledgeManager(
		st.KnowledgeBases(),
		st.KnowledgeDocs(),
		st.KnowledgeIndex(),
		root,
		nil,
	)
	ctx := context.Background()
	const kbID = "kb-novel-craft"
	kbDir := filepath.Join(root, kbID)
	if err := os.MkdirAll(kbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kbDir, "_meta.json"), []byte(
		`{"name":"小说创作技法","description":"跨书可复用的小说/网文创作技法"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	seedFiles := map[string]string{
		"01-pacing-structure.md": "# 节奏与结构\n\n黄金开篇与断章钩子。\n",
		"07-anti-ai-prose.md":    "# 去 AI 味（P0 / P1）\n\nP0 阻断套话。\n",
	}
	for name, content := range seedFiles {
		if err := os.WriteFile(filepath.Join(kbDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := mgr.ScanBuiltinKnowledgeDir(ctx, kbID, kbDir); err != nil {
		t.Fatal(err)
	}
	b, err := mgr.GetBase(ctx, kbID)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "小说创作技法" || b.Description != "跨书可复用的小说/网文创作技法" {
		t.Fatalf("base meta = %q / %q", b.Name, b.Description)
	}
	docs, err := mgr.ListDocs(ctx, kbID)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("docs=%d", len(docs))
	}
	for _, d := range docs {
		if !strings.HasPrefix(d.ID, "doc-kb-novel-craft-") {
			t.Fatalf("unexpected doc id %q", d.ID)
		}
	}
	// Seed-if-missing: second scan must not duplicate.
	if err := mgr.ScanBuiltinKnowledgeDir(ctx, kbID, kbDir); err != nil {
		t.Fatal(err)
	}
	docs, err = mgr.ListDocs(ctx, kbID)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("doc count changed on re-scan: %d", len(docs))
	}
	hits := mgr.Search([]string{kbID}, "断章钩子", 5)
	if len(hits) == 0 {
		t.Fatal("expected search hits in novel craft KB")
	}
}

package service_test

import (
	"context"
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

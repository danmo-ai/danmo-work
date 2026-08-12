package knowledge

import (
	"strings"
	"testing"
)

func TestSplitMarkdownHeadingsAndChinese(t *testing.T) {
	md := `# Product Guide

Intro text.

一、系统架构

架构概述

（一）总体设计

设计说明

## API Reference

API details
`
	chs := SplitMarkdown("doc1", "Product Guide", md, 512)
	if len(chs) == 0 {
		t.Fatal("expected chapters")
	}
	joined := ""
	for _, c := range chs {
		joined += c.Path + "\n"
	}
	if !strings.Contains(joined, "doc1/") {
		t.Fatalf("paths should be under doc1: %s", joined)
	}
	// With large budget, expect structural titles preserved somewhere.
	all := ""
	for _, c := range chs {
		all += c.Text
	}
	if !strings.Contains(all, "架构概述") && !strings.Contains(all, "API") {
		t.Fatalf("expected body content in chapters: %#v", chs)
	}
}

func TestSplitMarkdownSkipsFencedHeadings(t *testing.T) {
	md := "# Real\n\n```\n# Fake heading\n```\n\nBody after fence.\n"
	chs := SplitMarkdown("d", "Real", md, 512)
	for _, c := range chs {
		if strings.Contains(c.Path, "Fake") {
			t.Fatalf("fenced heading should not become chapter path: %s", c.Path)
		}
	}
}

func TestSplitMarkdownOversizedChapter(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("汉字内容段落重复填充用于超大叶切分测试。")
	}
	chs := SplitMarkdown("big", "Big", b.String(), 64)
	if len(chs) != 1 {
		t.Fatalf("expected 1 logical chapter, got %d", len(chs))
	}
	if !strings.Contains(chs[0].Path, "doc1") && !strings.Contains(chs[0].Path, "big") {
		t.Fatalf("expected chapter path under doc, got %s", chs[0].Path)
	}
}

func TestSplitChunks(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("汉字内容段落重复填充。")
	}
	chunks := SplitChunks("big/__root__", "Big", b.String(), 64)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if !strings.HasPrefix(c.ID, "big/__root__/") {
			t.Fatalf("chunk %d id should start with chapter path: %s", i, c.ID)
		}
		if c.Title != "Big" {
			t.Fatalf("chunk %d title mismatch: %s", i, c.Title)
		}
	}
	// small text should be 1 chunk
	small := SplitChunks("p/01", "T", "hello", 512)
	if len(small) != 1 {
		t.Fatalf("small text: expected 1 chunk, got %d", len(small))
	}
	if small[0].ID != "p/01/01" {
		t.Fatalf("unexpected chunk id: %s", small[0].ID)
	}
}

// TestSplitChunksPairs checks non-overlapping chunk boundaries.
func TestSplitChunksNoOverlap(t *testing.T) {
	text := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" // 26 chars
	// maxTokens=2 → windowRunes=8. 26/8 = 4 chunks (8,8,8,2)
	chunks := SplitChunks("p/t", "T", text, 2)
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}
	var all string
	for _, c := range chunks {
		all += c.Content
	}
	if all != text {
		t.Fatalf("reconstructed text mismatch: got %q want %q", all, text)
	}
}

func TestFTSQueryCJK(t *testing.T) {
	q := FTSQuery("系统架构")
	if q == "" || !strings.Contains(q, "系统") {
		t.Fatalf("unexpected query: %q", q)
	}
}

func TestSimpleEmbedStable(t *testing.T) {
	a := SimpleEmbed("hello 知识库")
	b := SimpleEmbed("hello 知识库")
	if len(a) != SimpleEmbeddingDim {
		t.Fatalf("dim=%d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("embed not stable")
		}
	}
	if CosineSimilarity(a, a) < 0.99 {
		t.Fatal("self similarity")
	}
}

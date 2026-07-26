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

func TestSplitMarkdownOversizedWindow(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("汉字内容段落重复填充用于超大叶切分测试。")
	}
	chs := SplitMarkdown("big", "Big", b.String(), 64)
	if len(chs) < 2 {
		t.Fatalf("expected multiple windows, got %d", len(chs))
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

package prompt

import (
	"io/fs"
	"path"
	"sort"
	"strings"
)

// NovelCraftKnowledgeBaseID is the stable id for the builtin novel craft KB.
const NovelCraftKnowledgeBaseID = "kb-novel-craft"

// NovelCraftSeedDoc is one Markdown chapter seeded into kb-novel-craft.
type NovelCraftSeedDoc struct {
	// SeedKey is a stable identity (filename without extension) used to avoid duplicates.
	SeedKey string
	Title   string
	Content string
}

// LoadNovelCraftDocs reads embedded novel-craft Markdown documents.
func LoadNovelCraftDocs() ([]NovelCraftSeedDoc, error) {
	entries, err := fs.ReadDir(NovelCraftKnowledge, "knowledge/novel-craft")
	if err != nil {
		return nil, err
	}
	var out []NovelCraftSeedDoc
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := fs.ReadFile(NovelCraftKnowledge, path.Join("knowledge/novel-craft", e.Name()))
		if err != nil {
			return nil, err
		}
		content := strings.TrimSpace(string(data)) + "\n"
		title := firstMarkdownHeading(content)
		if title == "" {
			title = strings.TrimSuffix(e.Name(), ".md")
		}
		seedKey := strings.TrimSuffix(e.Name(), ".md")
		out = append(out, NovelCraftSeedDoc{
			SeedKey: seedKey,
			Title:   title,
			Content: content,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SeedKey < out[j].SeedKey })
	return out, nil
}

func firstMarkdownHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

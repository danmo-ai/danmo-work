package home

import (
	"io/fs"
	"sort"
)

// NovelCraftKnowledgeBaseID is the stable id for the builtin novel craft KB.
const NovelCraftKnowledgeBaseID = "kb-novel-craft"

// KnowledgeDirs lists builtin knowledge base directory names under knowledge/.
func KnowledgeDirs() []string {
	entries, err := fs.ReadDir(FS, "knowledge")
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

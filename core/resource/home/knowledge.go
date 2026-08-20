package home

// NovelCraftKnowledgeBaseID is the stable id for the novel craft KB
// (shipped by the builtin "novel" plugin under ai.danmo.work/knowledge/).
const NovelCraftKnowledgeBaseID = "kb-novel-craft"

// KnowledgeDirs lists builtin knowledge base directory names under knowledge/.
// Empty after novel craft moved to the builtin novel plugin; retained for API
// compatibility with bootstrap ensureBuiltinKnowledge.
func KnowledgeDirs() []string {
	return nil
}

package home

// KnowledgeDirs lists builtin knowledge base directory names under knowledge/.
// Empty: the novel craft KB was removed together with the novel expert; the
// danmo-novel plugin ships no KB. Retained for API compatibility with
// bootstrap ensureBuiltinKnowledge.
func KnowledgeDirs() []string {
	return nil
}

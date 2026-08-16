package builtin

import (
	"os"
	"path/filepath"
	"strings"
)

// gitignoreRules is a minimal .gitignore parser for the pure-Go grep fallback
// (ripgrep handles the full syntax when present). Supports the common subset:
// comments, blank lines, trailing-slash dir patterns, leading-/ anchored
// patterns, * and ** globs. Negation (!) is not supported — the caller should
// prefer ripgrep when precise ignore semantics matter.
type gitignoreRules struct {
	root    string
	dirs    []string // patterns that match directories (with "/" suffix or bare names)
	files   []string // patterns that match file paths
	anchors []string // leading-/ anchored patterns
}

// loadGitignore reads .gitignore and .ignore from the project root.
func loadGitignore(root string) *gitignoreRules {
	r := &gitignoreRules{root: root}
	for _, name := range []string{".gitignore", ".ignore"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
			anchored := strings.HasPrefix(line, "/")
			line = strings.TrimPrefix(line, "/")
			if strings.HasSuffix(line, "/") {
				r.dirs = append(r.dirs, strings.TrimSuffix(line, "/"))
			} else if strings.Contains(line, "/") || strings.Contains(line, "*") {
				if anchored {
					r.anchors = append(r.anchors, line)
				} else {
					r.files = append(r.files, line)
				}
			} else if anchored {
				r.anchors = append(r.anchors, line)
			} else {
				// Bare name: matches files and directories with that name.
				r.files = append(r.files, line)
				r.dirs = append(r.dirs, line)
			}
		}
	}
	if len(r.dirs) == 0 && len(r.files) == 0 && len(r.anchors) == 0 {
		return nil
	}
	return r
}

func (r *gitignoreRules) rel(p string) string {
	rel, err := filepath.Rel(r.root, p)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

func (r *gitignoreRules) ignoresDir(path string) bool {
	rel := r.rel(path)
	if rel == "." || rel == "" {
		return false
	}
	for _, d := range r.dirs {
		if rel == d {
			return true
		}
		if strings.HasSuffix(rel, "/"+d) {
			return true
		}
	}
	for _, a := range r.anchors {
		if matched, _ := filepath.Match(a, rel); matched {
			return true
		}
		if matched, _ := filepath.Match(a+"/**", rel); matched {
			return true
		}
	}
	return false
}

func (r *gitignoreRules) ignoresFile(path string) bool {
	rel := r.rel(path)
	if rel == "" {
		return false
	}
	base := filepath.Base(rel)
	for _, f := range r.files {
		if f == base {
			return true
		}
		if matched, _ := filepath.Match(f, base); matched {
			return true
		}
		if matched, _ := filepath.Match(f, rel); matched {
			return true
		}
		if matched, _ := filepath.Match("**/"+f, rel); matched {
			return true
		}
	}
	for _, a := range r.anchors {
		if matched, _ := filepath.Match(a, rel); matched {
			return true
		}
	}
	return false
}

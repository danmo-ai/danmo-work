package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"danmo-work/core/domain"
)

var defaultExcludeDirs = []string{
	".git", "node_modules", "vendor", "dist", "build",
	"__pycache__", ".venv", "venv", "target", ".next",
}

type Grep struct{}

func (h *Grep) Name() string                { return "grep" }
func (h *Grep) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *Grep) Describe(args map[string]any) string {
	pattern, _ := args["pattern"].(string)
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	return "\"" + pattern + "\" in " + path
}
func (h *Grep) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "grep",
		Description: "Fast content search tool that searches file contents using regular expressions.\n\n" +
			"**Important**: Searches within the project root directory by default. Use relative paths when specifying a search location.\n\n" +
			"- Supports full regex syntax (e.g. \"log.*Error\", \"function\\s+\\w+\").\n" +
			"- Returns file paths, line numbers, and matching lines.\n" +
			"- Filter files by pattern with the include parameter (e.g. \"*.js\", \"*.{ts,tsx}\").\n" +
			"- Use this tool when you need to find files containing specific patterns.\n" +
			"- You can call multiple grep searches in parallel to batch lookups.\n" +
			"- Default exclusions: " + strings.Join(defaultExcludeDirs, ", ") + ".",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":          map[string]any{"type": "string", "description": "Regex pattern to search (e.g. \"log.*Error\", \"function\\s+\\w+\")"},
				"path":             map[string]any{"type": "string", "description": "Relative directory or file path to search (default: project root)"},
				"include":          map[string]any{"type": "string", "description": "File pattern to include in the search (e.g. \"*.js\", \"*.{ts,tsx}\")"},
				"context_lines":    map[string]any{"type": "integer", "description": "Number of context lines before and after each match (default: 0)"},
				"case_insensitive": map[string]any{"type": "boolean", "description": "Perform case-insensitive matching (default: false)"},
				"max":              map[string]any{"type": "integer", "description": "Maximum number of results (default: 100)"},
			},
			"required": []string{"pattern"},
		},
	}
}

type grepMatch struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Content  string `json:"content"`
	Context  []string `json:"context,omitempty"`
}

func (h *Grep) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return domain.ToolResult{}, fmt.Errorf("pattern is required")
	}

	caseInsensitive := optionalBoolField(input, "case_insensitive", false)
	if caseInsensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
	}

	root, _ := input["path"].(string)
	if root == "" {
		root = "."
	}
	workDir := workDirFromInput(input)
	root, err = resolvePath(workDir, root)
	if err != nil {
		return domain.ToolResult{}, err
	}

	include, _ := input["include"].(string)
	contextLines := optionalIntField(input, "context_lines")

	maxResults := optionalIntField(input, "max")
	if maxResults <= 0 {
		maxResults = 100
	}

	var results []grepMatch
	count := 0

	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				name := info.Name()
				for _, excl := range defaultExcludeDirs {
					if name == excl {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}

		if include != "" {
			if !matchGlob(include, info.Name()) {
				return nil
			}
		}

		if count >= maxResults {
			return filepath.SkipAll
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if isBinary(data) {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if count >= maxResults {
				return filepath.SkipAll
			}
			if re.MatchString(line) {
				match := grepMatch{
					File:    path,
					Line:    i + 1,
					Content: strings.TrimSpace(line),
				}
				if contextLines > 0 {
					ctx := make([]string, 0, contextLines*2)
					for k := maxInt(0, i-contextLines); k < minInt(len(lines), i+contextLines+1); k++ {
						if k != i {
							prefix := "  "
							if strings.TrimSpace(lines[k]) != "" {
								prefix = "│ "
							}
							ctx = append(ctx, prefix+lines[k])
						}
					}
					match.Context = ctx
				}
				results = append(results, match)
				count++
			}
		}
		return nil
	})

	return domain.ToolResult{
		Content: h.formatResults(results, contextLines, count, maxResults),
		Meta:    map[string]any{"total_matches": count, "truncated": count >= maxResults},
	}, nil
}

func (h *Grep) formatResults(results []grepMatch, contextLines, count, maxResults int) string {
	var b strings.Builder
	if len(results) == 0 {
		return "No matches found"
	}
	for i, m := range results {
		fmt.Fprintf(&b, "%s:%d: %s\n", m.File, m.Line, m.Content)
		if contextLines > 0 && len(m.Context) > 0 {
			for _, ctxLine := range m.Context {
				b.WriteString(ctxLine + "\n")
			}
		}
		if i < len(results)-1 {
			b.WriteString("\n")
		}
	}
	if count >= maxResults {
		b.WriteString(fmt.Sprintf("\n[Truncated: %d matches shown. Use a more specific pattern or path to narrow results]", count))
	}
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func matchGlob(pattern, name string) bool {
	if pattern == "" {
		return true
	}
	// Support brace expansion like "*.{ts,tsx}"
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		prefix := pattern[:strings.Index(pattern, "{")]
		suffix := pattern[strings.LastIndex(pattern, "}")+1:]
		inner := pattern[strings.Index(pattern, "{")+1 : strings.LastIndex(pattern, "}")]
		for _, alt := range strings.Split(inner, ",") {
			if ok, _ := filepath.Match(prefix+alt+suffix, name); ok {
				return true
			}
		}
		return false
	}
	ok, _ := filepath.Match(pattern, name)
	return ok
}

type Glob struct{}

func (h *Glob) Name() string                { return "glob" }
func (h *Glob) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *Glob) Describe(args map[string]any) string {
	pattern, _ := args["pattern"].(string)
	return pattern
}
func (h *Glob) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "glob",
		Description: "Fast file pattern matching tool to find files by name.\n\n" +
			"**Important**: Searches within the project root directory. Use relative paths in patterns.\n\n" +
			"- Supports glob patterns like \"**/*.go\" or \"src/**/*.ts\".\n" +
			"- The ** pattern matches any number of directory levels (recursive).\n" +
			"- Returns matching file paths, including dotfiles.\n" +
			"- Use this tool when you need to find files by name patterns.\n" +
			"- Batch multiple glob searches in parallel when looking for different file types.\n" +
			"- Use the path parameter to scope the search to a specific directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern, e.g. \"**/*.go\" or \"src/**/*.ts\""},
				"path":    map[string]any{"type": "string", "description": "Relative directory path to search in (default: project root)"},
				"max":     map[string]any{"type": "integer", "description": "Maximum number of results (default: 50)"},
			},
			"required": []string{"pattern"},
		},
	}
}
func (h *Glob) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return domain.ToolResult{}, fmt.Errorf("pattern is required")
	}

	searchRoot, _ := input["path"].(string)
	if searchRoot != "" {
		workDir := workDirFromInput(input)
		resolved, err := resolvePath(workDir, searchRoot)
		if err != nil {
			return domain.ToolResult{}, err
		}
		pattern = filepath.Join(resolved, pattern)
	}

	maxResults := optionalIntField(input, "max")
	if maxResults <= 0 {
		maxResults = 50
	}

	pattern, err := resolvePath(workDirFromInput(input), pattern)
	if err != nil {
		pattern, err = resolveGlobPattern(workDirFromInput(input), pattern)
		if err != nil {
			return domain.ToolResult{}, err
		}
	}

	var matches []string
	if strings.Contains(pattern, "**") {
		matches, err = doubleGlob(pattern)
	} else {
		matches, err = filepath.Glob(pattern)
	}
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("invalid glob: %w", err)
	}

	truncated := len(matches) > maxResults
	if truncated {
		matches = matches[:maxResults]
	}

	result := strings.Join(matches, "\n")
	if truncated {
		result += fmt.Sprintf("\n\n[Truncated: %d/%d results shown. Use a more specific path/pattern]", maxResults, len(matches))
	}

	return domain.ToolResult{
		Content: result,
		Meta:    map[string]any{"total_matches": len(matches), "truncated": truncated},
	}, nil
}

func resolveGlobPattern(workDir, pattern string) (string, error) {
	if !strings.Contains(pattern, "**") {
		resolved, err := resolvePath(workDir, pattern)
		if err != nil {
			return "", err
		}
		return resolved, nil
	}
	parts := strings.Split(pattern, string(filepath.Separator))
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid glob pattern: %q", pattern)
	}
	if parts[0] == "**" {
		return filepath.Join(workDir, pattern), nil
	}
	firstPart, err := resolvePath(workDir, parts[0])
	if err != nil {
		return "", err
	}
	remainingParts := append([]string{firstPart}, parts[1:]...)
	return filepath.Join(remainingParts...), nil
}

func doubleGlob(pattern string) ([]string, error) {
	base := pattern
	for strings.Contains(base, "**") {
		parent := filepath.Dir(base)
		if parent == base {
			break
		}
		base = parent
	}
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		base = filepath.Dir(base)
	}

	patSegs := strings.Split(filepath.ToSlash(pattern), "/")
	var matches []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		pathSegs := strings.Split(filepath.ToSlash(path), "/")
		if globMatchPath(patSegs, pathSegs) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func globMatchPath(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		rest := pattern[1:]
		if len(rest) == 0 {
			return true
		}
		for i := 0; i <= len(path); i++ {
			if globMatchPath(rest, path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	ok, _ := filepath.Match(pattern[0], path[0])
	if !ok {
		return false
	}
	return globMatchPath(pattern[1:], path[1:])
}

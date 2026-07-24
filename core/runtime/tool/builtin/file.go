package builtin

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"danmo-work/core/domain"
)

const (
	defaultReadLimit = 2000
	maxReadLines     = 5000
	maxLineLength    = 2000
	outputMaxChars   = 50000
	maxListEntries   = 500
)

type ReadFile struct{}

func (h *ReadFile) Name() string                { return "read_file" }
func (h *ReadFile) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *ReadFile) Describe(args map[string]any) string {
	path, _ := args["path"].(string)
	offset := optionalIntField(args, "offset")
	limit := optionalIntField(args, "limit")
	if offset == 0 && limit == 0 {
		return path
	}
	if limit == 0 {
		return fmt.Sprintf("%s (offset=%d)", path, offset)
	}
	return fmt.Sprintf("%s (offset=%d, limit=%d)", path, offset, limit)
}
func (h *ReadFile) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "read_file",
		Description: "Reads a file or directory from the local filesystem. If the path does not exist, an error is returned.\n\n" +
			"**Important**: All paths are relative to the project root directory. Use relative paths like 'src/main.go' instead of absolute paths.\n\n" +
			"Usage:\n" +
			"- By default, this tool returns up to " + fmt.Sprintf("%d", defaultReadLimit) + " lines from the start of the file.\n" +
			"- The offset parameter is the line number to start from (1-indexed).\n" +
			"- To read later sections, call this tool again with a larger offset.\n" +
			"- Use grep to find specific content in large files first, then use offset+limit to read around matches.\n" +
			"- Use glob to look up filenames by pattern.\n" +
			"- Contents are returned with each line prefixed by its line number as 'line: content'.\n" +
			"- Any line longer than " + fmt.Sprintf("%d", maxLineLength) + " characters is truncated.\n" +
			"- Call this tool in parallel when you know there are multiple files you want to read.\n" +
			"- Avoid tiny repeated slices (30 line chunks). If you need more context, read a larger window.\n" +
			"- This tool can also read directories: pass a directory path to list its entries.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Relative file or directory path from project root (e.g., 'src/main.go')"},
				"offset": map[string]any{"type": "integer", "description": "1-based line number to start reading from (default: 1)"},
				"limit":  map[string]any{"type": "integer", "description": "Maximum number of lines to read (default: " + fmt.Sprintf("%d", defaultReadLimit) + ")"},
			},
			"required": []string{"path"},
		},
	}
}

func (h *ReadFile) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}
	workDir := workDirFromInput(input)

	resolvedPath, info, err := readFilePath(workDir, path)
	if err != nil {
		return domain.ToolResult{}, err
	}

	if info.IsDir() {
		return h.listDirectory(resolvedPath)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return domain.ToolResult{}, err
	}

	if err := checkBinary(data, resolvedPath); err != nil {
		return domain.ToolResult{Content: fmt.Sprintf("[Binary file not displayed: %s (%d bytes)]", path, len(data))}, nil
	}

	noteReadFile(input, resolvedPath)

	offset := optionalIntField(input, "offset")
	if offset > 0 {
		offset--
	}
	limit := optionalIntField(input, "limit")
	if limit <= 0 {
		limit = defaultReadLimit
	}
	if limit > maxReadLines {
		limit = maxReadLines
	}

	lines := splitLines(string(data))
	startIdx := offset
	if startIdx >= len(lines) {
		return domain.ToolResult{}, fmt.Errorf("offset %d exceeds file length of %d lines", offset, len(lines))
	}
	endIdx := startIdx + limit
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		line := lines[i]
		if len(line) > maxLineLength {
			line = line[:maxLineLength] + fmt.Sprintf("... (line truncated to %d chars)", maxLineLength)
		}
		fmt.Fprintf(&b, "%d: %s\n", i+1, line)
	}

	if endIdx < len(lines) {
		b.WriteString(fmt.Sprintf("\n[Truncated. %d/%d lines shown. Use offset=%d limit=%d to continue reading...]\n",
			endIdx-startIdx, len(lines), endIdx+1, limit))
	}

	result := b.String()
	if len(result) > outputMaxChars {
		result = result[:outputMaxChars] + fmt.Sprintf("\n\n[Output truncated to %d chars]", outputMaxChars)
	}

	return domain.ToolResult{
		Content: result,
		Meta: map[string]any{
			"path":        path,
			"total_lines": len(lines),
			"shown_lines": endIdx - startIdx,
			"start_line":  startIdx + 1,
			"end_line":    endIdx,
		},
	}, nil
}

type dirEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (h *ReadFile) listDirectory(dirPath string) (domain.ToolResult, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return domain.ToolResult{}, err
	}

	items := make([]dirEntry, 0, len(entries))
	for _, e := range entries {
		info, infoErr := e.Info()
		if infoErr != nil {
			items = append(items, dirEntry{Name: e.Name(), IsDir: e.IsDir()})
			continue
		}
		items = append(items, dirEntry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	totalEntries := len(items)
	truncated := false
	if len(items) > maxListEntries {
		items = items[:maxListEntries]
		truncated = true
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Contents of %s (%d items):\n", dirPath, totalEntries))
	for _, item := range items {
		if item.IsDir {
			b.WriteString(fmt.Sprintf("  [dir ] %s/\n", item.Name))
		} else {
			b.WriteString(fmt.Sprintf("  [file] %-40s %10d bytes  %s\n", item.Name, item.Size, item.ModTime))
		}
	}
	if truncated {
		b.WriteString(fmt.Sprintf("\n[Truncated: %d entries shown of %d total]", len(items), totalEntries))
	}

	return domain.ToolResult{Content: b.String()}, nil
}

func optionalIntField(input map[string]any, key string) int {
	switch v := input[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func optionalBoolField(input map[string]any, key string, defaultVal bool) bool {
	v, ok := input[key].(bool)
	if !ok {
		return defaultVal
	}
	return v
}

func requiredStringField(input map[string]any, key string) (string, error) {
	s, ok := input[key].(string)
	if !ok || s == "" {
		return "", fmt.Errorf("field %q is required", key)
	}
	return s, nil
}

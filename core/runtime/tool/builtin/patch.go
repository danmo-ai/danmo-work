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

const (
	defaultPatchFuzz = 3
	maxPatchFuzz     = 50
)

type ApplyPatch struct{}

func (h *ApplyPatch) Name() string                { return "apply_patch" }
func (h *ApplyPatch) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *ApplyPatch) Describe(args map[string]any) string {
	return "apply_patch"
}
func (h *ApplyPatch) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "apply_patch",
		Description: "Applies a unified diff patch to files. Preferred for multi-file or multi-hunk edits.\n\n" +
			"**Important**: File paths in the patch must be relative to the project root directory (e.g., 'src/main.go', not '/absolute/path/src/main.go').\n\n" +
			"- patch must be a valid unified diff string with ---/+++ file headers and @@ hunk headers.\n" +
			"- Can apply multiple hunks across multiple files in a single call.\n" +
			"- Supports file creation (--- /dev/null) and file deletion (+++ /dev/null).\n" +
			"- Always read the target files first to understand the current state.\n" +
			"- Use this instead of multiple edit calls when changing several locations at once.\n" +
			"- Supports fuzz matching to tolerate line offsets (default: " + fmt.Sprintf("%d", defaultPatchFuzz) + ").",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"patch":             map[string]any{"type": "string", "description": "A unified diff patch string to apply"},
				"fuzz":              map[string]any{"type": "integer", "description": "Maximum lines to search for context match (default: " + fmt.Sprintf("%d", defaultPatchFuzz) + ", max: " + fmt.Sprintf("%d", maxPatchFuzz) + ")"},
				"create_if_missing": map[string]any{"type": "boolean", "description": "Create the file if it doesn't exist (default: false)"},
			},
			"required": []string{"patch"},
		},
	}
}

var (
	hunkHeaderRe  = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`)
	fileHeaderRe  = regexp.MustCompile(`^--- (?:a/|b/)?(\S+)`)
	fileHeaderRe2 = regexp.MustCompile(`^\+\+\+ (?:a/|b/)?(\S+)`)
)

type hunk struct {
	oldStart, oldCount int
	newStart, newCount int
	lines              []hunkLine
}

type hunkLine struct {
	op   byte
	text string
}

type filePatch struct {
	path     string // absolute after resolve
	relPath  string // project-relative path for Meta / tracking
	hunks    []hunk
	isCreate bool
	isDelete bool
	oldData  []byte
}

type pendingWrite struct {
	path    string
	data    []byte
	oldData []byte
}

func (h *ApplyPatch) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	patch, _ := input["patch"].(string)
	if patch == "" {
		return domain.ToolResult{}, fmt.Errorf("patch is required")
	}

	fuzz := optionalIntField(input, "fuzz")
	if fuzz <= 0 {
		fuzz = defaultPatchFuzz
	}
	if fuzz > maxPatchFuzz {
		fuzz = maxPatchFuzz
	}

	createIfMissing := optionalBoolField(input, "create_if_missing", false)

	workDir := workDirFromInput(input)
	patches, err := parsePatch(patch)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("invalid patch: %w", err)
	}
	if len(patches) == 0 {
		return domain.ToolResult{Content: "No files to patch"}, nil
	}

	// Preflight: resolve paths, detect create/delete, read files
	var results []string
	var writes []pendingWrite
	var changeMeta []map[string]any

	for i := range patches {
		fp := &patches[i]
		fp.relPath = fp.path

		if fp.isCreate {
			fp.path, err = resolvePath(workDir, fp.path)
			if err != nil && !createIfMissing {
				return domain.ToolResult{}, fmt.Errorf("cannot create file %q: %w", fp.relPath, err)
			}
			if createIfMissing || err == nil {
				if _, statErr := os.Stat(fp.path); statErr == nil && len(fp.hunks) > 0 {
					return domain.ToolResult{}, fmt.Errorf("cannot create file %q: already exists. Use create_if_missing=true to overwrite", fp.relPath)
				}
				dir := filepath.Dir(fp.path)
				if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
					return domain.ToolResult{}, fmt.Errorf("cannot create parent dirs for %q: %w", fp.relPath, mkErr)
				}
			}
		} else {
			fp.path, err = resolvePath(workDir, fp.path)
			if err != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot resolve path %q: %w", fp.relPath, err)
			}
		}
	}

	for i := range patches {
		fp := &patches[i]

		if fp.isDelete {
			data, readErr := os.ReadFile(fp.path)
			if readErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot read file %q for deletion: %w", fp.relPath, readErr)
			}
			fp.oldData = data
			writes = append(writes, pendingWrite{path: fp.path, data: nil, oldData: data})
			continue
		}

		if fp.isCreate {
			writes = append(writes, pendingWrite{path: fp.path, data: []byte{}, oldData: nil})
		} else {
			data, readErr := os.ReadFile(fp.path)
			if readErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot read file %q: %w", fp.relPath, readErr)
			}
			fp.oldData = data

			lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			_, applyErr := applyHunks(lines, fp.hunks, fuzz)
			if applyErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot apply patch to %q: %w", fp.relPath, applyErr)
			}
		}
	}

	// All hunks validated, now apply
	for i := range patches {
		fp := &patches[i]

		if fp.isDelete {
			if delErr := os.Remove(fp.path); delErr != nil {
				h.rollbackWrites(writes)
				return domain.ToolResult{}, fmt.Errorf("cannot delete file %q: %w", fp.relPath, delErr)
			}
			results = append(results, fmt.Sprintf("Deleted %q", fp.relPath))
			diff := generateUnifiedDiff(fp.relPath, string(fp.oldData), "")
			changeMeta = append(changeMeta, map[string]any{
				"path": fp.relPath, "op": "delete", "diff": diff, "bytes_written": 0,
			})
			continue
		}

		var newLines []string
		if fp.isCreate {
			lines := ([]string)(nil)
			newLines, _ = applyHunks(lines, fp.hunks, fuzz)
		} else {
			lines := strings.Split(strings.TrimSuffix(string(fp.oldData), "\n"), "\n")
			newLines, _ = applyHunks(lines, fp.hunks, fuzz)
		}

		newContent := strings.Join(newLines, "\n") + "\n"
		writes = append(writes, pendingWrite{
			path:    fp.path,
			data:    []byte(newContent),
			oldData: fp.oldData,
		})

		if err := os.WriteFile(fp.path, []byte(newContent), 0644); err != nil {
			h.rollbackWrites(writes)
			return domain.ToolResult{}, fmt.Errorf("cannot write file %q: %w", fp.relPath, err)
		}

		results = append(results, fmt.Sprintf("Patched %q (%d hunks)", fp.relPath, len(fp.hunks)))
		op := "update"
		oldStr := ""
		if fp.isCreate {
			op = "create"
		} else {
			oldStr = string(fp.oldData)
		}
		diff := generateUnifiedDiff(fp.relPath, oldStr, newContent)
		changeMeta = append(changeMeta, map[string]any{
			"path": fp.relPath, "op": op, "diff": diff, "bytes_written": len(newContent),
		})
	}

	return domain.ToolResult{
		Content: strings.Join(results, "\n"),
		Meta:    map[string]any{"file_changes": changeMeta},
	}, nil
}

func (h *ApplyPatch) rollbackWrites(writes []pendingWrite) {
	for i := len(writes) - 1; i >= 0; i-- {
		w := writes[i]
		if w.oldData == nil {
			if w.data == nil {
				continue
			}
			os.Remove(w.path)
		} else {
			os.WriteFile(w.path, w.oldData, 0644)
		}
	}
}

func parsePatch(patch string) ([]filePatch, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	var patches []filePatch
	var cur *filePatch
	var curHunk *hunk

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "--- "):
			if cur != nil && len(cur.hunks) > 0 {
				patches = append(patches, *cur)
			}
			path := extractHeaderPath(line)
			cur = &filePatch{path: filepath.Clean(path)}
			if path == "/dev/null" {
				cur.isCreate = true
				cur.path = ""
			}
		case strings.HasPrefix(line, "+++ "):
			if cur == nil {
				return nil, fmt.Errorf("unexpected +++ header at line %d", i+1)
			}
			path := extractHeaderPath(line)
			if path == "/dev/null" {
				cur.isDelete = true
			} else if cur.path == "" {
				cur.path = filepath.Clean(path)
			}
		case strings.HasPrefix(line, "@@ "):
			if cur == nil {
				return nil, fmt.Errorf("hunk without file header at line %d", i+1)
			}
			m := hunkHeaderRe.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("invalid hunk header at line %d", i+1)
			}
			h := hunk{lines: make([]hunkLine, 0)}
			h.oldStart = parseInt(m[1])
			if m[2] != "" {
				h.oldCount = parseInt(m[2])
			} else {
				h.oldCount = 1
			}
			h.newStart = parseInt(m[3])
			if m[4] != "" {
				h.newCount = parseInt(m[4])
			} else {
				h.newCount = 1
			}
			curHunk = &h
			cur.hunks = append(cur.hunks, h)
			curHunk = &cur.hunks[len(cur.hunks)-1]
		case strings.HasPrefix(line, " "):
			if curHunk != nil {
				curHunk.lines = append(curHunk.lines, hunkLine{op: ' ', text: line[1:]})
			}
		case strings.HasPrefix(line, "-"):
			if curHunk != nil {
				curHunk.lines = append(curHunk.lines, hunkLine{op: '-', text: line[1:]})
			}
		case strings.HasPrefix(line, "+"):
			if curHunk != nil {
				curHunk.lines = append(curHunk.lines, hunkLine{op: '+', text: line[1:]})
			}
		case line == "" || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file"):
			if cur != nil && len(cur.hunks) > 0 {
				patches = append(patches, *cur)
			}
			cur = nil
			curHunk = nil
		}
	}
	if cur != nil && len(cur.hunks) > 0 {
		patches = append(patches, *cur)
	}
	return patches, nil
}

func extractHeaderPath(line string) string {
	re := fileHeaderRe
	if strings.HasPrefix(line, "+++ ") {
		re = fileHeaderRe2
	}
	m := re.FindStringSubmatch(line)
	if m != nil {
		return m[1]
	}
	return ""
}

func applyHunks(lines []string, hunks []hunk, fuzz int) ([]string, error) {
	result := make([]string, 0, len(lines)+len(hunks)*10)
	originalLen := len(lines)
	offset := 0

	for _, h := range hunks {
		oldIdx := h.oldStart - 1 - offset
		if oldIdx < 0 {
			oldIdx = 0
		}

		// Fuzz matching: search within ±fuzz lines for context match
		matchedIdx := findHunkMatch(lines, h, oldIdx, fuzz, offset)
		if matchedIdx < 0 {
			return nil, fmt.Errorf("hunk mismatch: cannot find context around line %d", h.oldStart)
		}
		oldIdx = matchedIdx

		for len(result) < oldIdx {
			result = append(result, lines[len(result)])
		}

		newIdx := oldIdx + h.oldCount
		offset += h.oldCount - h.newCount

		for _, hl := range h.lines {
			switch hl.op {
			case ' ':
				if oldIdx >= len(lines) || lines[oldIdx] != hl.text {
					return nil, fmt.Errorf("context mismatch at line %d: expected %q, got %q", oldIdx+1, hl.text, safeGet(lines, oldIdx))
				}
				result = append(result, hl.text)
				oldIdx++
			case '-':
				if oldIdx >= len(lines) || lines[oldIdx] != hl.text {
					return nil, fmt.Errorf("removal mismatch at line %d: expected %q, got %q", oldIdx+1, hl.text, safeGet(lines, oldIdx))
				}
				oldIdx++
			case '+':
				result = append(result, hl.text)
			}
		}

		for i := oldIdx; i < newIdx; i++ {
			if i < len(lines) {
				result = append(result, lines[i])
			}
		}
	}

	for len(result) < originalLen {
		result = append(result, lines[len(result)])
	}

	return result, nil
}

func findHunkMatch(lines []string, h hunk, startIdx, fuzz, offset int) int {
	if len(lines) == 0 {
		return startIdx
	}

	contextLine := ""
	for _, hl := range h.lines {
		if hl.op == ' ' {
			contextLine = hl.text
			break
		}
	}
	if contextLine == "" {
		return startIdx
	}

	// Search from startIdx-fuzz to startIdx+fuzz
	for delta := -fuzz; delta <= fuzz; delta++ {
		idx := startIdx + delta
		if idx < 0 || idx >= len(lines) {
			continue
		}

		if lines[idx] == contextLine {
			cumulativeOffset := 0
			match := true
			lineIdx := idx

			for _, hl := range h.lines {
				switch hl.op {
				case ' ':
					if lineIdx >= len(lines) || lines[lineIdx] != hl.text {
						match = false
					}
					lineIdx++
				case '-':
					if lineIdx >= len(lines) || lines[lineIdx] != hl.text {
						match = false
					}
					lineIdx++
					cumulativeOffset--
				case '+':
					cumulativeOffset++
				}
				if !match {
					break
				}
			}
			if match {
				return idx
			}
		}
	}

	return -1
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func safeGet(lines []string, idx int) string {
	if idx < len(lines) {
		return lines[idx]
	}
	return "<EOF>"
}

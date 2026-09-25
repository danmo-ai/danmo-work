package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"danmo-work/core/domain"
)

type Write struct{}

func (h *Write) Name() string                { return "write" }
func (h *Write) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *Write) Describe(args map[string]any) string {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	writeType, _ := args["write_type"].(string)
	if writeType == "directory" {
		return "create directory " + path
	}
	return path + " (" + fmt.Sprintf("%d", len(content)) + " chars)"
}
func (h *Write) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "write",
		Description: "Creates a file or directory, or replaces a whole file.\n\n" +
			"Paths are relative to the project root (e.g. src/main.go).\n\n" +
			"- Parent directories are created automatically. Do not use exec_shell mkdir, cat, echo, or heredoc.\n" +
			"- Overwrites the file at path when one exists. An existing file must be read with read_file first.\n" +
			"- Prefer edit for one replacement, edit_batch for several replacements in order (the same file may repeat), and apply_patch for diff hunks. Use write for a new file or a full rewrite.\n" +
			parallelSamePathRule +
			"- Text is stored as UTF-8 (no BOM). Overwriting an existing file includes a unified diff.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "Relative file path from project root (e.g., 'src/main.go')"},
				"content":    map[string]any{"type": "string", "description": "The content to write to the file (required when type is 'file')"},
				"write_type": map[string]any{"type": "string", "enum": []string{"file", "directory"}, "default": "file", "description": "'file' to write a file (default), 'directory' to create a directory. content is optional for 'directory'."},
			},
			"required": []string{"path"},
		},
	}
}

func (h *Write) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)
	writeType, _ := input["write_type"].(string)

	if path == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}
	if writeType == "" {
		writeType = "file"
	}

	resolvedPath, err := resolveWritePath(workDirFromInput(input), path)
	if err != nil {
		return domain.ToolResult{}, err
	}

	if writeType == "directory" {
		if err := os.MkdirAll(resolvedPath, 0755); err != nil {
			return domain.ToolResult{}, fmt.Errorf("cannot create directory %q: %w", resolvedPath, err)
		}
		return domain.ToolResult{
			Content: fmt.Sprintf("Created directory %q", path),
			Meta:    map[string]any{"path": path, "op": "create", "write_type": "directory"},
		}, nil
	}

	// Check for existing file: require read-first
	existingText, existingMeta, fileExists := "", textFileMeta{}, false
	encPreserved := false
	if info, statErr := os.Stat(resolvedPath); statErr == nil && !info.IsDir() {
		fileExists = true
		if err := requireFreshRead(input, resolvedPath); err != nil {
			return domain.ToolResult{}, err
		}
		if data, readErr := os.ReadFile(resolvedPath); readErr == nil {
			if text, meta, decErr := decodeTextFile(data); decErr == nil {
				existingText = text
				existingMeta = meta
				encPreserved = true
			} else {
				existingText = string(data)
			}
		}
	}

	op := "create"
	diff := ""
	if fileExists {
		op = "update"
		diff = generateUnifiedDiff(path, existingText, content)
		if diff == "" {
			// Identical content: do not rewrite (bumping mtime) and report an
			// explicit no-op. A silent "Wrote file" success here is
			// indistinguishable from a real write and drives identical-write
			// loops.
			noteReadFile(input, resolvedPath)
			return domain.ToolResult{
				Content: fmt.Sprintf("No changes written to %q — the file already contains identical content (%d bytes)", path, len([]byte(content))),
				Meta: map[string]any{
					"path":          path,
					"op":            "noop",
					"diff":          "",
					"changed":       false,
					"bytes_written": 0,
					"overwrote":     true,
				},
			}, nil
		}
	}

	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return domain.ToolResult{}, fmt.Errorf("cannot create directory %q: %w", dir, err)
	}

	payload := []byte(content)
	encNote := ""
	if encPreserved {
		outMeta := writeEncodingMeta(existingMeta)
		payload = encodeTextFile(content, outMeta)
		encNote = conversionNote(existingMeta, outMeta)
	}
	if err := writeFilePreserving(resolvedPath, payload); err != nil {
		return domain.ToolResult{}, fmt.Errorf("cannot write file %q: %w", resolvedPath, err)
	}
	// Own write updates the snapshot so a later edit/write in this turn does not
	// fail with "changed since last read" unless something else touched the file.
	noteReadFile(input, resolvedPath)

	msg := fmt.Sprintf("Wrote file %q (%d bytes)", path, len(payload))
	if fileExists {
		msg += encNote + "\n" + diff
	}

	return domain.ToolResult{
		Content: msg,
		Meta: map[string]any{
			"path":          path,
			"op":            op,
			"diff":          diff,
			"bytes_written": len(payload),
			"overwrote":     fileExists,
		},
	}, nil
}

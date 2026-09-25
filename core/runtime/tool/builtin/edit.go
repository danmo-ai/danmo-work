package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"danmo-work/core/domain"
)

// parallelSamePathRule is shared by file-mutation tools. One path must not be
// written by concurrent tool calls; same-file sequences belong in edit_batch
// or a single apply_patch.
const parallelSamePathRule = "- Do not run this tool in parallel with edit, edit_batch, write, apply_patch, or file_op on the same path. Concurrent writes to one path race and drop changes. Different paths may run in parallel.\n"

type Edit struct{}

func (h *Edit) Name() string                { return "edit" }
func (h *Edit) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *Edit) Describe(args map[string]any) string {
	path, _ := args["path"].(string)
	oldStr, _ := args["oldString"].(string)
	newStr, _ := args["newString"].(string)
	oldShort := oldStr
	newShort := newStr
	if len(oldStr) > 40 {
		oldShort = oldStr[:40] + "..."
	}
	if len(newStr) > 40 {
		newShort = newStr[:40] + "..."
	}
	return path + " (" + oldShort + " -> " + newShort + ")"
}
func (h *Edit) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "edit",
		Description: "Replaces one exact string in an existing file.\n\n" +
			"Paths are relative to the project root (e.g. src/main.go).\n\n" +
			"- Read this exact path with read_file first in this turn. Fails if it was not read, or if it changed since that read.\n" +
			"- oldString is file text, not the read_file line-number prefix (e.g. '1: '). Keep indentation. Match order: exact, then indent-strip, then whitespace normalize. A miss returns the closest region — copy that text, or re-read, and retry.\n" +
			"- newString must differ from oldString. replaceAll changes every match; otherwise the match must be unique.\n" +
			"- One replacement per call. Several replacements, including more than one in the same file, go in one edit_batch (applied in order; later items see earlier results). Diff hunks go in one apply_patch.\n" +
			parallelSamePathRule +
			"- Text is stored as UTF-8 (no BOM). The result includes a unified diff.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "Relative file path from project root (e.g., 'src/main.go')"},
				"oldString":  map[string]any{"type": "string", "description": "The text to replace"},
				"newString":  map[string]any{"type": "string", "description": "The text to replace it with (must be different from oldString)"},
				"replaceAll": map[string]any{"type": "boolean", "description": "Replace all occurrences of oldString (default: false)"},
			},
			"required": []string{"path", "oldString", "newString"},
		},
	}
}

func (h *Edit) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	path, _ := input["path"].(string)
	oldStr, _ := input["oldString"].(string)
	newStr, _ := input["newString"].(string)
	replaceAll, _ := input["replaceAll"].(bool)

	if path == "" {
		return domain.ToolResult{}, fmt.Errorf("path is required")
	}
	if oldStr == "" {
		return domain.ToolResult{}, fmt.Errorf("oldString is required")
	}
	relPath := path
	resolvedPath, err := resolveWritePath(workDirFromInput(input), path)
	if err != nil {
		return domain.ToolResult{}, err
	}

	if err := requireFreshRead(input, resolvedPath); err != nil {
		return domain.ToolResult{}, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("cannot read file %q: %w", resolvedPath, err)
	}
	content, meta, err := decodeTextFile(data)
	if err != nil {
		return domain.ToolResult{}, fmt.Errorf("cannot edit %q: %w", relPath, err)
	}

	replacement, count, matchErr := applyStringReplace(relPath, content, oldStr, newStr, replaceAll)
	if matchErr != nil {
		return domain.ToolResult{}, matchErr
	}

	diff := generateUnifiedDiff(relPath, content, replacement)
	if diff == "" {
		// Fuzzy matching can land on the file's actual text (oldStr != newStr
		// but the replacement equals what's on disk): the edit is a no-op.
		// Report it explicitly instead of rewriting and claiming success.
		noteReadFile(input, resolvedPath)
		return domain.ToolResult{
			Content: fmt.Sprintf("No changes made to %q — the replacement text is identical to the current content", relPath),
			Meta: map[string]any{
				"path":          relPath,
				"op":            "noop",
				"diff":          "",
				"changed":       false,
				"replacements":  0,
				"bytes_written": 0,
				"encoding":      string(meta.Encoding),
				"line_ending":   meta.LineEnding,
			},
		}, nil
	}

	outMeta := writeEncodingMeta(meta)
	if err := writeFilePreserving(resolvedPath, encodeTextFile(replacement, outMeta)); err != nil {
		return domain.ToolResult{}, fmt.Errorf("cannot write file %q: %w", resolvedPath, err)
	}
	// Own write updates the snapshot so a later edit in this turn does not
	// fail with "changed since last read" unless something else touched the file.
	noteReadFile(input, resolvedPath)

	encNote := conversionNote(meta, outMeta)
	return domain.ToolResult{
		Content: fmt.Sprintf("Edited file %q, replaced %d occurrence(s):%s\n%s", relPath, count, encNote, diff),
		Meta: map[string]any{
			"path":          relPath,
			"op":            "update",
			"diff":          diff,
			"replacements":  count,
			"bytes_written": len(replacement),
			"encoding":      string(outMeta.Encoding),
			"line_ending":   outMeta.LineEnding,
		},
	}, nil
}

// applyStringReplace runs the edit match cascade against in-memory text.
// relPath is only used in the not-found hint.
func applyStringReplace(relPath, content, oldStr, newStr string, replaceAll bool) (string, int, error) {
	if oldStr == "" {
		return "", 0, fmt.Errorf("oldString is required")
	}
	if oldStr == newStr {
		return "", 0, fmt.Errorf("oldString and newString must be different")
	}

	replacement, count, matchErr := tryExactReplace(content, oldStr, newStr, replaceAll)
	if matchErr != nil {
		if !strings.Contains(matchErr.Error(), "not found") {
			return "", 0, matchErr
		}
		replacement, count, matchErr = tryIndentFuzzyReplace(content, oldStr, newStr, replaceAll)
	}
	if matchErr != nil {
		replacement, count, matchErr = tryWhitespaceFuzzyReplace(content, oldStr, newStr, replaceAll)
	}
	if matchErr != nil {
		if strings.Contains(matchErr.Error(), "not found") {
			return "", 0, formatEditNotFoundError(relPath, content, oldStr)
		}
		if strings.Contains(matchErr.Error(), "occurrences of oldString") {
			return "", 0, fmt.Errorf("%w. Tip: widen oldString with surrounding unique context, or set replaceAll=true", matchErr)
		}
		return "", 0, matchErr
	}
	return replacement, count, nil
}

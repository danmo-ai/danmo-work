package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"danmo-work/core/domain"
)

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
		Description: "Performs exact string replacements in an existing file.\n\n" +
			"**Important**: All paths are relative to the project root directory. Use relative paths like 'src/main.go' instead of absolute paths.\n\n" +
			"- You MUST use read_file first on this exact path -- the edit will fail if you haven't read it in this turn.\n" +
			"- When editing text from read_file output, preserve exact indentation (tabs/spaces). The line number prefix from read_file (e.g., '1: ') is NOT part of the file content.\n" +
			"- oldString must match the file content. If exact matching fails, fuzzy matching tries indent-strip then whitespace normalize.\n" +
			"- On failure, the error includes the closest matching file region — copy exact text from that hint (or re-read) and retry.\n" +
			"- newString must be different from oldString.\n" +
			"- Use replaceAll for replacing and renaming strings across the file.\n" +
			"- For multi-hunk or multi-file edits, prefer apply_patch (begin-patch) instead of many edit calls.\n" +
			"- The result includes a unified diff showing what was changed.",
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
	if oldStr == newStr {
		return domain.ToolResult{}, fmt.Errorf("oldString and newString must be different")
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

	replacement, count, matchErr := tryExactReplace(content, oldStr, newStr, replaceAll)

	if matchErr != nil {
		if !strings.Contains(matchErr.Error(), "not found") {
			return domain.ToolResult{}, matchErr
		}
		replacement, count, matchErr = tryIndentFuzzyReplace(content, oldStr, newStr, replaceAll)
	}

	if matchErr != nil {
		replacement, count, matchErr = tryWhitespaceFuzzyReplace(content, oldStr, newStr, replaceAll)
	}

	if matchErr != nil {
		if strings.Contains(matchErr.Error(), "not found") {
			return domain.ToolResult{}, formatEditNotFoundError(relPath, content, oldStr)
		}
		// Multiple matches: keep the original error but add a short next-step hint.
		if strings.Contains(matchErr.Error(), "occurrences of oldString") {
			return domain.ToolResult{}, fmt.Errorf("%w. Tip: widen oldString with surrounding unique context, or set replaceAll=true", matchErr)
		}
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

package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"danmo-work/core/domain"
)

const maxEditBatchItems = 100

type EditBatch struct{}

func (h *EditBatch) Name() string                { return "edit_batch" }
func (h *EditBatch) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *EditBatch) Describe(args map[string]any) string {
	items, err := parseEditBatchItems(args["edits"])
	if err != nil || len(items) == 0 {
		return "edit_batch"
	}
	files := map[string]struct{}{}
	for _, item := range items {
		if item.path != "" {
			files[item.path] = struct{}{}
		}
	}
	return fmt.Sprintf("edit_batch (%d edits, %d files)", len(items), len(files))
}

func (h *EditBatch) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "edit_batch",
		Description: "Applies an ordered list of exact string replacements in one call. Items run one after another, so a later item sees earlier results and the same file may appear more than once.\n\n" +
			"Paths are relative to the project root (e.g. src/main.go).\n\n" +
			"- Read every target path with read_file first in this turn.\n" +
			"- Each item uses the same match rules as edit: exact, then indent-strip, then whitespace normalize. oldString is file text, not the read_file line-number prefix.\n" +
			"- When a later item overlaps an earlier change, oldString must match the text after that earlier change, not the original read_file output.\n" +
			"- If any item fails, no file in the batch is written.\n" +
			"- Use edit for a single replacement. Use apply_patch when a diff hunk is clearer. Use write only to create a file or replace it entirely.\n" +
			parallelSamePathRule +
			"- Text is stored as UTF-8 (no BOM). The result includes one unified diff per file that changed.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"edits": map[string]any{
					"type":        "array",
					"description": fmt.Sprintf("Ordered replacements, up to %d. Applied serially; the same path may repeat.", maxEditBatchItems),
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path":       map[string]any{"type": "string", "description": "Relative file path from project root (e.g. src/main.go)"},
							"oldString":  map[string]any{"type": "string", "description": "The text to replace, matched against the file as left by earlier items"},
							"newString":  map[string]any{"type": "string", "description": "The text to replace it with (must differ from oldString)"},
							"replaceAll": map[string]any{"type": "boolean", "description": "Replace all occurrences of oldString (default: false)"},
						},
						"required": []string{"path", "oldString", "newString"},
					},
				},
			},
			"required": []string{"edits"},
		},
	}
}

type editBatchItem struct {
	path       string
	old        string
	new        string
	replaceAll bool
}

type batchFileState struct {
	rel          string
	resolved     string
	raw          []byte
	originalText string
	text         string
	meta         textFileMeta
	mode         os.FileMode
	dirty        bool
}

func (h *EditBatch) Execute(_ context.Context, input map[string]any) (domain.ToolResult, error) {
	items, err := parseEditBatchItems(input["edits"])
	if err != nil {
		return domain.ToolResult{}, err
	}
	if len(items) == 0 {
		return domain.ToolResult{}, fmt.Errorf("edits must contain at least one replacement")
	}
	if len(items) > maxEditBatchItems {
		return domain.ToolResult{}, fmt.Errorf("edits has %d items; the limit is %d — split into another edit_batch call", len(items), maxEditBatchItems)
	}

	workDir := workDirFromInput(input)
	files := map[string]*batchFileState{}
	var order []string
	lines := make([]string, 0, len(items))

	for i, item := range items {
		label := fmt.Sprintf("edit %d/%d", i+1, len(items))
		if item.path == "" {
			return domain.ToolResult{}, fmt.Errorf("%s: path is required. No files were written", label)
		}
		resolved, err := resolveWritePath(workDir, item.path)
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("%s (%s): %w. No files were written", label, item.path, err)
		}
		st, ok := files[resolved]
		if !ok {
			if err := requireFreshRead(input, resolved); err != nil {
				return domain.ToolResult{}, fmt.Errorf("%s (%s): %w. No files were written", label, item.path, err)
			}
			data, err := os.ReadFile(resolved)
			if err != nil {
				return domain.ToolResult{}, fmt.Errorf("%s: cannot read file %q: %w. No files were written", label, item.path, err)
			}
			text, meta, err := decodeTextFile(data)
			if err != nil {
				return domain.ToolResult{}, fmt.Errorf("%s: cannot edit %q: %w. No files were written", label, item.path, err)
			}
			mode := os.FileMode(0o644)
			if info, statErr := os.Stat(resolved); statErr == nil && !info.IsDir() {
				mode = info.Mode().Perm()
			}
			st = &batchFileState{
				rel:          item.path,
				resolved:     resolved,
				raw:          data,
				originalText: text,
				text:         text,
				meta:         meta,
				mode:         mode,
			}
			files[resolved] = st
			order = append(order, resolved)
		}
		replacement, count, matchErr := applyStringReplace(st.rel, st.text, item.old, item.new, item.replaceAll)
		if matchErr != nil {
			return domain.ToolResult{}, fmt.Errorf("%s (%s): %w. No files were written", label, st.rel, matchErr)
		}
		if replacement == st.text {
			lines = append(lines, fmt.Sprintf("%d. %s: no change", i+1, st.rel))
			continue
		}
		st.text = replacement
		st.dirty = true
		lines = append(lines, fmt.Sprintf("%d. %s: replaced %d occurrence(s)", i+1, st.rel, count))
	}

	var written []batchFileState
	var changeMeta []map[string]any
	var diffs []string
	for _, resolved := range order {
		st := files[resolved]
		if !st.dirty {
			continue
		}
		outMeta := writeEncodingMeta(st.meta)
		payload := encodeTextFile(st.text, outMeta)
		if err := writeFilePreserving(resolved, payload); err != nil {
			restoreBatchWrites(append(written, *st))
			return domain.ToolResult{}, fmt.Errorf("cannot write file %q: %w", st.rel, err)
		}
		written = append(written, *st)
		noteReadFile(input, resolved)
		diff := generateUnifiedDiff(st.rel, st.originalText, st.text)
		diffs = append(diffs, diff)
		changeMeta = append(changeMeta, map[string]any{
			"path":          st.rel,
			"op":            "update",
			"diff":          diff,
			"bytes_written": len(st.text),
			"encoding":      string(outMeta.Encoding),
			"line_ending":   outMeta.LineEnding,
		})
		if note := conversionNote(st.meta, outMeta); note != "" {
			lines = append(lines, fmt.Sprintf("wrote %s%s", st.rel, note))
		}
	}

	if len(changeMeta) == 0 {
		return domain.ToolResult{
			Content: "No changes made — every replacement matched text already in the file",
			Meta: map[string]any{
				"changed":      false,
				"file_changes": []map[string]any{},
			},
		}, nil
	}

	body := strings.Join(lines, "\n")
	if len(diffs) > 0 {
		body += "\n" + strings.Join(diffs, "\n")
	}
	return domain.ToolResult{
		Content: body,
		Meta: map[string]any{
			"changed":      true,
			"file_changes": changeMeta,
		},
	}, nil
}

func parseEditBatchItems(raw any) ([]editBatchItem, error) {
	if raw == nil {
		return nil, fmt.Errorf("edits is required")
	}
	var rows []any
	switch typed := raw.(type) {
	case []any:
		rows = typed
	case []map[string]any:
		rows = make([]any, len(typed))
		for i, row := range typed {
			rows[i] = row
		}
	default:
		return nil, fmt.Errorf("edits must be an array of {path, oldString, newString}")
	}
	items := make([]editBatchItem, 0, len(rows))
	for i, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("edits[%d] must be an object", i)
		}
		path, _ := m["path"].(string)
		oldStr, _ := m["oldString"].(string)
		newStr, _ := m["newString"].(string)
		items = append(items, editBatchItem{
			path:       path,
			old:        oldStr,
			new:        newStr,
			replaceAll: boolFromInput(m, "replaceAll"),
		})
	}
	return items, nil
}

func restoreBatchWrites(written []batchFileState) {
	for i := len(written) - 1; i >= 0; i-- {
		st := written[i]
		mode := st.mode
		if mode == 0 {
			mode = 0o644
		}
		_ = os.WriteFile(st.resolved, st.raw, mode)
	}
}

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
	defaultPatchFuzz = 40
	maxPatchFuzz     = 200
)

type ApplyPatch struct{}

func (h *ApplyPatch) Name() string                { return "apply_patch" }
func (h *ApplyPatch) RiskLevel() domain.RiskLevel { return domain.RiskMedium }
func (h *ApplyPatch) Describe(args map[string]any) string {
	patch, _ := args["patch"].(string)
	n := strings.Count(patch, updateFileMarker) + strings.Count(patch, addFileMarker) + strings.Count(patch, deleteFileMarker)
	if n == 0 {
		n = strings.Count(patch, "+++ ")
	}
	if n > 0 {
		return fmt.Sprintf("apply_patch (%d file op(s))", n)
	}
	return "apply_patch"
}
func (h *ApplyPatch) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "apply_patch",
		Description: "Applies a patch to one or more files. **Preferred** for multi-hunk or multi-file edits (use instead of many edit calls).\n\n" +
			"**Preferred format** (Codex-style begin-patch — easiest for models):\n" +
			"```\n" +
			"*** Begin Patch\n" +
			"*** Update File: relative/path.go\n" +
			"@@ optional_anchor_or_function_name\n" +
			" context line\n" +
			"-old line\n" +
			"+new line\n" +
			" context line\n" +
			"*** Add File: relative/new.go\n" +
			"+package main\n" +
			"*** Delete File: relative/old.go\n" +
			"*** End Patch\n" +
			"```\n\n" +
			"Rules:\n" +
			"- Paths MUST be relative to the project root (never absolute).\n" +
			"- Update hunks start with `@@` (optional anchor text after @@ helps locate the region).\n" +
			"- Each hunk line MUST start with ` ` (context), `-` (remove), or `+` (add).\n" +
			"- Add File lines are all `+` content. Delete File has no body.\n" +
			"- Optional `*** Move to: new/path` may follow Update File.\n" +
			"- Matching is search-based (no line numbers required) and tolerates small whitespace/indent drift.\n" +
			"- Also accepts classic unified diffs (`---/`+++`/`@@ -l,s +l,s @@`) as a fallback.\n" +
			"- Read target files first when updating existing content.\n" +
			"- Prefer this over multiple edit calls when changing several places at once.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"patch": map[string]any{
					"type":        "string",
					"description": "Begin-patch envelope (preferred) or unified diff string",
				},
				"fuzz": map[string]any{
					"type":        "integer",
					"description": "Max lines to search around an expected location (default: " + fmt.Sprintf("%d", defaultPatchFuzz) + ", max: " + fmt.Sprintf("%d", maxPatchFuzz) + ")",
				},
				"create_if_missing": map[string]any{
					"type":        "boolean",
					"description": "For unified-diff creates: allow creating the file if missing (default: false). Begin-patch *** Add File always creates.",
				},
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
	anchor             string // @@ header text (begin-patch)
	searchBased        bool   // locate by content, ignore oldStart when searching
	endOfFile          bool
}

type hunkLine struct {
	op   byte
	text string
}

type filePatch struct {
	path        string // absolute after resolve
	relPath     string // project-relative path for Meta / tracking
	hunks       []hunk
	isCreate    bool
	isDelete    bool
	moveTo      string // relative rename target (begin-patch)
	searchBased bool
	oldData     []byte
	oldText     string       // decoded/normalized original content (updates)
	meta        textFileMeta // decoded encoding/line ending (updates)
	mode        os.FileMode  // original permission bits (updates)
}

type pendingWrite struct {
	path    string
	data    []byte
	oldData []byte
	mode    os.FileMode
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
	var patches []filePatch
	var err error
	if looksLikeBeginPatch(patch) {
		patches, err = parseBeginPatch(patch)
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("invalid begin-patch: %w\n\nExpected:\n*** Begin Patch\n*** Update File: relative/path\n@@\n context\n-old\n+new\n*** End Patch", err)
		}
	} else {
		patches, err = parsePatch(patch)
		if err != nil {
			return domain.ToolResult{}, fmt.Errorf("invalid patch: %w\n\nPrefer begin-patch format:\n*** Begin Patch\n*** Update File: path\n@@\n context\n-old\n+new\n*** End Patch", err)
		}
	}
	if len(patches) == 0 {
		return domain.ToolResult{Content: "No files to patch"}, nil
	}

	var results []string
	var writes []pendingWrite
	var changeMeta []map[string]any

	for i := range patches {
		fp := &patches[i]
		fp.relPath = fp.path

		if fp.isCreate {
			fp.path, err = resolveWritePath(workDir, fp.path)
			if err != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot create file %q: %w", fp.relPath, err)
			}
			if _, statErr := os.Stat(fp.path); statErr == nil && !createIfMissing {
				return domain.ToolResult{}, fmt.Errorf("cannot create file %q: already exists. Use *** Update File / edit, or create_if_missing=true to overwrite", fp.relPath)
			}
			if mkErr := os.MkdirAll(filepath.Dir(fp.path), 0755); mkErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot create parent dirs for %q: %w", fp.relPath, mkErr)
			}
		} else {
			fp.path, err = resolveWritePath(workDir, fp.path)
			if err != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot resolve path %q: %w", fp.relPath, err)
			}
		}
		if fp.moveTo != "" {
			if _, moveErr := resolveWritePath(workDir, fp.moveTo); moveErr != nil {
				moveAbs := filepath.Clean(filepath.Join(workDir, fp.moveTo))
				rel, relErr := filepath.Rel(workDir, moveAbs)
				if relErr != nil || strings.HasPrefix(rel, "..") {
					return domain.ToolResult{}, fmt.Errorf("Move to %q is outside project", fp.moveTo)
				}
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
			if info, statErr := os.Stat(fp.path); statErr == nil {
				fp.mode = info.Mode().Perm()
			}
			writes = append(writes, pendingWrite{path: fp.path, data: nil, oldData: data, mode: fp.mode})
			continue
		}

		if fp.isCreate {
			// With create_if_missing=true the target may already exist —
			// capture its content so a rollback restores it instead of
			// deleting the pre-existing file.
			var prev []byte
			if data, readErr := os.ReadFile(fp.path); readErr == nil {
				prev = data
			}
			writes = append(writes, pendingWrite{path: fp.path, data: []byte{}, oldData: prev})
		} else {
			data, readErr := os.ReadFile(fp.path)
			if readErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot read file %q: %w. Hint: use *** Add File for new files, or check the relative path", fp.relPath, readErr)
			}
			text, meta, decErr := decodeTextFile(data)
			if decErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot patch %q: %w", fp.relPath, decErr)
			}
			fp.oldData = data
			fp.meta = meta
			if info, statErr := os.Stat(fp.path); statErr == nil {
				fp.mode = info.Mode().Perm()
			}
			fp.oldText = text

			lines := splitPatchFileLines(text)
			if _, applyErr := applyHunks(lines, fp.hunks, fuzz); applyErr != nil {
				return domain.ToolResult{}, fmt.Errorf("cannot apply patch to %q: %w\n\n%s", fp.relPath, applyErr, patchApplyHint(text, fp.hunks))
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
		var applyErr error
		if fp.isCreate {
			// Create patches are not pre-validated in the read phase; a bad
			// hunk here must fail loudly, not silently write an empty file.
			newLines, applyErr = applyHunks(nil, fp.hunks, fuzz)
		} else {
			newLines, applyErr = applyHunks(splitPatchFileLines(fp.oldText), fp.hunks, fuzz)
		}
		if applyErr != nil {
			h.rollbackWrites(writes)
			return domain.ToolResult{}, fmt.Errorf("cannot apply patch to %q: %w", fp.relPath, applyErr)
		}

		newContent := joinPatchFileLines(newLines)
		if !fp.isCreate && fp.moveTo == "" {
			if diff := generateUnifiedDiff(fp.relPath, fp.oldText, newContent); diff == "" {
				// The patch's net result is identical to the current file
				// (e.g., -X +X or context-only hunks): report an explicit
				// no-op instead of rewriting and claiming success.
				results = append(results, fmt.Sprintf("No changes to %q — patch results in identical content", fp.relPath))
				continue
			}
		}
		finalPath := fp.path
		finalRel := fp.relPath
		if fp.moveTo != "" {
			finalPath = filepath.Clean(filepath.Join(workDir, fp.moveTo))
			finalRel = fp.moveTo
			if mkErr := os.MkdirAll(filepath.Dir(finalPath), 0755); mkErr != nil {
				h.rollbackWrites(writes)
				return domain.ToolResult{}, fmt.Errorf("cannot create parent dirs for Move to %q: %w", fp.moveTo, mkErr)
			}
		}

		var payload []byte
		if fp.isCreate {
			payload = []byte(newContent)
		} else {
			payload = encodeTextFile(newContent, writeEncodingMeta(fp.meta))
		}
		prevAtFinal := fp.oldData
		if fp.moveTo != "" && finalPath != fp.path {
			// The destination of a Move did not exist before this patch:
			// rollback must remove it, not rewrite the source's old content there.
			prevAtFinal = nil
		}
		writes = append(writes, pendingWrite{
			path:    finalPath,
			data:    payload,
			oldData: prevAtFinal,
			mode:    fp.mode,
		})

		if err := writeFilePreserving(finalPath, payload); err != nil {
			h.rollbackWrites(writes)
			return domain.ToolResult{}, fmt.Errorf("cannot write file %q: %w", finalRel, err)
		}
		if fp.moveTo != "" && finalPath != fp.path {
			if rmErr := os.Remove(fp.path); rmErr != nil && !os.IsNotExist(rmErr) {
				h.rollbackWrites(writes)
				return domain.ToolResult{}, fmt.Errorf("patched but failed to remove old path %q after Move to: %w", fp.relPath, rmErr)
			}
			// Register the source removal so a later failure restores it.
			writes = append(writes, pendingWrite{path: fp.path, data: nil, oldData: fp.oldData, mode: fp.mode})
		}
		// Own write updates the snapshot so a later edit/write in this turn does not
		// fail with "changed since last read" unless something else touched the file.
		noteReadFile(input, finalPath)

		opLabel := "Patched"
		if fp.isCreate {
			opLabel = "Created"
		}
		msg := fmt.Sprintf("%s %q (%d hunks)", opLabel, finalRel, len(fp.hunks))
		if fp.moveTo != "" {
			msg = fmt.Sprintf("Patched %q → moved to %q (%d hunks)", fp.relPath, finalRel, len(fp.hunks))
		}
		if !fp.isCreate {
			msg += encodingNote(fp.meta)
		}
		results = append(results, msg)
		op := "update"
		oldStr := ""
		if fp.isCreate {
			op = "create"
		} else {
			oldStr = fp.oldText
		}
		diff := generateUnifiedDiff(finalRel, oldStr, newContent)
		changeMeta = append(changeMeta, map[string]any{
			"path": finalRel, "op": op, "diff": diff, "bytes_written": len(newContent),
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
			mode := os.FileMode(0o644)
			if w.mode != 0 {
				mode = w.mode
			}
			os.WriteFile(w.path, w.oldData, mode)
		}
	}
}

func splitPatchFileLines(data string) []string {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	if data == "" {
		return nil
	}
	// Preserve whether file ended with newline via joinPatchFileLines.
	return strings.Split(strings.TrimSuffix(data, "\n"), "\n")
}

func joinPatchFileLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func patchApplyHint(content string, hunks []hunk) string {
	var b strings.Builder
	b.WriteString("Hints: re-read the file; ensure context/- lines match current content; use a unique @@ anchor; keep more surrounding context lines.")
	for _, h := range hunks {
		oldSide := hunkOldSide(h)
		if len(oldSide) == 0 {
			continue
		}
		needle := strings.Join(oldSide, "\n")
		matches := findClosestContentMatches(content, needle, 1)
		if len(matches) == 0 {
			continue
		}
		m := matches[0]
		fmt.Fprintf(&b, "\n\nClosest region for a failing hunk (lines %d-%d, similarity %.0f%%):\n", m.startLine, m.endLine, m.score*100)
		for j, line := range strings.Split(m.snippet, "\n") {
			fmt.Fprintf(&b, "%d| %s\n", m.startLine+j, truncateLine(line, 160))
		}
		break
	}
	return b.String()
}

func hunkOldSide(h hunk) []string {
	var out []string
	for _, hl := range h.lines {
		if hl.op == ' ' || hl.op == '-' {
			out = append(out, hl.text)
		}
	}
	return out
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
			hh := hunk{lines: make([]hunkLine, 0)}
			hh.oldStart = parseInt(m[1])
			if m[2] != "" {
				hh.oldCount = parseInt(m[2])
			} else {
				hh.oldCount = 1
			}
			hh.newStart = parseInt(m[3])
			if m[4] != "" {
				hh.newCount = parseInt(m[4])
			} else {
				hh.newCount = 1
			}
			if len(m) > 5 {
				hh.anchor = strings.TrimSpace(m[5])
			}
			cur.hunks = append(cur.hunks, hh)
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
	// Search-based patches (begin-patch): apply sequentially by content locate.
	if len(hunks) > 0 && hunks[0].searchBased {
		return applySearchHunks(lines, hunks)
	}

	result := make([]string, 0, len(lines)+len(hunks)*10)
	originalLen := len(lines)
	offset := 0

	for hi, hh := range hunks {
		oldIdx := hh.oldStart - 1 - offset
		if oldIdx < 0 {
			oldIdx = 0
		}

		matchedIdx := findHunkMatch(lines, hh, oldIdx, fuzz)
		if matchedIdx < 0 {
			return nil, fmt.Errorf("hunk %d mismatch: cannot find context around line %d", hi+1, hh.oldStart)
		}
		oldIdx = matchedIdx

		for len(result) < oldIdx {
			result = append(result, lines[len(result)])
		}

		newIdx := oldIdx + hh.oldCount
		offset += hh.oldCount - hh.newCount

		applied, nextOld, err := applyHunkAt(lines, oldIdx, hh)
		if err != nil {
			return nil, fmt.Errorf("hunk %d: %w", hi+1, err)
		}
		result = append(result, applied...)
		oldIdx = nextOld

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

func applySearchHunks(lines []string, hunks []hunk) ([]string, error) {
	cur := append([]string(nil), lines...)
	cursor := 0
	for hi, hh := range hunks {
		if hh.isCreateOnly() {
			// Pure additions with no old side — append (Add File path).
			for _, hl := range hh.lines {
				if hl.op == '+' {
					cur = append(cur, hl.text)
				}
			}
			continue
		}

		startFrom := cursor
		if hh.anchor != "" {
			anchorIdx := findAnchorLine(cur, hh.anchor, startFrom)
			if anchorIdx < 0 {
				anchorIdx = findAnchorLine(cur, hh.anchor, 0)
			}
			if anchorIdx >= 0 {
				startFrom = anchorIdx
			}
		}
		if hh.endOfFile {
			// Prefer matching near EOF.
			oldSide := hunkOldSide(hh)
			if len(oldSide) > 0 && len(cur) >= len(oldSide) {
				startFrom = len(cur) - len(oldSide)
			}
		}

		matchIdx := findOldSideMatch(cur, hh, startFrom)
		if matchIdx < 0 && startFrom > 0 {
			matchIdx = findOldSideMatch(cur, hh, 0)
		}
		if matchIdx < 0 {
			return nil, fmt.Errorf("hunk %d: cannot locate old context%s", hi+1, formatAnchor(hh.anchor))
		}

		// Verify old side still matches at matchIdx with fuzzy equality.
		if !oldSideMatchesAt(cur, matchIdx, hh) {
			return nil, fmt.Errorf("hunk %d: context/removal mismatch at line %d%s", hi+1, matchIdx+1, formatAnchor(hh.anchor))
		}
		replacement, oldLen := buildReplacementFromFile(cur, matchIdx, hh)
		next := make([]string, 0, len(cur)-oldLen+len(replacement))
		next = append(next, cur[:matchIdx]...)
		next = append(next, replacement...)
		next = append(next, cur[matchIdx+oldLen:]...)
		cur = next
		cursor = matchIdx + len(replacement)
	}
	return cur, nil
}

func (h hunk) isCreateOnly() bool {
	if len(h.lines) == 0 {
		return true
	}
	for _, hl := range h.lines {
		if hl.op != '+' {
			return false
		}
	}
	return true
}

func formatAnchor(anchor string) string {
	if anchor == "" {
		return ""
	}
	return fmt.Sprintf(" (@@ %s)", anchor)
}

func findAnchorLine(lines []string, anchor string, from int) int {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return -1
	}
	normAnchor := normalizeWhitespace(anchor)
	for i := from; i < len(lines); i++ {
		if lines[i] == anchor || strings.Contains(lines[i], anchor) {
			return i
		}
		if normalizeWhitespace(lines[i]) == normAnchor || strings.Contains(normalizeWhitespace(lines[i]), normAnchor) {
			return i
		}
	}
	return -1
}

func findOldSideMatch(lines []string, h hunk, from int) int {
	oldSide := hunkOldSide(h)
	if len(oldSide) == 0 {
		// Pure insertion: place after anchor (from), or EOF.
		if from < 0 {
			return len(lines)
		}
		return from
	}
	for i := from; i+len(oldSide) <= len(lines); i++ {
		if oldSideEquals(lines[i:i+len(oldSide)], oldSide, matchExact) ||
			oldSideEquals(lines[i:i+len(oldSide)], oldSide, matchIndent) ||
			oldSideEquals(lines[i:i+len(oldSide)], oldSide, matchWhitespace) {
			return i
		}
	}
	return -1
}

func oldSideMatchesAt(lines []string, idx int, h hunk) bool {
	oldSide := hunkOldSide(h)
	if len(oldSide) == 0 {
		return true
	}
	if idx < 0 || idx+len(oldSide) > len(lines) {
		return false
	}
	return oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchExact) ||
		oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchIndent) ||
		oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchWhitespace)
}

type matchMode int

const (
	matchExact matchMode = iota
	matchIndent
	matchWhitespace
)

func oldSideEquals(have, want []string, mode matchMode) bool {
	if len(have) != len(want) {
		return false
	}
	for i := range have {
		switch mode {
		case matchExact:
			if have[i] != want[i] {
				return false
			}
		case matchIndent:
			if stripLeadingWhitespace(have[i]) != stripLeadingWhitespace(want[i]) {
				return false
			}
		case matchWhitespace:
			if normalizeWhitespace(have[i]) != normalizeWhitespace(want[i]) {
				return false
			}
		}
	}
	return true
}

func buildReplacementFromFile(lines []string, idx int, h hunk) ([]string, int) {
	oldLen := 0
	var out []string
	at := idx
	for _, hl := range h.lines {
		switch hl.op {
		case ' ':
			if at < len(lines) {
				out = append(out, lines[at]) // keep file whitespace on fuzzy context match
			} else {
				out = append(out, hl.text)
			}
			oldLen++
			at++
		case '-':
			oldLen++
			at++
		case '+':
			out = append(out, hl.text)
		}
	}
	return out, oldLen
}

func applyHunkAt(lines []string, oldIdx int, h hunk) ([]string, int, error) {
	var out []string
	for _, hl := range h.lines {
		switch hl.op {
		case ' ':
			if oldIdx >= len(lines) || !lineMatchFuzzy(lines[oldIdx], hl.text) {
				return nil, oldIdx, fmt.Errorf("context mismatch at line %d: expected %q, got %q", oldIdx+1, hl.text, safeGet(lines, oldIdx))
			}
			out = append(out, lines[oldIdx]) // keep file's original whitespace when fuzzy
			oldIdx++
		case '-':
			if oldIdx >= len(lines) || !lineMatchFuzzy(lines[oldIdx], hl.text) {
				return nil, oldIdx, fmt.Errorf("removal mismatch at line %d: expected %q, got %q", oldIdx+1, hl.text, safeGet(lines, oldIdx))
			}
			oldIdx++
		case '+':
			out = append(out, hl.text)
		}
	}
	return out, oldIdx, nil
}

func lineMatchFuzzy(have, want string) bool {
	if have == want {
		return true
	}
	if stripLeadingWhitespace(have) == stripLeadingWhitespace(want) {
		return true
	}
	return normalizeWhitespace(have) == normalizeWhitespace(want)
}

func findHunkMatch(lines []string, h hunk, startIdx, fuzz int) int {
	if len(lines) == 0 {
		return startIdx
	}

	// Prefer first unique old-side match near startIdx.
	oldSide := hunkOldSide(h)
	if len(oldSide) == 0 {
		return startIdx
	}

	best := -1
	bestDist := fuzz + 1
	searchLo := startIdx - fuzz
	if searchLo < 0 {
		searchLo = 0
	}
	searchHi := startIdx + fuzz
	if searchHi > len(lines) {
		searchHi = len(lines)
	}
	for idx := searchLo; idx+len(oldSide) <= len(lines) && idx <= searchHi; idx++ {
		if oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchExact) ||
			oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchIndent) ||
			oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchWhitespace) {
			dist := idx - startIdx
			if dist < 0 {
				dist = -dist
			}
			if dist < bestDist {
				bestDist = dist
				best = idx
			}
		}
	}
	if best >= 0 {
		return best
	}

	// Fallback: search whole file when fuzz window failed (unified diffs with bad line numbers).
	for idx := 0; idx+len(oldSide) <= len(lines); idx++ {
		if oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchExact) ||
			oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchIndent) ||
			oldSideEquals(lines[idx:idx+len(oldSide)], oldSide, matchWhitespace) {
			return idx
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

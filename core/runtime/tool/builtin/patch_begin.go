package builtin

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	beginPatchMarker = "*** Begin Patch"
	endPatchMarker   = "*** End Patch"
	addFileMarker    = "*** Add File: "
	deleteFileMarker = "*** Delete File: "
	updateFileMarker = "*** Update File: "
	moveToMarker     = "*** Move to: "
	endOfFileMarker  = "*** End of File"
)

func looksLikeBeginPatch(patch string) bool {
	s := strings.TrimSpace(patch)
	return strings.Contains(s, beginPatchMarker) ||
		strings.Contains(s, updateFileMarker) ||
		strings.Contains(s, addFileMarker) ||
		strings.Contains(s, deleteFileMarker)
}

// parseBeginPatch parses the Codex-style apply_patch envelope.
// Relative paths only; hunks use @@ anchors (optional) without requiring line numbers.
func parseBeginPatch(patch string) ([]filePatch, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	// Trim a single trailing empty line from Split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) {
		return nil, fmt.Errorf("empty patch")
	}
	// Allow missing Begin marker when a file op header is present.
	if strings.TrimSpace(lines[start]) == beginPatchMarker {
		start++
	}

	var patches []filePatch
	i := start
	for i < len(lines) {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if trim == "" || trim == endPatchMarker {
			i++
			continue
		}

		switch {
		case strings.HasPrefix(line, addFileMarker):
			path := strings.TrimSpace(strings.TrimPrefix(line, addFileMarker))
			if path == "" {
				return nil, fmt.Errorf("%s missing path at line %d", strings.TrimSpace(addFileMarker), i+1)
			}
			fp := filePatch{path: filepath.Clean(path), isCreate: true, searchBased: true}
			i++
			var added []hunkLine
			for i < len(lines) {
				l := lines[i]
				if isBeginFileOpHeader(l) || strings.TrimSpace(l) == endPatchMarker {
					break
				}
				if strings.HasPrefix(l, "+") {
					added = append(added, hunkLine{op: '+', text: l[1:]})
				} else if strings.TrimSpace(l) == "" {
					// Ignore blank separators between ops.
					i++
					continue
				} else {
					return nil, fmt.Errorf("Add File %q: expected '+' content lines, got %q at line %d", path, l, i+1)
				}
				i++
			}
			if len(added) == 0 {
				// Allow empty new file.
				fp.hunks = []hunk{{searchBased: true, lines: nil}}
			} else {
				fp.hunks = []hunk{{searchBased: true, lines: added}}
			}
			patches = append(patches, fp)

		case strings.HasPrefix(line, deleteFileMarker):
			path := strings.TrimSpace(strings.TrimPrefix(line, deleteFileMarker))
			if path == "" {
				return nil, fmt.Errorf("%s missing path at line %d", strings.TrimSpace(deleteFileMarker), i+1)
			}
			patches = append(patches, filePatch{
				path:        filepath.Clean(path),
				isDelete:    true,
				searchBased: true,
			})
			i++

		case strings.HasPrefix(line, updateFileMarker):
			path := strings.TrimSpace(strings.TrimPrefix(line, updateFileMarker))
			if path == "" {
				return nil, fmt.Errorf("%s missing path at line %d", strings.TrimSpace(updateFileMarker), i+1)
			}
			fp := filePatch{path: filepath.Clean(path), searchBased: true}
			i++
			if i < len(lines) && strings.HasPrefix(lines[i], moveToMarker) {
				fp.moveTo = strings.TrimSpace(strings.TrimPrefix(lines[i], moveToMarker))
				i++
			}
			hunks, next, err := parseBeginUpdateHunks(lines, i)
			if err != nil {
				return nil, fmt.Errorf("Update File %q: %w", path, err)
			}
			if len(hunks) == 0 {
				return nil, fmt.Errorf("Update File %q: no hunks (need @@ ... then context/-/+ lines)", path)
			}
			fp.hunks = hunks
			patches = append(patches, fp)
			i = next

		default:
			return nil, fmt.Errorf("unexpected line in begin-patch at %d: %q (expected *** Add/Update/Delete File)", i+1, line)
		}
	}

	if len(patches) == 0 {
		return nil, fmt.Errorf("begin-patch contained no file operations")
	}
	return patches, nil
}

func isBeginFileOpHeader(line string) bool {
	return strings.HasPrefix(line, addFileMarker) ||
		strings.HasPrefix(line, deleteFileMarker) ||
		strings.HasPrefix(line, updateFileMarker)
}

func parseBeginUpdateHunks(lines []string, start int) ([]hunk, int, error) {
	var hunks []hunk
	i := start
	for i < len(lines) {
		line := lines[i]
		if isBeginFileOpHeader(line) || strings.TrimSpace(line) == endPatchMarker {
			break
		}
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "@@") {
			return nil, i, fmt.Errorf("expected @@ hunk header at line %d, got %q", i+1, line)
		}
		anchor := strings.TrimSpace(strings.TrimPrefix(line, "@@"))
		i++
		h := hunk{searchBased: true, anchor: anchor}
		for i < len(lines) {
			l := lines[i]
			if strings.HasPrefix(l, "@@") || isBeginFileOpHeader(l) || strings.TrimSpace(l) == endPatchMarker {
				break
			}
			if strings.TrimSpace(l) == endOfFileMarker {
				h.endOfFile = true
				i++
				break
			}
			if l == "" {
				// Empty line without prefix — treat as context "" (rare but valid).
				h.lines = append(h.lines, hunkLine{op: ' ', text: ""})
				i++
				continue
			}
			switch l[0] {
			case ' ', '-', '+':
				h.lines = append(h.lines, hunkLine{op: l[0], text: l[1:]})
			default:
				return nil, i, fmt.Errorf("invalid hunk line at %d (must start with ' ', '-', or '+'): %q", i+1, l)
			}
			i++
		}
		if len(h.lines) == 0 && anchor == "" {
			return nil, i, fmt.Errorf("empty hunk with no @@ anchor")
		}
		// Derive old/new counts for diagnostics.
		for _, hl := range h.lines {
			switch hl.op {
			case ' ', '-':
				h.oldCount++
			}
			switch hl.op {
			case ' ', '+':
				h.newCount++
			}
		}
		hunks = append(hunks, h)
	}
	return hunks, i, nil
}

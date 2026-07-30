package service

import (
	"fmt"
	"strconv"
	"strings"
)

type parsedHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	OldLines []string // without '-' prefix (context + deletes)
	NewLines []string // without '+' prefix (context + inserts)
}

// applySelectedHunks rebuilds content:
//   - acceptAll → current
//   - no indexes → old (full reject)
//   - otherwise start from current and reverse-apply rejected hunks
func applySelectedHunks(oldContent, currentContent, patch string, acceptAll bool, hunkIndexes []int) (string, error) {
	if acceptAll {
		return currentContent, nil
	}
	hunks, err := parseUnifiedHunks(patch)
	if err != nil {
		return "", err
	}
	if len(hunkIndexes) == 0 {
		return oldContent, nil
	}
	accept := map[int]bool{}
	for _, i := range hunkIndexes {
		if i < 0 || i >= len(hunks) {
			return "", fmt.Errorf("hunk index out of range: %d", i)
		}
		accept[i] = true
	}

	lines := splitKeepNoTrailingEmpty(currentContent)
	// Reverse rejected hunks from bottom to top using new-side coordinates.
	for i := len(hunks) - 1; i >= 0; i-- {
		if accept[i] {
			continue
		}
		h := hunks[i]
		start := h.NewStart - 1
		if start < 0 {
			start = 0
		}
		end := start + h.NewCount
		if start > len(lines) {
			start = len(lines)
			end = start
		}
		if end > len(lines) {
			end = len(lines)
		}
		var next []string
		next = append(next, lines[:start]...)
		next = append(next, h.OldLines...)
		next = append(next, lines[end:]...)
		lines = next
	}
	trailing := strings.HasSuffix(oldContent, "\n") || strings.HasSuffix(currentContent, "\n")
	return joinLines(lines, trailing), nil
}

func parseUnifiedHunks(patch string) ([]parsedHunk, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	var hunks []parsedHunk
	var cur *parsedHunk
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			cur = &h
			continue
		}
		if cur == nil {
			continue
		}
		if line == `\ No newline at end of file` {
			continue
		}
		if line == "" {
			continue
		}
		switch line[0] {
		case ' ':
			cur.OldLines = append(cur.OldLines, line[1:])
			cur.NewLines = append(cur.NewLines, line[1:])
		case '-':
			cur.OldLines = append(cur.OldLines, line[1:])
		case '+':
			cur.NewLines = append(cur.NewLines, line[1:])
		}
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}
	return hunks, nil
}

func parseHunkHeader(line string) (parsedHunk, error) {
	var h parsedHunk
	parts := strings.Split(line, "@@")
	if len(parts) < 2 {
		return h, fmt.Errorf("bad hunk header: %s", line)
	}
	body := strings.TrimSpace(parts[1])
	fields := strings.Fields(body)
	if len(fields) < 2 {
		return h, fmt.Errorf("bad hunk header: %s", line)
	}
	os, oc, err := parseRange(strings.TrimPrefix(fields[0], "-"))
	if err != nil {
		return h, err
	}
	ns, nc, err := parseRange(strings.TrimPrefix(fields[1], "+"))
	if err != nil {
		return h, err
	}
	h.OldStart, h.OldCount = os, oc
	h.NewStart, h.NewCount = ns, nc
	return h, nil
}

func parseRange(s string) (start, count int, err error) {
	if strings.Contains(s, ",") {
		parts := strings.SplitN(s, ",", 2)
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return
		}
		count, err = strconv.Atoi(parts[1])
		return
	}
	start, err = strconv.Atoi(s)
	count = 1
	return
}

func splitKeepNoTrailingEmpty(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string, trailingNL bool) string {
	if len(lines) == 0 {
		return ""
	}
	out := strings.Join(lines, "\n")
	if trailingNL {
		out += "\n"
	}
	return out
}

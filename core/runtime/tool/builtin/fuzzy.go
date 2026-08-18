package builtin

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"unicode"

	"danmo-work/core/textdiff"
)

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = minInt(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[la][lb]
}

func minInt(vals ...int) int {
	m := math.MaxInt
	for _, v := range vals {
		if v < m {
			m = v
		}
	}
	return m
}

func normalizeWhitespace(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteRune(' ')
				inSpace = true
			}
		} else {
			b.WriteRune(r)
			inSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

func stripLeadingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = strings.TrimLeft(line, " \t")
	}
	return strings.Join(result, "\n")
}

func fuzzyFileSuggestions(absDir, missingName string) []string {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil
	}
	type candidate struct {
		name     string
		distance int
	}
	var cands []candidate
	lowerTarget := strings.ToLower(missingName)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lowerName := strings.ToLower(e.Name())
		d := levenshtein(lowerName, lowerTarget)
		maxLen := len(e.Name())
		if len(missingName) > maxLen {
			maxLen = len(missingName)
		}
		if d <= maxLen/2 {
			cands = append(cands, candidate{e.Name(), d})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].distance < cands[j].distance })
	if len(cands) > 3 {
		cands = cands[:3]
	}
	names := make([]string, len(cands))
	for i, c := range cands {
		names[i] = c.name
	}
	return names
}

func generateUnifiedDiff(path, oldContent, newContent string) string {
	return textdiff.Unified(path, oldContent, newContent)
}

func tryExactReplace(content, oldStr, newStr string, replaceAll bool) (string, int, error) {
	count := strings.Count(content, oldStr)
	if count == 0 {
		return "", 0, fmt.Errorf("oldString not found in content")
	}
	if !replaceAll && count > 1 {
		return "", count, fmt.Errorf("found %d occurrences of oldString; set replaceAll=true to replace all, or use more context to make oldString unique", count)
	}
	result := strings.ReplaceAll(content, oldStr, newStr)
	return result, count, nil
}

func tryIndentFuzzyReplace(content, oldStr, newStr string, replaceAll bool) (string, int, error) {
	oldNormalized := stripLeadingWhitespace(oldStr)
	contentLines := strings.Split(content, "\n")

	oldLines := strings.Split(oldStr, "\n")
	var matchStart int = -1
	for i := 0; i <= len(contentLines)-len(oldLines); i++ {
		candidate := strings.Join(contentLines[i:i+len(oldLines)], "\n")
		if stripLeadingWhitespace(candidate) == oldNormalized {
			matchStart = i
			break
		}
	}
	if matchStart == -1 {
		return "", 0, fmt.Errorf("oldString not found after indentation normalization")
	}

	oldPart := strings.Join(contentLines[matchStart:matchStart+len(oldLines)], "\n")
	result := strings.Replace(content, oldPart, newStr, 1)
	return result, 1, nil
}

func tryWhitespaceFuzzyReplace(content, oldStr, newStr string, replaceAll bool) (string, int, error) {
	oldNormalized := normalizeWhitespace(oldStr)
	contentLines := strings.Split(content, "\n")
	oldLines := strings.Split(oldStr, "\n")

	var matchStart int = -1
	for i := 0; i <= len(contentLines)-len(oldLines); i++ {
		candidate := strings.Join(contentLines[i:i+len(oldLines)], "\n")
		if normalizeWhitespace(candidate) == oldNormalized {
			matchStart = i
			break
		}
	}
	if matchStart == -1 {
		return "", 0, fmt.Errorf("oldString not found after whitespace normalization")
	}

	oldPart := strings.Join(contentLines[matchStart:matchStart+len(oldLines)], "\n")
	result := strings.Replace(content, oldPart, newStr, 1)
	return result, 1, nil
}

type closestMatch struct {
	startLine int // 1-based
	endLine   int
	snippet   string
	score     float64 // 0..1, higher is closer
}

// findClosestContentMatches finds windows in content that most resemble oldStr.
// Used only for actionable edit error messages (does not change match behavior).
func findClosestContentMatches(content, oldStr string, maxCandidates int) []closestMatch {
	if maxCandidates <= 0 {
		maxCandidates = 2
	}
	oldLines := splitLines(oldStr)
	if len(oldLines) == 0 {
		return nil
	}
	contentLines := splitLines(content)
	if len(contentLines) == 0 {
		return nil
	}

	targetNorm := normalizeWhitespace(oldStr)
	window := len(oldLines)
	// Cap scan cost on huge files / huge oldString.
	if window > 40 {
		window = 40
		oldLines = oldLines[:window]
		targetNorm = normalizeWhitespace(strings.Join(oldLines, "\n"))
	}
	maxScan := len(contentLines)
	if maxScan > 4000 {
		maxScan = 4000
	}

	type cand struct {
		m closestMatch
	}
	var best []cand
	consider := func(start, size int) {
		if start < 0 || size <= 0 || start+size > len(contentLines) || start >= maxScan {
			return
		}
		snippetLines := contentLines[start : start+size]
		snippet := strings.Join(snippetLines, "\n")
		norm := normalizeWhitespace(snippet)
		if norm == "" && targetNorm == "" {
			return
		}
		dist := levenshtein(norm, targetNorm)
		denom := len(norm)
		if len(targetNorm) > denom {
			denom = len(targetNorm)
		}
		if denom == 0 {
			return
		}
		score := 1.0 - float64(dist)/float64(denom)
		if score < 0.35 {
			return
		}
		m := closestMatch{
			startLine: start + 1,
			endLine:   start + size,
			snippet:   snippet,
			score:     score,
		}
		best = append(best, cand{m: m})
	}

	for i := 0; i <= len(contentLines)-window && i < maxScan; i++ {
		consider(i, window)
	}
	// Also try ±1 line windows when multi-line — models often drop/add a context line.
	if window > 1 {
		for i := 0; i <= len(contentLines)-(window-1) && i < maxScan; i++ {
			consider(i, window-1)
		}
	}
	if window+1 <= len(contentLines) {
		for i := 0; i <= len(contentLines)-(window+1) && i < maxScan; i++ {
			consider(i, window+1)
		}
	}

	sort.Slice(best, func(i, j int) bool {
		if best[i].m.score != best[j].m.score {
			return best[i].m.score > best[j].m.score
		}
		return best[i].m.startLine < best[j].m.startLine
	})

	// Dedup overlapping windows; keep highest score.
	out := make([]closestMatch, 0, maxCandidates)
	for _, c := range best {
		overlap := false
		for _, kept := range out {
			if c.m.startLine <= kept.endLine && c.m.endLine >= kept.startLine {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		out = append(out, c.m)
		if len(out) >= maxCandidates {
			break
		}
	}
	return out
}

func looksLikeLineNumberPrefix(s string) bool {
	lines := strings.Split(s, "\n")
	prefixed := 0
	checked := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		checked++
		i := 0
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i > 0 && i < len(line) && (line[i] == ':' || line[i] == '|') {
			prefixed++
		}
		if checked >= 3 {
			break
		}
	}
	return checked > 0 && prefixed == checked
}

func formatEditNotFoundError(relPath, content, oldStr string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "oldString not found in %q after exact and fuzzy matching.\n", relPath)

	if looksLikeLineNumberPrefix(oldStr) {
		b.WriteString("\nHint: oldString looks like it includes read_file line-number prefixes (e.g. \"12: \"). Those are NOT part of the file — strip them and retry.\n")
	}

	matches := findClosestContentMatches(content, oldStr, 2)
	if len(matches) > 0 {
		b.WriteString("\nClosest match(es) in the current file — compare carefully and retry with exact text from the file:\n")
		for i, m := range matches {
			fmt.Fprintf(&b, "\n[%d] lines %d-%d (similarity %.0f%%):\n", i+1, m.startLine, m.endLine, m.score*100)
			for j, line := range strings.Split(m.snippet, "\n") {
				fmt.Fprintf(&b, "%d| %s\n", m.startLine+j, truncateLine(line, 200))
			}
		}
	} else {
		b.WriteString("\nNo close match found. Re-read the file with read_file and copy oldString from the current contents.\n")
	}

	b.WriteString("\nTips: preserve exact indentation/whitespace; if the file changed, read_file again; for multi-hunk or multi-file edits prefer apply_patch.")
	return fmt.Errorf("%s", b.String())
}

func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatWithLineNumbers(lines []string, startLine int) string {
	var b strings.Builder
	for _, line := range lines {
		fmt.Fprintf(&b, "%d: %s\n", startLine, line)
		startLine++
	}
	return b.String()
}

func truncateLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen] + fmt.Sprintf("... (line truncated to %d chars)", maxLen)
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	checkLen := len(data)
	if checkLen > 4096 {
		checkLen = 4096
	}
	nonPrintable := 0
	for _, b := range data[:checkLen] {
		if b == 0 {
			return true
		}
		if b < 9 || (b > 13 && b < 32) {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(checkLen) > 0.3
}

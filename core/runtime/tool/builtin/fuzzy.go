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

func checkDisproportionateMatch(content, oldStr string) error {
	if oldStr == "" {
		return nil
	}
	idx := strings.Index(content, oldStr)
	if idx == -1 {
		return nil
	}
	matchedLen := len(content[idx : idx+len(oldStr)])
	oldLenMultiplied := 4 * len(oldStr)
	if matchedLen > oldLenMultiplied && oldLenMultiplied > 0 || matchedLen > 500 {
		return fmt.Errorf("disproportionate match: oldString matched a range of %d chars (original %d chars). Provide more specific oldString",
			matchedLen, len(oldStr))
	}
	return nil
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

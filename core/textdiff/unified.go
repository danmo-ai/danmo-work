// Package textdiff provides a small unified-diff helper shared by tools and AI review.
package textdiff

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Unified returns a unified diff for path between oldContent and newContent.
func Unified(path, oldContent, newContent string) string {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	if equalLines(oldLines, newLines) {
		return ""
	}

	ops := myersDiff(oldLines, newLines)

	var b strings.Builder
	b.WriteString("--- a/" + filepath.ToSlash(path) + "\n")
	b.WriteString("+++ b/" + filepath.ToSlash(path) + "\n")

	const ctx = 3
	i := 0
	for i < len(ops) {
		// Skip leading equals until a change (keep ctx before).
		for i < len(ops) && ops[i].kind == opEqual {
			i++
		}
		if i >= len(ops) {
			break
		}
		changeStart := i
		// Include context before.
		hunkStart := changeStart - ctx
		if hunkStart < 0 {
			hunkStart = 0
		}
		// Extend through changes and following equals (ctx after + next change cluster).
		j := changeStart
		for j < len(ops) {
			if ops[j].kind != opEqual {
				j++
				continue
			}
			// count following equals
			eq := 0
			k := j
			for k < len(ops) && ops[k].kind == opEqual {
				eq++
				k++
			}
			if k >= len(ops) {
				// trailing equals: keep ctx
				if eq > ctx {
					j += ctx
				} else {
					j = k
				}
				break
			}
			// next change exists — if equals gap <= 2*ctx, merge hunks
			if eq <= 2*ctx {
				j = k
				continue
			}
			j += ctx
			break
		}
		hunkEnd := j
		writeHunk(&b, ops, hunkStart, hunkEnd)
		i = hunkEnd
		// skip remaining equals already consumed as trailing context
		for i < len(ops) && ops[i].kind == opEqual {
			i++
		}
	}
	return b.String()
}

type opKind int

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

type diffOp struct {
	kind opKind
	line string
	// oldIdx/newIdx are 0-based indexes into the respective arrays (-1 if N/A).
	oldIdx int
	newIdx int
}

func writeHunk(b *strings.Builder, ops []diffOp, start, end int) {
	oldStart, newStart := 0, 0
	oldCount, newCount := 0, 0
	// Find first old/new index in range.
	oldStartSet, newStartSet := false, false
	for i := start; i < end; i++ {
		op := ops[i]
		switch op.kind {
		case opEqual:
			if !oldStartSet && op.oldIdx >= 0 {
				oldStart = op.oldIdx + 1
				oldStartSet = true
			}
			if !newStartSet && op.newIdx >= 0 {
				newStart = op.newIdx + 1
				newStartSet = true
			}
			oldCount++
			newCount++
		case opDelete:
			if !oldStartSet && op.oldIdx >= 0 {
				oldStart = op.oldIdx + 1
				oldStartSet = true
			}
			oldCount++
		case opInsert:
			if !newStartSet && op.newIdx >= 0 {
				newStart = op.newIdx + 1
				newStartSet = true
			}
			newCount++
		}
	}
	if !oldStartSet {
		oldStart = 0
	}
	if !newStartSet {
		newStart = 0
	}
	fmt.Fprintf(b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for i := start; i < end; i++ {
		op := ops[i]
		switch op.kind {
		case opEqual:
			b.WriteString(" " + op.line + "\n")
		case opDelete:
			b.WriteString("-" + op.line + "\n")
		case opInsert:
			b.WriteString("+" + op.line + "\n")
		}
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// myersDiff returns a simple LCS-based edit script (not full Myers bitset, but correct).
func myersDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	// DP LCS lengths
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			ops = append(ops, diffOp{kind: opEqual, line: a[i], oldIdx: i, newIdx: j})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOp{kind: opDelete, line: a[i], oldIdx: i, newIdx: -1})
			i++
		} else {
			ops = append(ops, diffOp{kind: opInsert, line: b[j], oldIdx: -1, newIdx: j})
			j++
		}
	}
	for i < n {
		ops = append(ops, diffOp{kind: opDelete, line: a[i], oldIdx: i, newIdx: -1})
		i++
	}
	for j < m {
		ops = append(ops, diffOp{kind: opInsert, line: b[j], oldIdx: -1, newIdx: j})
		j++
	}
	return ops
}

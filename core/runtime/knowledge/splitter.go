// Package knowledge implements Markdown chapter splitting and simple embeddings
// for the knowledge-base cold index (tiersum-inspired, no Bleve/HNSW/ONNX).
package knowledge

import (
	"regexp"
	"strings"
	"unicode"
)

// Chapter is a bottom-up extracted logical chapter from SplitMarkdown.
type Chapter struct {
	Path  string // docID/Title/... or docID/__root__
	Title string // leaf title or doc title
	Text  string // full logical chapter content (may be large)
}

// Chunk is a non-overlapping window of chapter content for indexing.
type Chunk struct {
	ID      string // chapterPath/01, chapterPath/02, ...
	Title   string
	Content string
}

type headingSpan struct {
	start int
	end   int // end of heading line (exclusive), body starts after
	level int
	title string
}

var (
	atxHeadingRe   = regexp.MustCompile(`^(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$`)
	chineseL2Re    = regexp.MustCompile(`^([一二三四五六七八九十百]+)、(.+)$`)
	chineseL3Re    = regexp.MustCompile(`^（([一二三四五六七八九十百]+)）(.+)$`)
	fenceLineRe    = regexp.MustCompile("^(`{3,}|~{3,})")
)

const defaultMaxTokens = 512
const defaultStrideTokens = 100

// SplitMarkdown splits markdown into logical chapters under docID.
// maxTokens controls bottom-up merge; chapters are NOT sliding-window split.
// Large chapters are kept intact — use SplitChunks for indexing purposes.
func SplitMarkdown(docID, docTitle, markdown string, maxTokens int) []Chapter {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	md := normalizeEOL(markdown)
	md = stripYAMLFrontmatter(md)
	spans := collectHeadingSpans(md)
	root := buildTree(md, spans)
	raw := extractChapters(root, maxTokens)
	out := make([]Chapter, 0, len(raw))
	for _, r := range raw {
		text := strings.TrimSpace(r.text)
		if text == "" {
			continue
		}
		titles := r.pathTitles
		if len(titles) == 0 {
			titles = []string{"__root__"}
		}
		path := docID + "/" + strings.Join(titles, "/")
		title := titles[len(titles)-1]
		if title == "__root__" || strings.TrimSpace(title) == "" {
			title = docTitle
		}
		if title == "" {
			title = docID
		}
		out = append(out, Chapter{Path: path, Title: title, Text: text})
	}
	if len(out) == 0 && strings.TrimSpace(md) != "" {
		path := docID + "/__root__"
		out = append(out, Chapter{Path: path, Title: docTitle, Text: strings.TrimSpace(md)})
	}
	return out
}

// SplitChunks splits chapter text into non-overlapping chunks for indexing.
// Chunk IDs are encoded as chapterPath/01, chapterPath/02, ...
func SplitChunks(chapterPath, chapterTitle, chapterText string, maxTokens int) []Chunk {
	text := strings.TrimSpace(chapterText)
	if text == "" {
		return nil
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	if EstimateTokens(text) <= maxTokens {
		return []Chunk{{ID: chapterPath + "/01", Title: chapterTitle, Content: text}}
	}
	windowRunes := maxTokens * 4
	runes := []rune(text)
	if windowRunes < 1 {
		windowRunes = 1
	}
	var chunks []Chunk
	seq := 1
	for start := 0; start < len(runes); {
		end := start + windowRunes
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, Chunk{
			ID:      chapterPath + "/" + pad2(seq),
			Title:   chapterTitle,
			Content: strings.TrimSpace(string(runes[start:end])),
		})
		start = end
		seq++
	}
	return chunks
}

func pad2(n int) string {
	s := itoa(n)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func normalizeEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func stripYAMLFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return s
	}
	rest := s[4:]
	if idx := strings.Index(rest, "\n---\n"); idx >= 0 {
		return rest[idx+5:]
	}
	if idx := strings.Index(rest, "\n---"); idx >= 0 {
		tail := rest[idx+4:]
		if tail == "" || strings.HasPrefix(tail, "\n") {
			return strings.TrimPrefix(tail, "\n")
		}
	}
	return s
}

func collectHeadingSpans(md string) []headingSpan {
	forbidden := fenceRanges(md)
	var gold []headingSpan
	offset := 0
	for _, line := range strings.Split(md, "\n") {
		lineStart := offset
		lineEnd := offset + len(line)
		offset = lineEnd + 1
		if overlapsAny(lineStart, lineEnd, forbidden) {
			continue
		}
		if m := atxHeadingRe.FindStringSubmatch(line); m != nil {
			gold = append(gold, headingSpan{
				start: lineStart,
				end:   lineEnd,
				level: len(m[1]),
				title: strings.TrimSpace(m[2]),
			})
		}
	}
	goldSet := make([][2]int, len(gold))
	for i, g := range gold {
		goldSet[i] = [2]int{g.start, g.end}
	}
	var chinese []headingSpan
	offset = 0
	for _, line := range strings.Split(md, "\n") {
		lineStart := offset
		lineEnd := offset + len(line)
		offset = lineEnd + 1
		if overlapsAny(lineStart, lineEnd, forbidden) || overlapsAny(lineStart, lineEnd, goldSet) {
			continue
		}
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "|") {
			continue
		}
		if m := chineseL2Re.FindStringSubmatch(trim); m != nil {
			chinese = append(chinese, headingSpan{
				start: lineStart, end: lineEnd, level: 2, title: strings.TrimSpace(m[0]),
			})
			continue
		}
		if m := chineseL3Re.FindStringSubmatch(trim); m != nil {
			chinese = append(chinese, headingSpan{
				start: lineStart, end: lineEnd, level: 3, title: strings.TrimSpace(m[0]),
			})
		}
	}
	spans := append(gold, chinese...)
	// sort by start
	for i := 1; i < len(spans); i++ {
		j := i
		for j > 0 && spans[j].start < spans[j-1].start {
			spans[j], spans[j-1] = spans[j-1], spans[j]
			j--
		}
	}
	return spans
}

func fenceRanges(md string) [][2]int {
	var ranges [][2]int
	offset := 0
	var open byte
	var openLen int
	openStart := -1
	for _, line := range strings.Split(md, "\n") {
		lineStart := offset
		lineEnd := offset + len(line)
		offset = lineEnd + 1
		m := fenceLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mark := m[1]
		ch := mark[0]
		n := len(mark)
		if openStart < 0 {
			open = ch
			openLen = n
			openStart = lineStart
			continue
		}
		if ch == open && n >= openLen {
			ranges = append(ranges, [2]int{openStart, lineEnd})
			openStart = -1
		}
	}
	if openStart >= 0 {
		ranges = append(ranges, [2]int{openStart, len(md)})
	}
	return ranges
}

func overlapsAny(start, end int, ranges [][2]int) bool {
	for _, r := range ranges {
		if start < r[1] && end > r[0] {
			return true
		}
	}
	return false
}

type treeNode struct {
	level     int
	title     string
	localBody string
	children  []*treeNode
}

func buildTree(md string, spans []headingSpan) *treeNode {
	root := &treeNode{level: 0}
	stack := []*treeNode{root}
	prevEnd := 0
	flushBody := func(to int, node *treeNode) {
		if to > prevEnd {
			node.localBody += md[prevEnd:to]
		}
		prevEnd = to
	}
	for _, sp := range spans {
		cur := stack[len(stack)-1]
		flushBody(sp.start, cur)
		for len(stack) > 1 && stack[len(stack)-1].level >= sp.level {
			stack = stack[:len(stack)-1]
		}
		n := &treeNode{level: sp.level, title: sp.title}
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, n)
		stack = append(stack, n)
		// skip heading line; body starts after newline
		prevEnd = sp.end
		if prevEnd < len(md) && md[prevEnd] == '\n' {
			prevEnd++
		}
	}
	flushBody(len(md), stack[len(stack)-1])
	return root
}

type rawChapter struct {
	pathTitles []string
	text       string
}

// extractChapters extracts bottom-up merged logical chapters.
// When a heading's merged text exceeds maxTokens it emits the parent body
// + children as separate chapters. Leaves keep full text (no sliding window).
func extractChapters(node *treeNode, maxTokens int) []rawChapter {
	var childChapters []rawChapter
	for _, ch := range node.children {
		childChapters = append(childChapters, extractChapters(ch, maxTokens)...)
	}

	local := strings.TrimSpace(node.localBody)
	if node.level == 0 {
		if len(childChapters) == 0 && local != "" {
			return []rawChapter{{pathTitles: nil, text: local}}
		}
		return childChapters
	}

	headingLine := strings.Repeat("#", clampLevel(node.level)) + " " + node.title
	merged := headingLine
	if local != "" {
		merged += "\n\n" + local
	}
	for _, c := range childChapters {
		if c.text != "" {
			merged += "\n\n" + c.text
		}
	}
	merged = strings.TrimSpace(merged)

	path := []string{node.title}
	if EstimateTokens(merged) <= maxTokens {
		return []rawChapter{{pathTitles: path, text: merged}}
	}

	var out []rawChapter
	if local != "" {
		leaf := headingLine + "\n\n" + local
		out = append(out, rawChapter{pathTitles: path, text: strings.TrimSpace(leaf)})
	}
	for _, c := range childChapters {
		childPath := append([]string{node.title}, c.pathTitles...)
		out = append(out, rawChapter{pathTitles: childPath, text: c.text})
	}
	if len(out) == 0 {
		return []rawChapter{{pathTitles: path, text: merged}}
	}
	return out
}

// EstimateTokens uses a CJK-aware heuristic (≈1 unit per CJK rune, else 4 runes/unit).
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	cjk := 0
	other := 0
	for _, r := range s {
		if isCJK(r) {
			cjk++
		} else {
			other++
		}
	}
	return cjk + (other+3)/4
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) ||
		(r >= 0x3000 && r <= 0x303F) || // CJK punctuation
		(r >= 0xFF00 && r <= 0xFFEF) // fullwidth
}

func clampLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// CJKBigrams expands CJK runs into space-separated bigrams for FTS unicode61.
func CJKBigrams(s string) string {
	var b strings.Builder
	var run []rune
	flush := func() {
		if len(run) == 0 {
			return
		}
		if len(run) == 1 {
			b.WriteRune(run[0])
			b.WriteByte(' ')
		} else {
			for i := 0; i+1 < len(run); i++ {
				b.WriteRune(run[i])
				b.WriteRune(run[i+1])
				b.WriteByte(' ')
			}
		}
		run = run[:0]
	}
	for _, r := range s {
		if isCJK(r) {
			run = append(run, r)
			continue
		}
		flush()
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if r == ' ' || r == '\n' || r == '\t' {
			b.WriteByte(' ')
		} else {
			b.WriteByte(' ')
		}
	}
	flush()
	return strings.Join(strings.Fields(b.String()), " ")
}

// FTSQuery builds a MATCH query from user text (OR of tokens / CJK bigrams).
func FTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	expanded := CJKBigrams(q)
	parts := strings.Fields(expanded)
	if len(parts) == 0 {
		// fallback: quote whole query
		return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
	}
	for i, p := range parts {
		p = strings.ReplaceAll(p, `"`, `""`)
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, " OR ")
}

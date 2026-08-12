package knowledge

import (
	"math"
	"sort"
	"strings"
	"sync"
)

// BM25 parameters.
const (
	BM25k1 = 1.2
	BM25b  = 0.75
)

// IndexEntry is one chunk-level unit fed into the inverted index.
type IndexEntry struct {
	ID    string
	KBID  string
	DocID string
	Text  string
}

// ChunkHit is a ranked chunk result from BM25 search.
type ChunkHit struct {
	ChunkID string
	Score   float64
}

// InvertedIndex is an in-memory BM25 inverted index over chunk entries.
// Keyed by chunk ID; posting lists store (chunkID, tf) pairs.
type InvertedIndex struct {
	mu       sync.RWMutex
	chunks   map[string]*chunkMeta // chunkID → metadata
	inverted map[string][]posting  // token → posting list
	avgLen   float64
}

type chunkMeta struct {
	KBID   string
	DocID  string
	length int
}

// ChunkMeta is a read-only snapshot of chunk metadata (exported).
type ChunkMeta struct {
	KBID  string
	DocID string
}

func (m *chunkMeta) snapshot() ChunkMeta {
	return ChunkMeta{KBID: m.KBID, DocID: m.DocID}
}

type posting struct {
	chunkID string
	tf      float64
}

// NewInvertedIndex returns a ready-to-use inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		chunks:   make(map[string]*chunkMeta),
		inverted: make(map[string][]posting),
	}
}

// Index replaces the entire chunk index for a doc (delete old + insert new).
func (idx *InvertedIndex) Index(docID string, entries []IndexEntry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.deleteLocked(docID)

	for _, e := range entries {
		tokens := strings.Fields(CJKBigrams(e.Text))
		freq := tokenFreq(tokens)
		length := len(tokens)

		idx.chunks[e.ID] = &chunkMeta{KBID: e.KBID, DocID: e.DocID, length: length}

		for tok := range freq {
			idx.inverted[tok] = append(idx.inverted[tok], posting{chunkID: e.ID, tf: float64(freq[tok])})
		}
	}

	idx.recomputeAvgLenLocked()
}

func (idx *InvertedIndex) recomputeAvgLenLocked() {
	totalLen := 0
	for _, m := range idx.chunks {
		totalLen += m.length
	}
	if len(idx.chunks) > 0 {
		idx.avgLen = float64(totalLen) / float64(len(idx.chunks))
	} else {
		idx.avgLen = 1 // avoid div-by-zero; any positive value works when empty
	}
}

// Delete removes all chunks belonging to a doc or kb.
func (idx *InvertedIndex) DeleteByDoc(docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.deleteLocked(docID)
}

func (idx *InvertedIndex) DeleteByKB(kbID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	removeIDs := make(map[string]bool)
	for cid, meta := range idx.chunks {
		if meta.KBID == kbID {
			removeIDs[cid] = true
			delete(idx.chunks, cid)
		}
	}
	if len(removeIDs) == 0 {
		return
	}
	idx.filterPostingsLocked(removeIDs)
}

func (idx *InvertedIndex) deleteLocked(docID string) {
	removeIDs := make(map[string]bool)
	for cid, meta := range idx.chunks {
		if meta.DocID == docID {
			removeIDs[cid] = true
			delete(idx.chunks, cid)
		}
	}
	if len(removeIDs) == 0 {
		return
	}
	idx.filterPostingsLocked(removeIDs)
}

func (idx *InvertedIndex) filterPostingsLocked(removeIDs map[string]bool) {
	for tok, postings := range idx.inverted {
		filtered := postings[:0]
		for _, p := range postings {
			if !removeIDs[p.chunkID] {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			delete(idx.inverted, tok)
		} else {
			idx.inverted[tok] = filtered
		}
	}
	idx.recomputeAvgLenLocked()
}

// Search performs BM25 retrieval over kbIDs, returning top-limit chunk hits.
func (idx *InvertedIndex) Search(query string, kbIDs []string, limit int) []ChunkHit {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 5
	}

	queryTokens := strings.Fields(CJKBigrams(query))
	if len(queryTokens) == 0 || len(idx.chunks) == 0 {
		return nil
	}

	N := float64(len(idx.chunks))

	// Compute IDF and filter by KB.
	kbSet := make(map[string]bool, len(kbIDs))
	for _, id := range kbIDs {
		kbSet[id] = true
	}

	idf := make(map[string]float64)
	uniqueTokens := make([]string, 0, len(queryTokens))
	seen := make(map[string]bool)
	for _, qt := range queryTokens {
		if seen[qt] {
			continue
		}
		seen[qt] = true
		uniqueTokens = append(uniqueTokens, qt)
		n := float64(len(idx.inverted[qt]))
		if n > 0 {
			idf[qt] = math.Log((N-n+0.5)/(n+0.5) + 1)
		}
	}

	// Score each matching chunk.
	scores := make(map[string]float64)
	for _, qt := range uniqueTokens {
		idfVal := idf[qt]
		if idfVal == 0 {
			continue
		}
		for _, p := range idx.inverted[qt] {
			meta := idx.chunks[p.chunkID]
			if meta == nil || !kbSet[meta.KBID] {
				continue
			}
			tf := p.tf
			numer := idfVal * tf * (BM25k1 + 1)
			denom := tf + BM25k1*(1-BM25b+BM25b*float64(meta.length)/idx.avgLen)
			scores[p.chunkID] += numer / denom
		}
	}

	// Sort by score descending.
	type pair struct {
		chunkID string
		score   float64
	}
	pairs := make([]pair, 0, len(scores))
	for cid, s := range scores {
		pairs = append(pairs, pair{cid, s})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].score > pairs[j].score })

	if len(pairs) > limit {
		pairs = pairs[:limit]
	}

	// Normalize by max score (like tiersum).
	var maxScore float64
	for _, p := range pairs {
		if p.score > maxScore {
			maxScore = p.score
		}
	}

	out := make([]ChunkHit, len(pairs))
	for i, p := range pairs {
		score := p.score
		if maxScore > 0 {
			score = p.score / maxScore
		}
		out[i] = ChunkHit{ChunkID: p.chunkID, Score: score}
	}
	return out
}

// ChunkIDsByKB returns all chunk metadata for the given KBs (used by vector index for filtering).
func (idx *InvertedIndex) ChunkIDsByKB(kbIDs []string) map[string]ChunkMeta {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	kbSet := make(map[string]bool, len(kbIDs))
	for _, id := range kbIDs {
		kbSet[id] = true
	}
	out := make(map[string]ChunkMeta)
	for cid, meta := range idx.chunks {
		if kbSet[meta.KBID] {
			out[cid] = meta.snapshot()
		}
	}
	return out
}

func tokenFreq(tokens []string) map[string]int {
	freq := make(map[string]int, len(tokens))
	for _, t := range tokens {
		freq[t]++
	}
	return freq
}

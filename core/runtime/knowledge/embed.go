package knowledge

import (
	"hash/fnv"
	"math"
)

// SimpleEmbeddingDim is the fixed dimension for the deterministic hash embedder.
const SimpleEmbeddingDim = 64

// SimpleEmbed produces a deterministic unit vector from text (tiersum-style fallback).
// Used when runtime.knowledge.vector_hybrid is enabled without an external model.
func SimpleEmbed(text string) []float32 {
	vec := make([]float32, SimpleEmbeddingDim)
	if text == "" {
		return vec
	}
	// Hash overlapping token windows into buckets.
	expanded := CJKBigrams(text)
	tokens := splitEmbedTokens(expanded)
	if len(tokens) == 0 {
		tokens = []string{text}
	}
	for _, tok := range tokens {
		h := fnv.New32a()
		_, _ = h.Write([]byte(tok))
		v := h.Sum32()
		idx := int(v % uint32(SimpleEmbeddingDim))
		sign := float32(1)
		if v&1 == 1 {
			sign = -1
		}
		vec[idx] += sign
	}
	return l2normalize(vec)
}

// CosineSimilarity returns cosine in [0,1] mapped from [-1,1] via (1+cos)/2.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	cos := dot / (math.Sqrt(na) * math.Sqrt(nb))
	if cos < -1 {
		cos = -1
	}
	if cos > 1 {
		cos = 1
	}
	return (cos + 1) / 2
}

func splitEmbedTokens(s string) []string {
	fields := make([]string, 0, 16)
	start := -1
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}

func l2normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}

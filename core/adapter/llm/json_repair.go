package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// repairJSONObject attempts to fix common LLM tool-argument JSON damage,
// especially unescaped quotes and raw control characters inside string values
// that break object deserialization.
//
// It only returns success when the repaired bytes unmarshal to a JSON object.
func repairJSONObject(raw []byte) ([]byte, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, fmt.Errorf("empty json")
	}

	candidates := []string{s}
	if repaired := repairEscapeDamage(s); repaired != s {
		candidates = append(candidates, repaired)
	}
	// Trailing-comma cleanup may help after escape repair or alone.
	for _, c := range append([]string{}, candidates...) {
		if trimmed := stripTrailingCommas(c); trimmed != c {
			candidates = append(candidates, trimmed)
		}
	}

	var lastErr error
	truncatedOnly := false
	for _, c := range candidates {
		closed := closeOpenJSON(c)
		for _, attempt := range []string{c, closed} {
			if !json.Valid([]byte(attempt)) {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(attempt), &obj); err != nil {
				lastErr = err
				continue
			}
			if obj == nil {
				lastErr = fmt.Errorf("arguments parsed to nil")
				continue
			}
			// Parsing only succeeded after force-closing open strings/brackets:
			// the payload was truncated mid-generation. Executing a tool with
			// silently amputated arguments (e.g. half a file for `write`) is
			// worse than failing — reject so the model retries.
			if attempt == closed && closed != c {
				truncatedOnly = true
				lastErr = fmt.Errorf("tool arguments appear truncated (unbalanced JSON); refusing to execute with incomplete input")
				continue
			}
			out, err := json.Marshal(obj)
			if err != nil {
				lastErr = err
				continue
			}
			return out, nil
		}
	}
	if truncatedOnly || lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("unable to repair json object")
}

// repairEscapeDamage walks the input and escapes quotes / control chars that
// appear inside string values but are not valid JSON escapes. A quote is treated
// as structural (end of string) only when what follows looks like JSON syntax
// (comma, colon, closing brace/bracket, or EOF after whitespace).
func repairEscapeDamage(s string) string {
	var out strings.Builder
	out.Grow(len(s) + 16)

	inString := false
	escaped := false

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8 byte: keep as-is outside strings; inside, replace.
			if inString {
				out.WriteRune(unicode.ReplacementChar)
			} else {
				out.WriteByte(s[i])
			}
			i++
			continue
		}

		if !inString {
			out.WriteString(s[i : i+size])
			if r == '"' {
				inString = true
				escaped = false
			}
			i += size
			continue
		}

		// Inside a JSON string.
		if escaped {
			out.WriteString(s[i : i+size])
			escaped = false
			i += size
			continue
		}

		if r == '\\' {
			out.WriteByte('\\')
			escaped = true
			i += size
			continue
		}

		if r == '"' {
			if isLikelyStringEnd(s, i+size) {
				out.WriteByte('"')
				inString = false
			} else {
				out.WriteString(`\"`)
			}
			i += size
			continue
		}

		switch r {
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				out.WriteString(s[i : i+size])
			}
		}
		i += size
	}

	return out.String()
}

// isLikelyStringEnd reports whether a quote at the current position is more
// likely a closing quote than content. After optional whitespace, a structural
// JSON token or EOF must follow.
func isLikelyStringEnd(s string, next int) bool {
	j := next
	for j < len(s) {
		r, size := utf8.DecodeRuneInString(s[j:])
		if unicode.IsSpace(r) {
			j += size
			continue
		}
		switch r {
		case ',', '}', ']', ':':
			return true
		default:
			return false
		}
	}
	return true
}

// stripTrailingCommas removes commas that appear immediately before } or ].
func stripTrailingCommas(s string) string {
	var out bytes.Buffer
	out.Grow(len(s))
	inString := false
	escaped := false
outer:
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		ch := s[i : i+size]

		if inString {
			out.WriteString(ch)
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			i += size
			continue
		}

		if r == '"' {
			inString = true
			out.WriteString(ch)
			i += size
			continue
		}

		if r == ',' {
			k := i + size
			for k < len(s) {
				rr, sz := utf8.DecodeRuneInString(s[k:])
				if unicode.IsSpace(rr) {
					k += sz
					continue
				}
				if rr == '}' || rr == ']' {
					// Drop the trailing comma; keep }/] for later writes.
					i += size
					continue outer
				}
				break
			}
		}

		out.WriteString(ch)
		i += size
	}
	return out.String()
}

// closeOpenJSON appends missing quotes / brackets / braces for truncated output.
func closeOpenJSON(s string) string {
	inString := false
	escaped := false
	stack := make([]byte, 0, 8)

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if inString {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			i += size
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == byte(r) {
				stack = stack[:len(stack)-1]
			}
		}
		i += size
	}

	var b strings.Builder
	b.WriteString(s)
	if inString {
		b.WriteByte('"')
	}
	for i := len(stack) - 1; i >= 0; i-- {
		b.WriteByte(stack[i])
	}
	return b.String()
}

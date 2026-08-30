package builtin

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// TextEncoding names the character encoding of a text file.
type TextEncoding string

const (
	EncUTF8    TextEncoding = "utf-8"
	EncUTF8BOM TextEncoding = "utf-8-bom"
	EncUTF16LE TextEncoding = "utf-16le"
	EncUTF16BE TextEncoding = "utf-16be"
	EncGB18030 TextEncoding = "gb18030" // GB18030 is a superset of GBK
)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// textFileMeta records how a file was decoded. Writes always persist as
// UTF-8 (see writeEncodingMeta); line endings may still be preserved.
type textFileMeta struct {
	Encoding   TextEncoding
	LineEnding string // "\n", "\r\n", or "\r"
}

// decodeTextFile decodes raw file bytes into normalized UTF-8 text ("\n"
// line endings) for tool operations. Binary files (undecodable or
// NUL-heavy non-UTF-16 data) return an error so editors never corrupt them.
func decodeTextFile(data []byte) (string, textFileMeta, error) {
	meta := textFileMeta{Encoding: EncUTF8, LineEnding: "\n"}

	if len(data) >= 2 {
		switch {
		case bytes.HasPrefix(data, bomUTF16LE):
			return decodeUTF16(data[2:], true, meta)
		case bytes.HasPrefix(data, bomUTF16BE):
			return decodeUTF16(data[2:], false, meta)
		}
	}
	if bytes.HasPrefix(data, bomUTF8) {
		meta.Encoding = EncUTF8BOM
		data = data[len(bomUTF8):]
	}

	if utf8.Valid(data) {
		return normalizeLineEndings(string(data), &meta), meta, nil
	}

	// Not valid UTF-8: assume a legacy Chinese encoding (GB18030 covers GBK).
	decoded, err := decodeGB18030(data)
	if err != nil {
		return "", meta, fmt.Errorf("not a text file: invalid UTF-8 and GB18030 decode failed (binary or unknown encoding)")
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", meta, fmt.Errorf("not a text file: NUL bytes present (binary)")
	}
	meta.Encoding = EncGB18030
	return normalizeLineEndings(decoded, &meta), meta, nil
}

func decodeUTF16(data []byte, littleEndian bool, meta textFileMeta) (string, textFileMeta, error) {
	if len(data)%2 != 0 {
		return "", meta, fmt.Errorf("not a text file: odd-length UTF-16 data")
	}
	u := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if littleEndian {
			u = append(u, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			u = append(u, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	if littleEndian {
		meta.Encoding = EncUTF16LE
	} else {
		meta.Encoding = EncUTF16BE
	}
	return normalizeLineEndings(string(utf16.Decode(u)), &meta), meta, nil
}

func decodeGB18030(data []byte) (string, error) {
	decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), data)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(decoded) {
		return "", errors.New("invalid decoded bytes")
	}
	return string(decoded), nil
}

// normalizeLineEndings converts CRLF/CR to LF for matching and records the
// dominant original line ending.
func normalizeLineEndings(s string, meta *textFileMeta) string {
	crlf := strings.Count(s, "\r\n")
	loneCR := strings.Count(s, "\r") - crlf
	lf := strings.Count(s, "\n") - crlf

	if crlf >= lf && crlf >= loneCR && crlf > 0 {
		meta.LineEnding = "\r\n"
	} else if loneCR > lf && loneCR > crlf {
		meta.LineEnding = "\r"
	}
	// Default stays "\n" when it dominates.

	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// encodeTextFile converts normalized UTF-8 text back into the encoding,
// BOM, and line endings recorded in meta. Agent write/edit/patch paths
// should call writeEncodingMeta first so on-disk files become UTF-8.
func encodeTextFile(text string, meta textFileMeta) []byte {
	if meta.LineEnding != "\n" {
		text = strings.ReplaceAll(text, "\n", meta.LineEnding)
	}
	switch meta.Encoding {
	case EncUTF8BOM:
		return append(append([]byte{}, bomUTF8...), []byte(text)...)
	case EncUTF16LE:
		return encodeUTF16(text, true)
	case EncUTF16BE:
		return encodeUTF16(text, false)
	case EncGB18030:
		if out, err := encodeGB18030(text); err == nil {
			return out
		}
		// GB18030 covers all of Unicode, so the encoder only fails on
		// unexpected input; degrade to UTF-8 rather than corrupting the file.
	}
	return []byte(text)
}

// writeEncodingMeta returns the meta used when agent tools persist text.
// Always UTF-8 (no BOM). Keeps the detected line ending so CRLF files do
// not churn endings on every edit.
func writeEncodingMeta(detected textFileMeta) textFileMeta {
	le := detected.LineEnding
	if le == "" {
		le = "\n"
	}
	return textFileMeta{Encoding: EncUTF8, LineEnding: le}
}

// encodingNote returns a short human note appended to tool output when the
// file uses a non-plain-UTF-8 encoding (and was preserved).
func encodingNote(meta textFileMeta) string {
	if meta.Encoding == EncUTF8 && meta.LineEnding == "\n" {
		return ""
	}
	return fmt.Sprintf(" (encoding: %s, line ending: %s)", meta.Encoding, meta.LineEnding)
}

// conversionNote explains UTF-8 normalization when the on-disk encoding changed.
func conversionNote(before, after textFileMeta) string {
	if before.Encoding == after.Encoding {
		return encodingNote(after)
	}
	le := after.LineEnding
	if le == "" {
		le = "\n"
	}
	return fmt.Sprintf(" (converted %s → utf-8, line ending: %q)", before.Encoding, le)
}

func encodeUTF16(text string, littleEndian bool) []byte {
	u := utf16.Encode([]rune(text))
	var b bytes.Buffer
	if littleEndian {
		b.Write(bomUTF16LE)
		for _, c := range u {
			b.WriteByte(byte(c))
			b.WriteByte(byte(c >> 8))
		}
	} else {
		b.Write(bomUTF16BE)
		for _, c := range u {
			b.WriteByte(byte(c >> 8))
			b.WriteByte(byte(c))
		}
	}
	return b.Bytes()
}

func encodeGB18030(text string) ([]byte, error) {
	out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(text))
	return out, err
}

// writeFilePreserving writes data while preserving the existing file's
// permission bits (defaults to 0644 for new files). Without this, editing a
// script would silently strip its executable bit.
func writeFilePreserving(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}

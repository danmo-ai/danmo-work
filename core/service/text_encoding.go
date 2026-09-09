package service

import (
	"bytes"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// decodeProjectTextBytes decodes project file bytes for API/UI display.
// Mirrors agent tool decode: UTF-8 / BOM / UTF-16 / GB18030 → UTF-8 string.
// Does not rewrite the file; WriteFileContent still persists UTF-8.
func decodeProjectTextBytes(data []byte) (string, error) {
	if len(data) >= 2 {
		switch {
		case bytes.HasPrefix(data, bomUTF16LE):
			return decodeUTF16Project(data[2:], true)
		case bytes.HasPrefix(data, bomUTF16BE):
			return decodeUTF16Project(data[2:], false)
		}
	}
	if bytes.HasPrefix(data, bomUTF8) {
		data = data[len(bomUTF8):]
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), data)
	if err != nil {
		return "", fmt.Errorf("invalid UTF-8 and GB18030 decode failed")
	}
	if !utf8.Valid(decoded) {
		return "", fmt.Errorf("invalid decoded bytes")
	}
	return string(decoded), nil
}

func decodeUTF16Project(data []byte, littleEndian bool) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("odd-length UTF-16 data")
	}
	u := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if littleEndian {
			u = append(u, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			u = append(u, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	return string(utf16.Decode(u)), nil
}

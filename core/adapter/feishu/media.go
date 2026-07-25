package feishu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/paths"
)

func saveFeishuMedia(accountID, messageID, name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty media")
	}
	accountID = sanitizePart(accountID)
	if accountID == "" {
		accountID = "default"
	}
	name = sanitizeName(name)
	if name == "" {
		name = "file.bin"
	}
	day := time.Now().UTC().Format("2006/01/02")
	dir := filepath.Join(paths.DataDir(), "channels", "feishu", accountID, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	prefix := sanitizePart(messageID)
	if prefix == "" {
		prefix = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	path := filepath.Join(dir, prefix+"_"+name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizePart(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func sanitizeName(s string) string {
	s = filepath.Base(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "..", "_")
	return sanitizePart(s)
}

package service

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"
)

// ChannelMediaRoot returns ~/.danmo-work/data/channels (or WORK_DATA_DIR/channels).
func ChannelMediaRoot() string {
	return filepath.Join(paths.DataDir(), "channels")
}

// SaveChannelMedia writes bytes under channels/{type}/{account}/{date}/{filename}.
func SaveChannelMedia(channel port.ChannelType, accountID, messageID, name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty media")
	}
	accountID = sanitizePathPart(accountID)
	if accountID == "" {
		accountID = "default"
	}
	name = sanitizeFilename(name)
	if name == "" {
		name = "file.bin"
	}
	day := time.Now().UTC().Format("2006/01/02")
	dir := filepath.Join(ChannelMediaRoot(), string(channel), accountID, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	prefix := sanitizePathPart(messageID)
	if prefix == "" {
		prefix = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	path := filepath.Join(dir, prefix+"_"+name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// DownloadURL fetches a remote media URL into channel media storage.
func DownloadURL(client *http.Client, channel port.ChannelType, accountID, messageID, name, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty url")
	}
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 32 MiB cap
	if err != nil {
		return "", err
	}
	if name == "" {
		name = filepath.Base(strings.Split(rawURL, "?")[0])
	}
	return SaveChannelMedia(channel, accountID, messageID, name, data)
}

// FormatMediaUserText appends local path lines for agent context.
func FormatMediaUserText(text string, media []port.InboundMedia) string {
	text = strings.TrimSpace(text)
	if len(media) == 0 {
		return text
	}
	var b strings.Builder
	if text != "" {
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	for _, m := range media {
		kind := m.Kind
		if kind == "" {
			kind = "file"
		}
		label := m.Name
		if label == "" {
			label = filepath.Base(m.Path)
		}
		b.WriteString(fmt.Sprintf("[%s saved: %s]", kind, m.Path))
		if label != "" && label != filepath.Base(m.Path) {
			b.WriteString(" (" + label + ")")
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// MediaToVisionAttachments converts local image files to UserAttachment (base64) when small enough.
func MediaToVisionAttachments(media []port.InboundMedia) []domain.UserAttachment {
	var out []domain.UserAttachment
	for _, m := range media {
		if m.Kind != "image" || m.Path == "" {
			continue
		}
		data, err := os.ReadFile(m.Path)
		if err != nil || len(data) == 0 || len(data) > 10<<20 {
			continue
		}
		mime := m.MimeType
		if mime == "" {
			mime = guessImageMIME(m.Name, m.Path)
		}
		out = append(out, domain.UserAttachment{
			Type:     "image",
			Name:     m.Name,
			MimeType: mime,
			Data:     base64.StdEncoding.EncodeToString(data),
		})
	}
	return out
}

func sanitizePathPart(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, s)
	return s
}

func sanitizeFilename(s string) string {
	s = filepath.Base(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "..", "_")
	return sanitizePathPart(s)
}

func guessImageMIME(name, path string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(path))
	}
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

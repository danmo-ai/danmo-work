package qq

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/paths"
	"danmo-work/core/port"
)

func downloadAndSave(client *http.Client, accountID, messageID, name, rawURL string) (string, error) {
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
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", err
	}
	return saveQQMedia(accountID, messageID, name, data)
}

func saveQQMedia(accountID, messageID, name string, data []byte) (string, error) {
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
	dir := filepath.Join(paths.DataDir(), "channels", "qq", accountID, day)
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

// EnrichInboundMedia downloads remote attachment URLs into local channel media paths.
func (a *Adapter) EnrichInboundMedia(msg *port.InboundMessage) {
	if a == nil || msg == nil || len(msg.Media) == 0 {
		return
	}
	for i := range msg.Media {
		m := &msg.Media[i]
		if m.Path != "" || m.URL == "" {
			continue
		}
		path, err := downloadAndSave(a.client, msg.AccountID, msg.MessageID, m.Name, m.URL)
		if err != nil {
			continue
		}
		m.Path = path
	}
}

// SendLocalFile sends a local file reference as markdown (reliable across tenants).
// Full binary upload APIs vary by QQ scene and are left for a later iteration.
func (a *Adapter) SendLocalFile(ctx context.Context, in *port.InboundMessage, path, filename string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("qq send file: path required")
	}
	if filename == "" {
		filename = filepath.Base(path)
	}
	// Phase C: text path notice is reliable across tenants; full file upload APIs vary by scene.
	body := fmt.Sprintf("文件：`%s`\n本地路径：`%s`", filename, path)
	return a.DeliverOutbound(ctx, in, port.OutboundMessage{
		Kind:  port.OutboundKindMarkdown,
		Title: "文件",
		Text:  body,
	})
}

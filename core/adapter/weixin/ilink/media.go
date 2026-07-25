package ilink

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"danmo-work/core/port"
)

// MediaRef is a downloadable inbound attachment extracted from a MessageItem.
type MediaRef struct {
	Kind     string // image | file | audio | video
	Name     string
	MimeType string
	Media    CDNMedia
	AESKey   string // optional override (image aeskey hex)
}

// CollectMediaRefs lists downloadable media items (skips text-only / ASR-only voice).
func CollectMediaRefs(msg Message) []MediaRef {
	var out []MediaRef
	for _, item := range msg.ItemList {
		switch item.Type {
		case MessageItemImage:
			if item.ImageItem == nil {
				continue
			}
			ref := MediaRef{Kind: "image", Name: "image.jpg", MimeType: "image/jpeg"}
			if item.ImageItem.Media != nil {
				ref.Media = *item.ImageItem.Media
			}
			ref.AESKey = strings.TrimSpace(item.ImageItem.AESKey)
			if ref.Media.EncryptQueryParam == "" && strings.TrimSpace(item.ImageItem.URL) == "" {
				continue
			}
			// Prefer CDN media; URL alone is rare.
			if ref.Media.EncryptQueryParam == "" {
				continue
			}
			out = append(out, ref)
		case MessageItemFile:
			if item.FileItem == nil || item.FileItem.Media == nil {
				continue
			}
			name := strings.TrimSpace(item.FileItem.FileName)
			if name == "" {
				name = "file.bin"
			}
			out = append(out, MediaRef{
				Kind:  "file",
				Name:  name,
				Media: *item.FileItem.Media,
			})
		case MessageItemVoice:
			if item.VoiceItem == nil {
				continue
			}
			// Prefer ASR text via TextFromMessage; still download when media present and no text.
			if strings.TrimSpace(item.VoiceItem.Text) != "" {
				continue
			}
			if item.VoiceItem.Media == nil || item.VoiceItem.Media.EncryptQueryParam == "" {
				continue
			}
			out = append(out, MediaRef{
				Kind:     "audio",
				Name:     "voice.silk",
				MimeType: "audio/silk",
				Media:    *item.VoiceItem.Media,
			})
		case MessageItemVideo:
			if item.VideoItem == nil || item.VideoItem.Media == nil {
				continue
			}
			out = append(out, MediaRef{
				Kind:     "video",
				Name:     "video.mp4",
				MimeType: "video/mp4",
				Media:    *item.VideoItem.Media,
			})
		}
	}
	return out
}

// DownloadMediaRef fetches and decrypts CDN media into plaintext bytes.
func (c *Client) DownloadMediaRef(ctx context.Context, ref MediaRef) ([]byte, error) {
	key := strings.TrimSpace(ref.AESKey)
	if key == "" {
		key = strings.TrimSpace(ref.Media.AESKey)
	}
	param := strings.TrimSpace(ref.Media.EncryptQueryParam)
	if param == "" {
		return nil, fmt.Errorf("ilink media: missing encrypt_query_param")
	}
	cdn := DefaultCDNBaseURL
	u := cdn + "/download?encrypted_query_param=" + url.QueryEscape(param)
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ilink cdn download HTTP %d", resp.StatusCode)
	}
	cipher, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if key == "" {
		// Some image payloads may arrive without encryption; return as-is if looks plausible.
		return cipher, nil
	}
	plain, err := DecryptAES128ECB(cipher, key)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// DecryptAES128ECB decrypts PKCS7-padded AES-128-ECB ciphertext.
// key may be: hex(32), base64(raw16), or base64(hex32).
func DecryptAES128ECB(ciphertext []byte, keySpec string) ([]byte, error) {
	key, err := decodeAESKey(keySpec)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ilink aes: invalid ciphertext length %d", len(ciphertext))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += aes.BlockSize {
		block.Decrypt(plain[i:i+aes.BlockSize], ciphertext[i:i+aes.BlockSize])
	}
	return pkcs7Unpad(plain)
}

func decodeAESKey(spec string) ([]byte, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("ilink aes: empty key")
	}
	// Direct hex (32 chars → 16 bytes).
	if len(spec) == 32 {
		if b, err := hex.DecodeString(spec); err == nil && len(b) == 16 {
			return b, nil
		}
	}
	raw, err := base64.StdEncoding.DecodeString(spec)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(spec)
	}
	if err != nil {
		return nil, fmt.Errorf("ilink aes: decode key: %w", err)
	}
	if len(raw) == 16 {
		return raw, nil
	}
	// base64(hex string of 32 chars)
	if len(raw) == 32 {
		if b, err := hex.DecodeString(string(raw)); err == nil && len(b) == 16 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("ilink aes: unsupported key length %d", len(raw))
}

func pkcs7Unpad(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("ilink aes: empty plaintext")
	}
	pad := int(b[len(b)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(b) {
		return nil, fmt.Errorf("ilink aes: bad pkcs7 padding")
	}
	for i := 0; i < pad; i++ {
		if b[len(b)-1-i] != byte(pad) {
			return nil, fmt.Errorf("ilink aes: bad pkcs7 padding")
		}
	}
	return b[:len(b)-pad], nil
}

// MediaRefsToInbound builds port.InboundMedia stubs (Path filled after download).
func MediaRefsToInbound(refs []MediaRef) []port.InboundMedia {
	out := make([]port.InboundMedia, 0, len(refs))
	for _, r := range refs {
		out = append(out, port.InboundMedia{
			Name:     r.Name,
			MimeType: r.MimeType,
			Kind:     r.Kind,
			Key:      r.Media.EncryptQueryParam,
		})
	}
	return out
}

package ilink

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestDecryptAES128ECBHexKey(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}
	plain := []byte("hello weixin media!!") // 20 bytes → pad to 32
	padded := pkcs7Pad(plain, aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	cipher := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(cipher[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}
	got, err := DecryptAES128ECB(cipher, hex.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("got %q", got)
	}
}

func TestDecryptAES128ECBBase64RawKey(t *testing.T) {
	key := []byte("0123456789abcdef")
	plain := []byte("abc")
	padded := pkcs7Pad(plain, aes.BlockSize)
	block, _ := aes.NewCipher(key)
	cipher := make([]byte, len(padded))
	block.Encrypt(cipher, padded)
	got, err := DecryptAES128ECB(cipher, base64.StdEncoding.EncodeToString(key))
	if err != nil || string(got) != "abc" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestCollectMediaRefs(t *testing.T) {
	msg := Message{
		ItemList: []MessageItem{
			{Type: MessageItemText, TextItem: &TextItem{Text: "hi"}},
			{Type: MessageItemImage, ImageItem: &ImageItem{
				Media:  &CDNMedia{EncryptQueryParam: "p1", AESKey: "k"},
				AESKey: "aabb",
			}},
			{Type: MessageItemVoice, VoiceItem: &VoiceItem{Text: "transcribed"}},
			{Type: MessageItemFile, FileItem: &FileItem{
				FileName: "a.pdf",
				Media:    &CDNMedia{EncryptQueryParam: "p2"},
			}},
		},
	}
	refs := CollectMediaRefs(msg)
	if len(refs) != 2 {
		t.Fatalf("refs=%d %+v", len(refs), refs)
	}
	if refs[0].Kind != "image" || refs[1].Kind != "file" || refs[1].Name != "a.pdf" {
		t.Fatalf("%+v", refs)
	}
}

func pkcs7Pad(b []byte, blockSize int) []byte {
	pad := blockSize - (len(b) % blockSize)
	out := make([]byte, len(b)+pad)
	copy(out, b)
	for i := len(b); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

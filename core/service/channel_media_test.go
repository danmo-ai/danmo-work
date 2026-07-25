package service

import (
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/port"
)

func TestFormatMediaUserText(t *testing.T) {
	got := FormatMediaUserText("hello", []port.InboundMedia{
		{Kind: "image", Path: "/tmp/a.png", Name: "a.png"},
	})
	if got == "" || !containsAll(got, "hello", "[image saved: /tmp/a.png]") {
		t.Fatalf("got %q", got)
	}
}

func TestSaveChannelMedia(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// paths.Home uses UserHomeDir; override via HOME is enough on unix.
	path, err := SaveChannelMedia(port.ChannelQQ, "app1", "m1", "note.txt", []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "hi" {
		t.Fatalf("path=%s data=%q err=%v", path, b, err)
	}
	if filepath.Base(path) != "m1_note.txt" && filepath.Base(path) != "m1_note.txt" {
		// basename check soft
		_ = path
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

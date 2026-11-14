//go:build windows

package computer

import (
	"strings"
	"testing"
)

func TestWindowsUnicodeChunksPreserveSurrogatePairs(t *testing.T) {
	chunks := splitUTF16Chunks(strings.Repeat("a😀中", 100), 32)
	if len(chunks) < 2 {
		t.Fatal("expected chunks")
	}
	if strings.Join(chunks, "") != strings.Repeat("a😀中", 100) {
		t.Fatal("text was changed by chunking")
	}
	for _, chunk := range chunks {
		if strings.HasSuffix(chunk, "\xed\xa0") {
			t.Fatal("split surrogate pair")
		}
	}
}

func TestWindowsVirtualKeys(t *testing.T) {
	keys, err := windowsVirtualKeys("ctrl+shift+s")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0] != 0x11 || keys[1] != 0x10 || keys[2] != 'S' {
		t.Fatalf("keys=%v", keys)
	}
	keys, err = windowsVirtualKeys("shift++")
	if err != nil || len(keys) != 2 || keys[1] != 0xBB {
		t.Fatalf("plus keys=%v err=%v", keys, err)
	}
}

func TestWindowsChordUsesNativeKeyEvents(t *testing.T) {
	if got := windowsNativeChord("ctrl+s"); got != "s\tctrl" {
		t.Fatalf("chord=%q", got)
	}
	if got := sendKeysChord("ctrl+F12"); got != "^"+"{F12}" {
		t.Fatalf("sendkeys=%q", got)
	}
}

package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func gbkBytes(s string) []byte {
	out, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(s))
	if err != nil {
		panic(err)
	}
	return out
}

func TestDecodeTextFileUTF8(t *testing.T) {
	text, meta, err := decodeTextFile([]byte("hello\nworld\n"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello\nworld\n" || meta.Encoding != EncUTF8 || meta.LineEnding != "\n" {
		t.Fatalf("unexpected: %q %+v", text, meta)
	}
}

func TestDecodeTextFileGBK(t *testing.T) {
	text, meta, err := decodeTextFile(gbkBytes("中文内容\r\n第二行\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "中文内容\n第二行\n" {
		t.Fatalf("unexpected text: %q", text)
	}
	if meta.Encoding != EncGB18030 || meta.LineEnding != "\r\n" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
}

func TestDecodeTextFileCRLFOnly(t *testing.T) {
	text, meta, err := decodeTextFile([]byte("a\r\nb\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "a\nb\n" || meta.Encoding != EncUTF8 || meta.LineEnding != "\r\n" {
		t.Fatalf("unexpected: %q %+v", text, meta)
	}
}

func TestDecodeTextFileUTF8BOM(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("a\n")...)
	text, meta, err := decodeTextFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if text != "a\n" || meta.Encoding != EncUTF8BOM {
		t.Fatalf("unexpected: %q %+v", text, meta)
	}
}

func TestDecodeTextFileBinary(t *testing.T) {
	if _, _, err := decodeTextFile([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE}); err == nil {
		t.Fatal("expected error for binary data")
	}
}

func TestEncodeTextFileGBKRoundTrip(t *testing.T) {
	orig := gbkBytes("第一行\r\n第二行\r\n")
	text, meta, err := decodeTextFile(orig)
	if err != nil {
		t.Fatal(err)
	}
	back := encodeTextFile(text, meta)
	if string(back) != string(orig) {
		t.Fatalf("round trip mismatch:\norig: %v\nback: %v", orig, back)
	}
}

func TestEncodeTextFileUTF16LERoundTrip(t *testing.T) {
	data := append(append([]byte{}, bomUTF16LE...), utf16LEBytes("你好\n世界\n")...)
	text, meta, err := decodeTextFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Encoding != EncUTF16LE {
		t.Fatalf("unexpected encoding: %s", meta.Encoding)
	}
	back := encodeTextFile(text, meta)
	if string(back) != string(data) {
		t.Fatalf("round trip mismatch:\norig: %v\nback: %v", data, back)
	}
}

func utf16LEBytes(s string) []byte {
	var b []byte
	for _, r := range []rune(s) {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func TestWriteFilePreservingKeepsExecutableBit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFilePreserving(p, []byte("#!/bin/sh\necho hi\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("permissions lost: %v", info.Mode().Perm())
	}
}

func TestEncodingNoteEmptyForPlainUTF8(t *testing.T) {
	if n := encodingNote(textFileMeta{Encoding: EncUTF8, LineEnding: "\n"}); n != "" {
		t.Fatalf("expected empty note, got %q", n)
	}
	if n := encodingNote(textFileMeta{Encoding: EncGB18030, LineEnding: "\r\n"}); !strings.Contains(n, "gb18030") {
		t.Fatalf("expected gb18030 note, got %q", n)
	}
}

func TestEditConvertsGBKToUTF8KeepsCRLF(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "notes.txt")
	orig := gbkBytes("第一行\r\n第二行\r\n第三行\r\n")
	if err := os.WriteFile(f, orig, 0o644); err != nil {
		t.Fatal(err)
	}

	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Edit{}
	result, err := h.Execute(nil, map[string]any{
		"path":           f,
		"oldString":      "第二行",
		"newString":      "改过的行",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "converted gb18030 → utf-8") {
		t.Fatalf("expected conversion note, got: %q", result.Content)
	}

	want := []byte("第一行\r\n改过的行\r\n第三行\r\n") // UTF-8 + CRLF
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("expected UTF-8 with CRLF preserved:\nwant: %v\ngot:  %v", want, got)
	}
}

func TestWriteConvertsGBKToUTF8(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(f, gbkBytes("旧内容\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ft := setupTracker(dir)
	ft.NoteRead(f)

	h := &Write{}
	result, err := h.Execute(nil, map[string]any{
		"path":           "notes.txt",
		"content":        "新内容\n",
		"__work_dir":     dir,
		"__file_tracker": ft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "converted gb18030 → utf-8") {
		t.Fatalf("expected conversion note, got: %q", result.Content)
	}
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "新内容\n" {
		t.Fatalf("expected UTF-8 write, got %v", got)
	}
}

func TestReadFileDecodesGBK(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(f, gbkBytes("中文标题\r\n内容\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadFile{}
	result, err := h.Execute(nil, map[string]any{
		"path":       "readme.txt",
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "中文标题") {
		t.Fatalf("expected decoded content, got: %q", result.Content)
	}
}

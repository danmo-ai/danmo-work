package builtin

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestPNG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadImageReturnsPart(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(f, makeTestPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadImage{}
	result, err := h.Execute(nil, map[string]any{
		"path":       "shot.png",
		"__work_dir": dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Parts) != 1 || result.Parts[0].Type != "image" {
		t.Fatalf("expected one image part, got %+v", result.Parts)
	}
	if result.Parts[0].MimeType != "image/png" {
		t.Fatalf("unexpected mime: %s", result.Parts[0].MimeType)
	}
	raw, err := base64.StdEncoding.DecodeString(result.Parts[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty image payload")
	}
	if !strings.Contains(result.Content, "4x3") {
		t.Fatalf("expected dimensions in content, got: %s", result.Content)
	}
}

func TestReadImageVisionGate(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(f, makeTestPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadImage{SupportsImage: func(modelID string) bool { return false }}
	_, err := h.Execute(nil, map[string]any{
		"path":       "shot.png",
		"__work_dir": dir,
		"__model_id": "acme/text-only",
	})
	if err == nil {
		t.Fatal("expected vision-gate rejection")
	}

	permissive := &ReadImage{SupportsImage: func(modelID string) bool { return true }}
	if _, err := permissive.Execute(nil, map[string]any{
		"path":       "shot.png",
		"__work_dir": dir,
		"__model_id": "acme/vision-model",
	}); err != nil {
		t.Fatalf("vision-capable model should pass: %v", err)
	}
}

func TestReadImageRejectsTextFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(f, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &ReadImage{}
	if _, err := h.Execute(nil, map[string]any{
		"path":       "note.txt",
		"__work_dir": dir,
	}); err == nil {
		t.Fatal("expected unsupported-format error")
	}
}

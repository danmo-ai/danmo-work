package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
)

func TestUploadFileWritesUnderUploads(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	pm := service.NewProjectManager(st, dir)
	ctx := context.Background()

	work := filepath.Join(dir, "repo")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	proj, err := pm.Create(ctx, domain.CreateProjectRequest{Name: "upload-test", Directory: work})
	if err != nil {
		t.Fatal(err)
	}

	path, err := pm.UploadFile(ctx, proj.ID, "../../evil.pdf", []byte("%PDF-1.4 hello"))
	if err != nil {
		t.Fatal(err)
	}
	if path != "uploads/evil.pdf" {
		t.Fatalf("path = %q, want uploads/evil.pdf", path)
	}
	raw, err := os.ReadFile(filepath.Join(work, "uploads", "evil.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "%PDF-1.4 hello" {
		t.Fatalf("content = %q", raw)
	}

	path2, err := pm.UploadFile(ctx, proj.ID, "evil.pdf", []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if path2 != "uploads/evil_1.pdf" {
		t.Fatalf("path2 = %q, want uploads/evil_1.pdf", path2)
	}
}

func TestSanitizeUploadFilename(t *testing.T) {
	if got := service.SanitizeUploadFilename("../a/b.txt"); got != "b.txt" {
		t.Fatalf("got %q", got)
	}
	if got := service.SanitizeUploadFilename(""); got != "file" {
		t.Fatalf("got %q", got)
	}
	if got := service.SanitizeUploadFilename(".."); got != "file" {
		t.Fatalf("got %q", got)
	}
}

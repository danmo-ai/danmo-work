package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
)

func TestNewRejectsReadOnlyDirectory(t *testing.T) {
	root := t.TempDir()
	// Seed a normal DB, then reopen a copy inside a chmod 0555 directory.
	src := filepath.Join(root, "seed.db")
	st, err := New(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	roDir := filepath.Join(root, "ro")
	if err := os.MkdirAll(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	roDB := filepath.Join(roDir, "work.db")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roDB, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// Also copy WAL/SHM if present so reopen is consistent.
	for _, suf := range []string{"-wal", "-shm"} {
		if b, err := os.ReadFile(src + suf); err == nil {
			_ = os.WriteFile(roDB+suf, b, 0o644)
		}
	}
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	_, err = New(roDB)
	if err == nil {
		t.Fatal("expected open/write-probe to fail in read-only directory")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not writable") && !strings.Contains(strings.ToLower(msg), "readonly") {
		t.Fatalf("error should mention not writable/readonly, got: %v", err)
	}
	if !strings.Contains(msg, roDir) && !strings.Contains(msg, roDB) {
		t.Fatalf("error should include db or directory path, got: %v", err)
	}
}

func TestSessionCreateIncludesDBPathOnFailure(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Now().UTC()
	err = st.Sessions().Create(context.Background(), domain.Session{
		ID:        "session-1",
		AgentID:   "team",
		Content:   "hi",
		Status:    domain.SessionStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Duplicate primary key — wrapped with db path for diagnostics.
	err = st.Sessions().Create(context.Background(), domain.Session{
		ID:        "session-1",
		AgentID:   "team",
		Content:   "again",
		Status:    domain.SessionStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected duplicate create to fail")
	}
	if !strings.Contains(err.Error(), st.Path()) {
		t.Fatalf("create error should include db path %q, got: %v", st.Path(), err)
	}
}

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestNewEnablesWALAndRejectsUnwritableDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "work.db")
	st, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := st.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := sqlDB.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode=%q want wal", mode)
	}
	if err := st.Sessions().Create(context.Background(), domain.Session{
		ID: "s1", AgentID: "a1", Status: domain.SessionStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	roDir := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	_, err = New(filepath.Join(roDir, "work.db"))
	if err == nil {
		t.Fatal("expected error for unwritable database directory")
	}
	if !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("error=%v", err)
	}
}

func TestSessionCreateSurvivesAfterFileReplaceWithFreshOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "work.db")
	st, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Sessions().Create(context.Background(), domain.Session{
		ID: "before", AgentID: "a1", Status: domain.SessionStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := st.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	// Simulate an external replace of the database file, then reopen.
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	st2, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st2.Sessions().Create(context.Background(), domain.Session{
		ID: "after", AgentID: "a1", Status: domain.SessionStatusActive,
	}); err != nil {
		t.Fatalf("write after reopen: %v", err)
	}
}

func TestSessionCreateRetriesAfterDBMoved(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "work.db")
	st, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Replace the file while the Store still holds the old inode.
	sqlDB, err := st.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB // keep handle open via Store
	tmp := dbPath + ".new"
	stTmp, err := New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	sqlTmp, _ := stTmp.DB().DB()
	_ = sqlTmp.Close()
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	if err := os.Rename(tmp, dbPath); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(tmp + "-wal")
	_ = os.Remove(tmp + "-shm")

	if err := st.Sessions().Create(context.Background(), domain.Session{
		ID: "recovered", AgentID: "a1", Status: domain.SessionStatusActive,
	}); err != nil {
		t.Fatalf("expected reopen+retry to succeed, got %v", err)
	}
}

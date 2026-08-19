package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeHonorsWorkHome(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "custom-root")
	t.Setenv("WORK_HOME", root)

	if Home() != root {
		t.Fatalf("Home() = %q, want %q", Home(), root)
	}
	if _, err := os.Stat(filepath.Join(root, "data")); err != nil {
		t.Fatalf("data dir not created under WORK_HOME: %v", err)
	}
	// Every derived path stays inside the root.
	if got := DatabaseFile(); got != filepath.Join(root, "work.db") {
		t.Fatalf("DatabaseFile() = %q", got)
	}
	if got := StoreDatabaseFile(); got != filepath.Join(root, "store.db") {
		t.Fatalf("StoreDatabaseFile() = %q", got)
	}
	if got := HistoryDatabaseFileFor(DatabaseFile()); got != filepath.Join(root, "history.db") {
		t.Fatalf("HistoryDatabaseFileFor() = %q", got)
	}
	if got := ResolveAgainstHome("data"); got != filepath.Join(root, "data") {
		t.Fatalf("ResolveAgainstHome() = %q", got)
	}
}

func TestHomeLayoutAndMigrate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("WORK_HOME", "")

	// Migrate from pre-Danmo layout (~/.dq-teams/teams.db).
	legacyDir := filepath.Join(tmp, ".dq-teams")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyDB := filepath.Join(legacyDir, "teams.db")
	if err := os.WriteFile(legacyDB, []byte("sqlite-fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	MigrateLegacyOnce()

	wantHome := filepath.Join(tmp, ".danmo-work")
	if Home() != wantHome {
		t.Fatalf("Home() = %q, want %q", Home(), wantHome)
	}
	got, err := os.ReadFile(DatabaseFile())
	if err != nil {
		t.Fatalf("migrated db missing: %v", err)
	}
	if string(got) != "sqlite-fake" {
		t.Fatalf("migrated db content = %q", got)
	}
	// Second migrate must not overwrite.
	if err := os.WriteFile(legacyDB, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	MigrateLegacyOnce()
	got, _ = os.ReadFile(DatabaseFile())
	if string(got) != "sqlite-fake" {
		t.Fatalf("db was overwritten on second migrate: %q", got)
	}
}

func TestResolveAgainstHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("WORK_HOME", "")
	rel := ResolveAgainstHome("data")
	if rel != filepath.Join(tmp, ".danmo-work", "data") {
		t.Fatalf("rel = %q", rel)
	}
	abs := ResolveAgainstHome("/var/tmp/x")
	if abs != "/var/tmp/x" {
		t.Fatalf("abs = %q", abs)
	}
}

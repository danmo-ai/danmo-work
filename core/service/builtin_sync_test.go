package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/paths"
	"danmo-work/core/resource/home"
)

func TestSyncBuiltinToFSCopiesAgentsSkills(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")

	skillPath := filepath.Join(dataDir, "skills", "debugging", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("TAMPERED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".builtin_version"), []byte("old-hash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncBuiltinToFS(dataDir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "TAMPERED") {
		t.Fatal("builtin skill was not overwritten")
	}

	agentPath := filepath.Join(dataDir, "agents", "explorer.md")
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("missing builtin agent: %v", err)
	}

	// Migrated packs must not remain under home sync targets.
	for _, name := range []string{"github.md", "novel.md", "browser.md", "danmo-make.md"} {
		if _, err := os.Stat(filepath.Join(dataDir, "agents", name)); err == nil {
			t.Fatalf("migrated agent %s should not be synced from home", name)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.KnowledgeDir(), "kb-novel-craft")); err == nil {
		t.Fatal("kb-novel-craft should not sync to KnowledgeDir from home")
	}

	ver, err := os.ReadFile(filepath.Join(dataDir, ".builtin_version"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := home.BuiltinContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != hash {
		t.Fatalf("version file=%q want content hash %q", strings.TrimSpace(string(ver)), hash)
	}
}

func TestSyncBuiltinToFSSkipsWhenHashMatches(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")

	if err := SyncBuiltinToFS(dataDir); err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(dataDir, "skills", "debugging", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("TAMPERED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SyncBuiltinToFS(dataDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "TAMPERED" {
		t.Fatal("matching content hash should skip overwrite")
	}
}

func TestBuiltinFileTargetRoutesSkills(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")

	got := builtinFileTarget(dataDir, "skills/debugging/SKILL.md")
	want := filepath.Join(dataDir, "skills", "debugging", "SKILL.md")
	if got != want {
		t.Fatalf("skill target=%q want %q", got, want)
	}
}

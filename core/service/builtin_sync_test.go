package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/paths"
	"danmo-work/core/resource/home"
)

func TestSyncBuiltinToFSCopiesAgentsSkillsKnowledge(t *testing.T) {
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

	kbMeta := filepath.Join(paths.KnowledgeDir(), "kb-novel-craft", "_meta.json")
	if _, err := os.Stat(kbMeta); err != nil {
		t.Fatalf("knowledge not written to KnowledgeDir: %v", err)
	}
	kbDoc := filepath.Join(paths.KnowledgeDir(), "kb-novel-craft", "01-pacing-structure.md")
	if _, err := os.Stat(kbDoc); err != nil {
		t.Fatalf("knowledge markdown not written to KnowledgeDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "knowledge", "kb-novel-craft")); err == nil {
		t.Fatal("knowledge should not be copied under dataDir")
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

func TestBuiltinFileTargetRoutesKnowledge(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORK_HOME", root)
	dataDir := filepath.Join(root, "data")

	got := builtinFileTarget(dataDir, "knowledge/kb-novel-craft/_meta.json")
	want := filepath.Join(paths.KnowledgeDir(), "kb-novel-craft", "_meta.json")
	if got != want {
		t.Fatalf("knowledge target=%q want %q", got, want)
	}
	got = builtinFileTarget(dataDir, "skills/debugging/SKILL.md")
	want = filepath.Join(dataDir, "skills", "debugging", "SKILL.md")
	if got != want {
		t.Fatalf("skill target=%q want %q", got, want)
	}
}

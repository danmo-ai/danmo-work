package paths

import (
	"path/filepath"
	"testing"
)

func TestUserSkillDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dirs := UserSkillDirs()
	if len(dirs) != 2 {
		t.Fatalf("len=%d", len(dirs))
	}
	if dirs[0] != filepath.Join(tmp, ".agents", "skills") {
		t.Fatalf("agents = %q", dirs[0])
	}
	if dirs[1] != filepath.Join(tmp, ".danmo-work", "skills") {
		t.Fatalf("danmo-work = %q", dirs[1])
	}
}

func TestProjectSkillDirs(t *testing.T) {
	root := "/proj/root"
	dirs := ProjectSkillDirs(root)
	if len(dirs) != 2 {
		t.Fatalf("len=%d", len(dirs))
	}
	if dirs[0] != filepath.Join(root, ".agents", "skills") {
		t.Fatalf("agents = %q", dirs[0])
	}
	if dirs[1] != filepath.Join(root, ".danmo-work", "skills") {
		t.Fatalf("danmo-work = %q", dirs[1])
	}
	if ProjectSkillDirs("") != nil {
		t.Fatal("empty workDir should return nil")
	}
}

func TestAllSkillDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	work := filepath.Join(tmp, "work")
	dirs := AllSkillDirs(work)
	if len(dirs) != 4 {
		t.Fatalf("len=%d want 4", len(dirs))
	}
	wantLast := filepath.Join(work, ".danmo-work", "skills")
	if dirs[3] != wantLast {
		t.Fatalf("last = %q want %q", dirs[3], wantLast)
	}
}

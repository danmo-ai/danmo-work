package paths

import (
	"path/filepath"
	"testing"
)

func TestGlobalSkillDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dirs := GlobalSkillDirs()
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

func TestHomeSkillDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dataDir := filepath.Join(tmp, ".danmo-work", "data")
	got := HomeSkillDirs(dataDir)
	want := []string{
		filepath.Join(tmp, ".agents", "skills"),
		filepath.Join(tmp, ".danmo-work", "skills"),
		filepath.Join(dataDir, "skills"),
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d got=%v want=%v", len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("i=%d got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestJoinSkillID(t *testing.T) {
	got := JoinSkillID("/root/skills", "foo/bar")
	want := filepath.Join("/root/skills", "foo", "bar")
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
	if JoinSkillID("/root", "..") != "" || JoinSkillID("/root", "") != "" {
		t.Fatal("expected empty for invalid id")
	}
}

func TestSkillLookupRootsOrder(t *testing.T) {
	got := SkillLookupRoots(
		[]string{"home-low", "home-high"},
		[]string{"plug-a", "plug-b"},
		[]string{"proj-agents", "proj-danmo"},
	)
	want := []string{"proj-danmo", "proj-agents", "plug-b", "plug-a", "home-high", "home-low"}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("i=%d got=%q want=%q", i, got[i], want[i])
		}
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

package service_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
)

func TestGetGitDiffModifiedAndUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

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
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "init")
	run("git", "checkout", "-b", "main")

	if err := os.WriteFile(filepath.Join(work, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "a.go")
	run("git", "commit", "-m", "init")

	if err := os.WriteFile(filepath.Join(work, "a.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "new.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, err := pm.Create(ctx, domain.CreateProjectRequest{Name: "diff-test", Directory: work})
	if err != nil {
		t.Fatal(err)
	}

	mod, err := pm.GetGitDiff(ctx, proj.ID, "a.go", false)
	if err != nil {
		t.Fatal(err)
	}
	if mod.Error != "" || mod.Code != "" {
		t.Fatalf("unexpected error: %+v", mod)
	}
	if !strings.Contains(mod.Patch, "+func main()") {
		t.Fatalf("expected addition in patch, got:\n%s", mod.Patch)
	}

	untracked, err := pm.GetGitDiff(ctx, proj.ID, "new.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if !untracked.Untracked {
		t.Fatalf("expected untracked: %+v", untracked)
	}
	if !strings.Contains(untracked.Patch, "+hello") {
		t.Fatalf("expected synthesized add patch, got:\n%s", untracked.Patch)
	}

	changes, err := pm.GetGitChanges(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sawUntracked bool
	for _, c := range changes.Changes {
		if c.File == "new.txt" {
			sawUntracked = true
			if c.Staged {
				t.Fatalf("untracked file must not be staged: %+v", c)
			}
			if c.Status != "??" && c.Status != "?" {
				t.Fatalf("unexpected untracked status: %+v", c)
			}
		}
	}
	if !sawUntracked {
		t.Fatalf("expected new.txt in changes: %+v", changes.Changes)
	}

	run("git", "add", "a.go")
	staged, err := pm.GetGitDiff(ctx, proj.ID, "a.go", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(staged.Patch, "+func main()") {
		t.Fatalf("expected staged patch, got:\n%s", staged.Patch)
	}
}

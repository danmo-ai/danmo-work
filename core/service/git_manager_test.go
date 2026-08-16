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

// setupGitTestRepo creates a project whose directory is a git repo with one
// commit, a bare file:// remote, and a modified + untracked file.
func setupGitTestRepo(t *testing.T) (ctx context.Context, pm *service.ProjectManager, git *service.GitManager, proj domain.Project, remoteURL string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx = context.Background()
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	pm = service.NewProjectManager(st, dir)
	git = service.NewGitManager(pm)
	git.SetSecretStore(st.Secrets())

	work := filepath.Join(dir, "repo")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(cwd string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_CONFIG_GLOBAL=", "GIT_CONFIG_SYSTEM=",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run(work, "git", "init")
	run(work, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "git", "add", "a.txt")
	run(work, "git", "commit", "-m", "init")

	bare := filepath.Join(dir, "remote.git")
	run(dir, "git", "init", "--bare", bare)
	run(work, "git", "remote", "add", "origin", "file://"+bare)

	proj, err = pm.Create(ctx, domain.CreateProjectRequest{Name: "git-panel-test", Directory: work})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, pm, git, proj, "file://" + bare
}

func TestGitRemotesAndLog(t *testing.T) {
	ctx, _, git, proj, _ := setupGitTestRepo(t)

	remotes, err := git.ListGitRemotes(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remotes.Remotes) != 1 || remotes.Remotes[0].Name != "origin" {
		t.Fatalf("expected origin remote: %+v", remotes)
	}

	log, err := git.GetGitLog(ctx, proj.ID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Commits) != 1 || log.Commits[0].Subject != "init" {
		t.Fatalf("expected one init commit: %+v", log.Commits)
	}
}

func TestGitStageCommitPushPullRoundtrip(t *testing.T) {
	ctx, _, git, proj, _ := setupGitTestRepo(t)
	work := proj.Directory

	if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := git.StageFiles(ctx, proj.ID, []string{"a.txt", "b.txt"}, true)
	if err != nil {
		t.Fatal(err)
	}
	staged := 0
	for _, c := range changes.Changes {
		if c.Staged {
			staged++
		}
	}
	if staged != 2 {
		t.Fatalf("expected 2 staged files: %+v", changes.Changes)
	}

	if _, err := git.Commit(ctx, proj.ID, "panel commit"); err != nil {
		t.Fatal(err)
	}

	log, err := git.GetGitLog(ctx, proj.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Commits) != 2 || log.Commits[0].Subject != "panel commit" {
		t.Fatalf("expected new commit on top: %+v", log.Commits)
	}

	// file:// remotes need no credentials; push/pull must succeed without any.
	events := make(chan service.GitStreamEvent, 128)
	pushCh, err := git.StreamGitOp(ctx, proj.ID, "push")
	if err != nil {
		t.Fatal(err)
	}
	for ev := range pushCh {
		events <- ev
	}
	close(events)
	var pushDone int
	for ev := range events {
		if ev.Type == "done" {
			pushDone = ev.Exit
		}
	}
	if pushDone != 0 {
		t.Fatalf("push failed with exit %d", pushDone)
	}

	// Change remote branch and pull locally.
	remoteWork := t.TempDir()
	run := func(cwd string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	bare := filepath.Dir(proj.Directory) + "/remote.git"
	run(remoteWork, "git", "clone", "file://"+bare, ".")
	run(remoteWork, "git", "config", "user.email", "t@t")
	run(remoteWork, "git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(remoteWork, "remote.txt"), []byte("from remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(remoteWork, "git", "add", "remote.txt")
	run(remoteWork, "git", "commit", "-m", "remote change")
	run(remoteWork, "git", "push")

	pullCh, err := git.StreamGitOp(ctx, proj.ID, "pull")
	if err != nil {
		t.Fatal(err)
	}
	var pulled int
	for ev := range pullCh {
		if ev.Type == "done" {
			pulled = ev.Exit
		}
	}
	if pulled != 0 {
		t.Fatalf("pull failed with exit %d", pulled)
	}
	if _, err := os.Stat(filepath.Join(work, "remote.txt")); err != nil {
		t.Fatalf("expected remote.txt after pull: %v", err)
	}
}

func TestGitCredentialsStoredAndFileDerived(t *testing.T) {
	ctx, _, git, proj, remoteURL := setupGitTestRepo(t)

	// Project remote host is empty for file:// — credentials stored without verification.
	host := "github.com"
	if err := git.PutGitCredential(ctx, proj.ID, host, "octocat", "secret-token-123"); err != nil {
		t.Fatal(err)
	}

	creds, err := git.GitCredentialStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range creds {
		if c.Host == host && c.User == "octocat" && c.HasToken {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stored credential: %+v", creds)
	}

	dir := git.CredentialDir()
	if dir == "" {
		t.Fatal("credential dir empty")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".git-credentials"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://octocat:secret-token-123@github.com") {
		t.Fatalf("derived credential file wrong:\n%s", data)
	}

	if err := git.DeleteGitCredential(ctx, host); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, ".git-credentials"))
	if strings.Contains(string(data), "secret-token-123") {
		t.Fatalf("credential file not cleared:\n%s", data)
	}
	if remoteURL == "" {
		t.Fatal("unreachable")
	}
}

func TestGitStreamOpBusy(t *testing.T) {
	ctx, _, git, proj, _ := setupGitTestRepo(t)

	first, err := git.StreamGitOp(ctx, proj.ID, "fetch")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for range first {
		}
	}()
	_, err = git.StreamGitOp(ctx, proj.ID, "fetch")
	if err != service.ErrGitBusy {
		t.Fatalf("expected ErrGitBusy, got %v", err)
	}
}

func TestGitAddRemoteValidation(t *testing.T) {
	ctx, _, git, proj, _ := setupGitTestRepo(t)

	if _, err := git.AddGitRemote(ctx, proj.ID, "bad name", "https://example.com/r.git"); err == nil {
		t.Fatal("expected invalid name error")
	}
	if _, err := git.AddGitRemote(ctx, proj.ID, "ok", "not-a-url"); err == nil {
		t.Fatal("expected invalid url error")
	}
	if _, err := git.AddGitRemote(ctx, proj.ID, "upstream", "https://example.com/team/repo.git"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	remotes, _ := git.ListGitRemotes(ctx, proj.ID)
	if len(remotes.Remotes) != 2 {
		t.Fatalf("expected 2 remotes: %+v", remotes)
	}
}

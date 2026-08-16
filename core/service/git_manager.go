package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"danmo-work/core/paths"
	"danmo-work/core/port"
)

// GitManager powers the Git work panel: HTTPS credentials (per remote host),
// remotes, stage/unstage, commit, log, and pull/push/fetch with line streaming.
// Credentials live in the encrypted SecretStore (source of truth) and are
// derived into a plaintext credential file that container sandboxes bind-mount.
type GitManager struct {
	projects *ProjectManager
	secrets  port.SecretStore
	credDir  string
	mu       sync.Mutex
	ops      map[string]chan GitStreamEvent
}

// ErrGitBusy is returned when another git operation is already running for a project.
var ErrGitBusy = errors.New("another git operation is in progress")

const (
	gitOpTimeout   = 10 * time.Minute
	gitCredPrefix  = "git.cred."
	gitCredHosts   = "git.cred.hosts"
	gitCredDirName = "git-cred"
	gitCredFile    = ".git-credentials"
)

type gitCredential struct {
	User  string `json:"user"`
	Token string `json:"token"`
}

// GitRemote is one configured remote with masked URLs.
type GitRemote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetchUrl"`
	PushURL  string `json:"pushUrl,omitempty"`
}

type GitRemotes struct {
	Remotes []*GitRemote `json:"remotes"`
	Error   string       `json:"error,omitempty"`
	Code    string       `json:"code,omitempty"` // git_missing | init_failed
}

// GitCredentialInfo describes a stored credential without exposing the token.
type GitCredentialInfo struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	HasToken bool   `json:"hasToken"`
}

type GitCommitInfo struct {
	Hash    string `json:"hash"`
	Short   string `json:"short"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

type GitLog struct {
	Commits []*GitCommitInfo `json:"commits"`
	Error   string           `json:"error,omitempty"`
	Code    string           `json:"code,omitempty"` // git_missing | init_failed
}

// GitStreamEvent is one SSE payload for pull/push/fetch progress.
type GitStreamEvent struct {
	Type string `json:"type"`           // line | done | error
	Data string `json:"data,omitempty"` // line text, done summary, or error detail
	Exit int    `json:"exit,omitempty"` // done only
}

// NewGitManager wires the git panel service. The credential dir is created
// eagerly (with an empty .git-credentials) so project containers can always
// bind-mount it, even before the user logs in.
func NewGitManager(projects *ProjectManager) *GitManager {
	credDir := filepath.Join(paths.Home(), gitCredDirName)
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		credDir = ""
	}
	m := &GitManager{
		projects: projects,
		credDir:  credDir,
		ops:      make(map[string]chan GitStreamEvent),
	}
	m.ensureCredentialFile()
	return m
}

func (m *GitManager) SetSecretStore(s port.SecretStore) { m.secrets = s }

// CredentialDir implements port.GitCredentialProvider for container backends.
func (m *GitManager) CredentialDir() string { return m.credDir }

// ---------------------------------------------------------------------------
// Credentials

func normalizeGitHost(raw string) string {
	host := strings.TrimSpace(strings.ToLower(raw))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "ssh://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if u, err := url.Parse("//" + host); err == nil && u.Hostname() != "" {
		host = u.Host
	}
	host = strings.TrimSuffix(host, ".")
	if host == "" || strings.ContainsAny(host, " \t\r\n") {
		return ""
	}
	return host
}

func hostOfURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		return u.Host
	}
	// scp-like: git@github.com:owner/repo.git
	if i := strings.Index(raw, "@"); i >= 0 {
		rest := raw[i+1:]
		if j := strings.Index(rest, ":"); j > 0 {
			return rest[:j]
		}
	}
	return ""
}

// maskURL strips embedded credentials from a remote URL for display.
func maskURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, has := u.User.Password(); has {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}

func (m *GitManager) hostList(ctx context.Context) ([]string, error) {
	raw, err := m.secrets.Get(ctx, gitCredHosts)
	if err != nil {
		return nil, nil
	}
	var hosts []string
	if err := json.Unmarshal([]byte(raw), &hosts); err != nil {
		return nil, nil
	}
	return hosts, nil
}

func (m *GitManager) saveHostList(ctx context.Context, hosts []string) error {
	sort.Strings(hosts)
	data, err := json.Marshal(hosts)
	if err != nil {
		return err
	}
	return m.secrets.Put(ctx, gitCredHosts, string(data))
}

func (m *GitManager) credential(ctx context.Context, host string) (*gitCredential, error) {
	raw, err := m.secrets.Get(ctx, gitCredPrefix+host)
	if err != nil {
		return nil, err
	}
	var c gitCredential
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, err
	}
	if c.Token == "" {
		return nil, fmt.Errorf("no token")
	}
	return &c, nil
}

// GitCredentialStatus lists stored credentials (never the tokens themselves).
func (m *GitManager) GitCredentialStatus(ctx context.Context) ([]GitCredentialInfo, error) {
	if m.secrets == nil {
		return nil, fmt.Errorf("git credentials unavailable")
	}
	hosts, err := m.hostList(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]GitCredentialInfo, 0, len(hosts))
	for _, h := range hosts {
		c, err := m.credential(ctx, h)
		if err != nil {
			out = append(out, GitCredentialInfo{Host: h})
			continue
		}
		out = append(out, GitCredentialInfo{Host: h, User: c.User, HasToken: c.Token != ""})
	}
	return out, nil
}

// PutGitCredential stores HTTPS credentials for a host. When the project has a
// remote on that host, the credentials are verified via ls-remote first.
func (m *GitManager) PutGitCredential(ctx context.Context, projectID, host, username, token string) error {
	if m.secrets == nil {
		return fmt.Errorf("git credentials unavailable")
	}
	host = normalizeGitHost(host)
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = "git"
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token required")
	}

	if err := m.verifyCredential(ctx, projectID, host, username, token); err != nil {
		return err
	}

	data, err := json.Marshal(gitCredential{User: username, Token: token})
	if err != nil {
		return err
	}
	if err := m.secrets.Put(ctx, gitCredPrefix+host, string(data)); err != nil {
		return err
	}
	hosts, err := m.hostList(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, h := range hosts {
		if h == host {
			found = true
			break
		}
	}
	if !found {
		if err := m.saveHostList(ctx, append(hosts, host)); err != nil {
			return err
		}
	}
	return m.rewriteCredentialFile(ctx)
}

// DeleteGitCredential removes stored credentials for a host.
func (m *GitManager) DeleteGitCredential(ctx context.Context, host string) error {
	if m.secrets == nil {
		return fmt.Errorf("git credentials unavailable")
	}
	host = normalizeGitHost(host)
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	if err := m.secrets.Delete(ctx, gitCredPrefix+host); err != nil {
		return err
	}
	hosts, _ := m.hostList(ctx)
	kept := hosts[:0]
	for _, h := range hosts {
		if h != host {
			kept = append(kept, h)
		}
	}
	_ = m.saveHostList(ctx, kept)
	return m.rewriteCredentialFile(ctx)
}

// verifyCredential runs ls-remote against the project remote on host when one
// exists; projects without a matching remote skip verification.
func (m *GitManager) verifyCredential(ctx context.Context, projectID, host, username, token string) error {
	if projectID == "" {
		return nil
	}
	gitRoot, _, code, _, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil || code != "" {
		return nil
	}
	var target string
	for _, u := range m.remoteURLs(gitRoot) {
		if hostOfURL(u) == host {
			target = u
			break
		}
	}
	if target == "" {
		return nil
	}
	cmd := exec.Command(ResolveGitBin(), "-c", "credential.helper=", "ls-remote", injectCreds(target, username, token))
	cmd.Dir = gitRoot
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("认证失败: %s", firstLine(detail))
	}
	return nil
}

// injectCreds rewrites an https URL with embedded credentials.
func injectCreds(rawURL, user, token string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" {
		return rawURL
	}
	u.User = url.UserPassword(user, token)
	return u.String()
}

// rewriteCredentialFile derives the plaintext .git-credentials from SecretStore.
func (m *GitManager) rewriteCredentialFile(ctx context.Context) error {
	if m.credDir == "" || m.secrets == nil {
		return nil
	}
	hosts, err := m.hostList(ctx)
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, h := range hosts {
		c, err := m.credential(ctx, h)
		if err != nil || c.Token == "" {
			continue
		}
		b.WriteString("https://" + url.QueryEscape(c.User) + ":" + url.QueryEscape(c.Token) + "@" + h + "\n")
	}
	p := filepath.Join(m.credDir, gitCredFile)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (m *GitManager) ensureCredentialFile() {
	if m.credDir == "" {
		return
	}
	p := filepath.Join(m.credDir, gitCredFile)
	if _, err := os.Stat(p); err == nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_ = f.Close()
}

// ---------------------------------------------------------------------------
// Remotes

func (m *GitManager) remoteURLs(gitRoot string) map[string]string {
	out := map[string]string{}
	cmd := exec.Command(ResolveGitBin(), "remote", "-v")
	cmd.Dir = gitRoot
	if raw, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			if _, ok := out[fields[0]]; !ok {
				out[fields[0]] = fields[1]
			}
		}
	}
	return out
}

func (m *GitManager) ListGitRemotes(ctx context.Context, projectID string) (*GitRemotes, error) {
	gitRoot, _, code, msg, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return &GitRemotes{Error: msg, Code: code}, nil
	}

	cmd := exec.Command(ResolveGitBin(), "remote", "-v")
	cmd.Dir = gitRoot
	raw, err := cmd.Output()
	if err != nil {
		return &GitRemotes{Remotes: []*GitRemote{}}, nil
	}
	result := &GitRemotes{Remotes: []*GitRemote{}}
	byName := map[string]*GitRemote{}
	var names []string
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		name, u, kind := fields[0], fields[1], strings.Trim(fields[2], "()")
		r := byName[name]
		if r == nil {
			r = &GitRemote{Name: name}
			byName[name] = r
			names = append(names, name)
		}
		switch kind {
		case "push":
			r.PushURL = maskURL(u)
		default:
			r.FetchURL = maskURL(u)
		}
	}
	for _, n := range names {
		result.Remotes = append(result.Remotes, byName[n])
	}
	return result, nil
}

func (m *GitManager) AddGitRemote(ctx context.Context, projectID, name, remoteURL string) (*GitRemotes, error) {
	name = strings.TrimSpace(name)
	remoteURL = strings.TrimSpace(remoteURL)
	if name == "" || len(name) > 64 || strings.HasPrefix(name, "-") {
		return nil, fmt.Errorf("invalid remote name")
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '.' && r != '_' && r != '-' {
			return nil, fmt.Errorf("invalid remote name")
		}
	}
	if strings.HasPrefix(remoteURL, "-") {
		return nil, fmt.Errorf("invalid remote url")
	}
	u, err := url.Parse(remoteURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return nil, fmt.Errorf("remote url must be http(s)")
	}

	gitRoot, _, code, msg, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	cmd := exec.Command(ResolveGitBin(), "remote", "add", "--", name, remoteURL)
	cmd.Dir = gitRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return m.ListGitRemotes(ctx, projectID)
}

// ---------------------------------------------------------------------------
// Stage / commit / log

func validGitPath(p string) bool {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	return p != "" && p != "." && !strings.Contains(p, "..") &&
		!strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "-") &&
		!strings.Contains(p, "\x00")
}

// StageFiles stages (staged=true → git add) or unstages (staged=false →
// git restore --staged) the given project-relative paths.
func (m *GitManager) StageFiles(ctx context.Context, projectID string, files []string, staged bool) (*GitChanges, error) {
	gitRoot, _, code, msg, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	clean := make([]string, 0, len(files))
	for _, f := range files {
		if !validGitPath(f) {
			return nil, fmt.Errorf("invalid file path")
		}
		clean = append(clean, strings.TrimSpace(strings.ReplaceAll(f, "\\", "/")))
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("no files")
	}

	var args []string
	if staged {
		args = []string{"add", "--"}
	} else {
		args = []string{"restore", "--staged", "--"}
	}
	args = append(args, clean...)
	cmd := exec.Command(ResolveGitBin(), args...)
	cmd.Dir = gitRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && !staged {
		// Fallback for git < 2.23: mixed reset unstages the index.
		legacy := append([]string{"reset", "-q", "HEAD", "--"}, clean...)
		legacyCmd := exec.Command(ResolveGitBin(), legacy...)
		legacyCmd.Dir = gitRoot
		var legacyErr bytes.Buffer
		legacyCmd.Stderr = &legacyErr
		if legacyErr2 := legacyCmd.Run(); legacyErr2 != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = err.Error()
			}
			return nil, fmt.Errorf("%s", detail)
		}
	} else if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return m.projects.GetGitChanges(ctx, projectID)
}

// Commit creates a commit with the given message on the current branch.
func (m *GitManager) Commit(ctx context.Context, projectID, message string) (*GitLog, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("commit message required")
	}
	gitRoot, _, code, msg, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	cmd := exec.Command(ResolveGitBin(), "commit", "-m", message)
	cmd.Dir = gitRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return m.GetGitLog(ctx, projectID, 1)
}

func (m *GitManager) GetGitLog(ctx context.Context, projectID string, limit int) (*GitLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	gitRoot, _, code, msg, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return &GitLog{Error: msg, Code: code}, nil
	}
	cmd := exec.Command(ResolveGitBin(), "log", "--pretty=format:%H%x1f%h%x1f%an%x1f%ad%x1f%s", "--date=short", "-n", strconv.Itoa(limit))
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	result := &GitLog{}
	if err != nil {
		// A repo without commits is not an error.
		result.Commits = []*GitCommitInfo{}
		return result, nil
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\x1f", 5)
		if len(parts) < 5 {
			continue
		}
		result.Commits = append(result.Commits, &GitCommitInfo{
			Hash:    parts[0],
			Short:   parts[1],
			Author:  parts[2],
			Date:    parts[3],
			Subject: parts[4],
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Pull / push / fetch streaming

// StreamGitOp starts a pull/push/fetch for the project and returns the event
// channel. Only one op may run per project at a time.
func (m *GitManager) StreamGitOp(ctx context.Context, projectID, op string) (<-chan GitStreamEvent, error) {
	switch op {
	case "pull", "push", "fetch":
	default:
		return nil, fmt.Errorf("invalid op")
	}
	m.mu.Lock()
	if _, busy := m.ops[projectID]; busy {
		m.mu.Unlock()
		return nil, ErrGitBusy
	}
	ch := make(chan GitStreamEvent, 256)
	m.ops[projectID] = ch
	m.mu.Unlock()

	go func() {
		defer func() {
			close(ch)
			m.mu.Lock()
			delete(m.ops, projectID)
			m.mu.Unlock()
		}()
		m.runGitOp(ctx, projectID, op, ch)
	}()
	return ch, nil
}

func emitGit(ctx context.Context, ch chan<- GitStreamEvent, ev GitStreamEvent) bool {
	select {
	case ch <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *GitManager) runGitOp(ctx context.Context, projectID, op string, ch chan<- GitStreamEvent) {
	gitRoot, _, code, msg, err := m.projects.ensureGitRepo(ctx, projectID)
	if err != nil {
		emitGit(ctx, ch, GitStreamEvent{Type: "error", Data: err.Error()})
		return
	}
	if code != "" {
		emitGit(ctx, ch, GitStreamEvent{Type: "error", Data: msg})
		return
	}

	args, redactions, err := m.gitOpArgs(ctx, gitRoot, op)
	if err != nil {
		emitGit(ctx, ch, GitStreamEvent{Type: "error", Data: err.Error()})
		return
	}

	opCtx, cancel := context.WithTimeout(ctx, gitOpTimeout)
	defer cancel()

	cmd := exec.CommandContext(opCtx, ResolveGitBin(), args...)
	cmd.Dir = gitRoot
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emitGit(ctx, ch, GitStreamEvent{Type: "error", Data: err.Error()})
		return
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		emitGit(ctx, ch, GitStreamEvent{Type: "error", Data: err.Error()})
		return
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if !emitGit(ctx, ch, GitStreamEvent{Type: "line", Data: sanitizeGitLine(scanner.Text(), redactions)}) {
			_ = cmd.Process.Kill()
			return
		}
	}
	waitErr := <-waitCh

	exit := 0
	if waitErr != nil {
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			exit = ee.ExitCode()
		} else {
			exit = -1
		}
	}
	summary := op + " done"
	if exit != 0 {
		summary = op + " failed"
	}
	emitGit(ctx, ch, GitStreamEvent{Type: "done", Data: summary, Exit: exit})
}

// gitOpArgs builds the git CLI args for pull/push/fetch with credentials
// injected via command-scoped remote URL overrides. Local remotes (file paths,
// no resolvable host) skip credential handling entirely.
func (m *GitManager) gitOpArgs(ctx context.Context, gitRoot, op string) (args []string, redactions []string, err error) {
	args = []string{"-c", "credential.helper="}
	remotes := m.remoteURLs(gitRoot)
	if len(remotes) == 0 {
		return nil, nil, fmt.Errorf("尚未配置远端仓库 — 请先添加 remote")
	}
	authHosts := 0
	for _, u := range remotes {
		if hostOfURL(u) != "" {
			authHosts++
		}
	}
	if authHosts == 0 {
		// Local-only remotes need no credentials.
		switch op {
		case "pull":
			args = append(args, "pull")
		case "fetch":
			args = append(args, "fetch", "--prune")
		case "push":
			args = append(args, "push")
			if branch := gitCurrentBranch(gitRoot); branch != "" && !gitHasUpstream(gitRoot, branch) {
				args = append(args, "-u", "origin", branch)
			}
		}
		return args, nil, nil
	}

	credFound := false
	for name, u := range remotes {
		host := hostOfURL(u)
		if host == "" {
			continue
		}
		cred, cerr := m.credential(ctx, host)
		if cerr != nil || cred.Token == "" {
			continue
		}
		credFound = true
		rew := injectCreds(u, cred.User, cred.Token)
		args = append(args, "-c", fmt.Sprintf("remote.%s.url=%s", name, rew))
		redactions = append(redactions, cred.Token, url.QueryEscape(cred.Token), rew)
	}
	if !credFound {
		return nil, nil, fmt.Errorf("远端主机尚未登录 — 请先在面板中配置凭据")
	}

	switch op {
	case "pull":
		args = append(args, "pull")
	case "fetch":
		args = append(args, "fetch", "--prune")
	case "push":
		args = append(args, "push")
		if branch := gitCurrentBranch(gitRoot); branch != "" && !gitHasUpstream(gitRoot, branch) {
			args = append(args, "-u", "origin", branch)
		}
	}
	return args, redactions, nil
}

func gitHasUpstream(gitRoot, branch string) bool {
	cmd := exec.Command(ResolveGitBin(), "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = gitRoot
	return cmd.Run() == nil
}

func sanitizeGitLine(line string, redactions []string) string {
	for _, r := range redactions {
		if r == "" {
			continue
		}
		line = strings.ReplaceAll(line, r, "***")
	}
	return line
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"

)


type ProjectManager struct {
	store   port.Repository
	dataDir string
}

func NewProjectManager(store port.Repository, dataDir string) *ProjectManager {
	return &ProjectManager{store: store, dataDir: dataDir}
}

func (m *ProjectManager) ProjectDir(projectID string) string {
	return filepath.Join(m.dataDir, projectID)
}

func (m *ProjectManager) Create(ctx context.Context, req domain.CreateProjectRequest) (domain.Project, error) {
	if req.Name == "" {
		return domain.Project{}, fmt.Errorf("name required")
	}
	now := time.Now().UTC()
	projectID := fmt.Sprintf("proj-%d", time.Now().UnixNano())
	dir := req.Directory
	if dir == "" {
		dir = filepath.Join(m.ProjectDir(projectID), "files")
	}
	// Resolve relative paths against dataDir so we always store absolute paths.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(m.dataDir, dir)
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return domain.Project{}, fmt.Errorf("failed to create project directory: %w", err)
	}
	p := domain.Project{
		ID:        projectID,
		Name:      req.Name,
		Directory: dir,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := m.store.Projects().Create(ctx, p); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

func (m *ProjectManager) Get(ctx context.Context, id string) (domain.Project, error) {
	return m.store.Projects().Get(ctx, id)
}

func (m *ProjectManager) List(ctx context.Context) ([]domain.Project, error) {
	return m.store.Projects().List(ctx)
}

func (m *ProjectManager) Update(ctx context.Context, id string, req domain.UpdateProjectRequest) (domain.Project, error) {
	p, err := m.store.Projects().Get(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Directory != "" {
		dir := req.Directory
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(m.dataDir, dir)
		}
		p.Directory = filepath.Clean(dir)
	}
	p.UpdatedAt = time.Now().UTC()
	if err := m.store.Projects().Update(ctx, p); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

func (m *ProjectManager) Delete(ctx context.Context, id string) error {
	return m.store.Projects().Delete(ctx, id)
}

func (m *ProjectManager) SessionsForProject(ctx context.Context, projectID string) ([]domain.Session, error) {
	return m.store.Sessions().ListByProject(ctx, projectID)
}

func (m *ProjectManager) resolveFilesRoot(ctx context.Context, projectID string) (string, error) {
	p, err := m.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	root := p.Directory
	if root == "" {
		root = filepath.Join(m.ProjectDir(projectID), "files")
	}
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	return root, nil
}

func (m *ProjectManager) ResolveDir(ctx context.Context, projectID, fallbackDir string) string {
	p, err := m.Get(ctx, projectID)
	if err != nil {
		return fallbackDir
	}
	if p.Directory == "" {
		dir := filepath.Join(m.ProjectDir(projectID), "files")
		os.MkdirAll(dir, 0755)
		return dir
	}
	if filepath.IsAbs(p.Directory) {
		return p.Directory
	}
	return filepath.Join(fallbackDir, p.Directory)
}

// ResolveWorkDir returns the agent working directory for a project, matching
// runtime Engine.resolveWorkDir (empty projectID → dataDir).
func (m *ProjectManager) ResolveWorkDir(ctx context.Context, projectID string) string {
	if projectID == "" {
		return m.dataDir
	}
	return m.ResolveDir(ctx, projectID, m.dataDir)
}

type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Size     int64       `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

func (m *ProjectManager) ListFiles(ctx context.Context, projectID, subPath string) ([]*FileNode, error) {
	root, err := m.resolveFilesRoot(ctx, projectID)
	if err != nil {
		return nil, err
	}

	target := filepath.Join(root, subPath)
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(target, root) {
		return nil, fmt.Errorf("path escapes project directory")
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}

	nodes := make([]*FileNode, 0, len(entries))
	for _, e := range entries {
		rel, _ := filepath.Rel(root, filepath.Join(target, e.Name()))
		node := &FileNode{
			Name:  e.Name(),
			Path:  rel,
			IsDir: e.IsDir(),
		}
		if !e.IsDir() {
			info, _ := e.Info()
			if info != nil {
				node.Size = info.Size()
			}
		}
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

type FileContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
	Binary      bool   `json:"binary"`
}

// ReadFileRaw reads a project file and returns raw bytes + content type.
func (m *ProjectManager) ReadFileRaw(ctx context.Context, projectID, subPath string) ([]byte, string, error) {
	root, err := m.resolveFilesRoot(ctx, projectID)
	if err != nil {
		return nil, "", err
	}

	target := filepath.Join(root, subPath)
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, "", fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(target, root) {
		return nil, "", fmt.Errorf("path escapes project directory")
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("cannot read directory as file")
	}

	ext := filepath.Ext(target)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		// Fallback for extensions not covered by Go's mime package
		switch ext {
		case ".md", ".markdown":
			contentType = "text/markdown"
		case ".txt", ".log", ".csv", ".tsv":
			contentType = "text/plain"
		case ".xml", ".rss", ".atom":
			contentType = "application/xml"
		case ".yml", ".yaml":
			contentType = "text/yaml"
		default:
			contentType = "application/octet-stream"
		}
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return nil, "", err
	}

	return data, contentType, nil
}

func (m *ProjectManager) ReadFileContent(ctx context.Context, projectID, subPath string) (*FileContent, error) {
	root, err := m.resolveFilesRoot(ctx, projectID)
	if err != nil {
		return nil, err
	}

	target := filepath.Join(root, subPath)
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(target, root) {
		return nil, fmt.Errorf("path escapes project directory")
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot read directory as file")
	}

	ext := filepath.Ext(target)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "text/plain"
	}

	isBinary := false
	if strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/javascript" ||
		contentType == "application/xml" ||
		contentType == "image/svg+xml" {
		isBinary = false
	} else if strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "audio/") ||
		strings.HasPrefix(contentType, "video/") {
		isBinary = true
	}

	fc := &FileContent{
		Name:        info.Name(),
		Path:        subPath,
		Size:        info.Size(),
		ContentType: contentType,
		Binary:      isBinary,
	}

	if isBinary {
		data, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		fc.Content = fmt.Sprintf("data:%s;base64,%s", contentType, base64Encode(data))
		return fc, nil
	}

	f, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const maxSize = 1 << 20
	var buf strings.Builder
	if info.Size() > maxSize {
		lr := io.LimitReader(f, maxSize)
		data, _ := io.ReadAll(lr)
		buf.Write(data)
		buf.WriteString("\n\n... (file truncated)")
	} else {
		data, _ := io.ReadAll(f)
		buf.Write(data)
	}
	fc.Content = buf.String()
	return fc, nil
}

// WriteFileContent writes UTF-8 text content to a project-relative path.
// Parent directories are created as needed. Path must stay inside the project root.
func (m *ProjectManager) WriteFileContent(ctx context.Context, projectID, subPath, content string) error {
	return m.WriteFileBytes(ctx, projectID, subPath, []byte(content))
}

// WriteFileBytes writes raw bytes to a project-relative path.
// Parent directories are created as needed. Path must stay inside the project root.
func (m *ProjectManager) WriteFileBytes(ctx context.Context, projectID, subPath string, data []byte) error {
	root, err := m.resolveFilesRoot(ctx, projectID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(subPath) == "" {
		return fmt.Errorf("path is required")
	}
	if strings.Contains(subPath, "..") {
		return fmt.Errorf("path escapes project directory")
	}

	target := filepath.Join(root, subPath)
	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(target, root+string(os.PathSeparator)) && target != root {
		return fmt.Errorf("path escapes project directory")
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

// SanitizeUploadFilename returns a safe base filename for composer uploads.
func SanitizeUploadFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	const maxLen = 180
	if len(name) > maxLen {
		ext := filepath.Ext(name)
		if len(ext) > 40 {
			ext = ext[:40]
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		maxBase := maxLen - len(ext)
		if maxBase < 1 {
			maxBase = 1
		}
		if len(base) > maxBase {
			base = base[:maxBase]
		}
		name = base + ext
	}
	return name
}

// UploadFile writes data under uploads/<filename>, allocating a unique name if needed.
// Returns the project-relative path (slash-normalized).
func (m *ProjectManager) UploadFile(ctx context.Context, projectID, filename string, data []byte) (string, error) {
	name := SanitizeUploadFilename(filename)
	rel, err := m.uniqueUploadPath(ctx, projectID, name)
	if err != nil {
		return "", err
	}
	if err := m.WriteFileBytes(ctx, projectID, rel, data); err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (m *ProjectManager) uniqueUploadPath(ctx context.Context, projectID, name string) (string, error) {
	root, err := m.resolveFilesRoot(ctx, projectID)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 0; i < 1000; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d%s", base, i, ext)
		}
		rel := filepath.Join("uploads", candidate)
		target := filepath.Join(root, rel)
		if _, err := os.Stat(target); err != nil {
			if os.IsNotExist(err) {
				return rel, nil
			}
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate unique upload path")
}

type GitFileChange struct {
	Status   string `json:"status"`
	File     string `json:"file"`
	OrigFile string `json:"origFile,omitempty"`
	Staged   bool   `json:"staged"`
}

type GitChanges struct {
	Branch    string           `json:"branch"`
	Ahead     int              `json:"ahead,omitempty"`
	Behind    int              `json:"behind,omitempty"`
	HasRemote bool             `json:"hasRemote,omitempty"`
	Changes   []*GitFileChange `json:"changes"`
	Error     string           `json:"error,omitempty"`
	Code      string           `json:"code,omitempty"` // git_missing | init_failed
}

type GitBranches struct {
	Current  string   `json:"current"`
	Branches []string `json:"branches"`
	Error    string   `json:"error,omitempty"`
	Code     string   `json:"code,omitempty"` // git_missing | init_failed
}

func base64Encode(data []byte) string {
	enc := make([]byte, ((len(data)+2)/3)*4)
	encode(data, enc)
	return string(enc)
}

const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func encode(src, dst []byte) {
	di, si := 0, 0
	n := (len(src) / 3) * 3
	for si < n {
		val := uint(src[si])<<16 | uint(src[si+1])<<8 | uint(src[si+2])
		dst[di] = base64Table[val>>18&0x3F]
		dst[di+1] = base64Table[val>>12&0x3F]
		dst[di+2] = base64Table[val>>6&0x3F]
		dst[di+3] = base64Table[val&0x3F]
		si += 3
		di += 4
	}
	remain := len(src) - si
	if remain == 0 {
		return
	}
	val := uint(src[si]) << 16
	if remain == 2 {
		val |= uint(src[si+1]) << 8
	}
	dst[di] = base64Table[val>>18&0x3F]
	dst[di+1] = base64Table[val>>12&0x3F]
	if remain == 1 {
		dst[di+2] = '='
		dst[di+3] = '='
	} else {
		dst[di+2] = base64Table[val>>6&0x3F]
		dst[di+3] = '='
	}
}

func (m *ProjectManager) projectWorkDir(ctx context.Context, projectID string) (string, error) {
	p, err := m.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	root := p.Directory
	if root == "" {
		root = filepath.Join(m.ProjectDir(projectID), "files")
	}
	root = filepath.Clean(root)
	// Resolve symlinks so git root comparisons agree (e.g. macOS /var → /private/var).
	if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil {
		root = resolved
	}
	return root, nil
}

func gitRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCurrentBranch(gitRoot string) string {
	cur := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cur.Dir = gitRoot
	if out, err := cur.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" && b != "HEAD" {
			return b
		}
	}
	// Unborn HEAD (fresh git init, no commits yet).
	sym := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	sym.Dir = gitRoot
	if out, err := sym.Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// ensureGitRepo locates the project git root. If git is missing, returns code
// git_missing. If the directory is not a repo, runs git init there.
func (m *ProjectManager) ensureGitRepo(ctx context.Context, projectID string) (gitRoot, workDir, code, msg string, err error) {
	workDir, err = m.projectWorkDir(ctx, projectID)
	if err != nil {
		return "", "", "", "", err
	}
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return "", workDir, "git_missing", "git 未安装或不在 PATH 中", nil
	}
	gitRoot, rootErr := gitRepoRoot(workDir)
	if rootErr == nil {
		return gitRoot, workDir, "", "", nil
	}
	initCmd := exec.Command("git", "init")
	initCmd.Dir = workDir
	out, initErr := initCmd.CombinedOutput()
	if initErr != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = initErr.Error()
		}
		return "", workDir, "init_failed", detail, nil
	}
	gitRoot, rootErr = gitRepoRoot(workDir)
	if rootErr != nil {
		return "", workDir, "init_failed", "git init 后仍无法识别仓库", nil
	}
	return gitRoot, workDir, "", "", nil
}

func (m *ProjectManager) GetGitChanges(ctx context.Context, projectID string) (*GitChanges, error) {
	gitRoot, workDir, code, msg, err := m.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return &GitChanges{Error: msg, Code: code}, nil
	}

	cmd := exec.Command("git", "status", "--porcelain", "-b")
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return &GitChanges{}, nil
	}

	prefix := ""
	if gitRoot != workDir {
		rel, relErr := filepath.Rel(gitRoot, workDir)
		if relErr == nil && rel != "." {
			prefix = rel + "/"
		}
	}

	return parseGitStatus(out, gitRoot, workDir, prefix), nil
}

func (m *ProjectManager) ListGitBranches(ctx context.Context, projectID string) (*GitBranches, error) {
	gitRoot, _, code, msg, err := m.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return &GitBranches{Error: msg, Code: code}, nil
	}

	result := &GitBranches{}

	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		result.Current = gitCurrentBranch(gitRoot)
		return result, nil
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if b := strings.TrimSpace(line); b != "" {
			result.Branches = append(result.Branches, b)
		}
	}

	result.Current = gitCurrentBranch(gitRoot)
	if result.Current != "" {
		found := false
		for _, b := range result.Branches {
			if b == result.Current {
				found = true
				break
			}
		}
		if !found {
			result.Branches = append([]string{result.Current}, result.Branches...)
		}
	}

	return result, nil
}

// MaxGitDiffBytes caps patch payload returned to clients.
const MaxGitDiffBytes = 512 * 1024

type GitDiff struct {
	Path      string `json:"path"`
	Staged    bool   `json:"staged"`
	Patch     string `json:"patch"`
	Truncated bool   `json:"truncated,omitempty"`
	Binary    bool   `json:"binary,omitempty"`
	Untracked bool   `json:"untracked,omitempty"`
	Error     string `json:"error,omitempty"`
	Code      string `json:"code,omitempty"` // git_missing | init_failed | not_found
}

// GetGitDiff returns a unified diff for a project-relative path.
// staged=true → git diff --cached; otherwise working-tree vs index (or /dev/null for untracked).
func (m *ProjectManager) GetGitDiff(ctx context.Context, projectID, subPath string, staged bool) (*GitDiff, error) {
	subPath = strings.TrimSpace(strings.ReplaceAll(subPath, "\\", "/"))
	if subPath == "" || strings.Contains(subPath, "..") {
		return nil, fmt.Errorf("invalid path")
	}

	gitRoot, workDir, code, msg, err := m.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return &GitDiff{Path: subPath, Staged: staged, Error: msg, Code: code}, nil
	}

	// Resolve project-relative path to absolute and ensure it stays under workDir.
	absFile := filepath.Clean(filepath.Join(workDir, filepath.FromSlash(subPath)))
	if absFile != workDir && !strings.HasPrefix(absFile, workDir+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path escapes project directory")
	}

	gitPath := subPath
	if gitRoot != workDir {
		rel, relErr := filepath.Rel(gitRoot, absFile)
		if relErr != nil {
			return nil, fmt.Errorf("invalid path")
		}
		gitPath = filepath.ToSlash(rel)
	}

	result := &GitDiff{Path: subPath, Staged: staged}

	// Untracked: synthesize a diff from file contents (unstaged only).
	if !staged {
		st := exec.Command("git", "status", "--porcelain", "--", gitPath)
		st.Dir = gitRoot
		if out, err := st.Output(); err == nil {
			line := strings.TrimSpace(string(out))
			if strings.HasPrefix(line, "??") {
				result.Untracked = true
				data, readErr := os.ReadFile(absFile)
				if readErr != nil {
					if os.IsNotExist(readErr) {
						result.Code = "not_found"
						result.Error = "file not found"
						return result, nil
					}
					return nil, readErr
				}
				if isLikelyBinary(data) {
					result.Binary = true
					result.Patch = "Binary file (untracked)\n"
					return result, nil
				}
				result.Patch = synthesizeAddPatch(gitPath, string(data))
				result.Patch, result.Truncated = truncatePatch(result.Patch, MaxGitDiffBytes)
				return result, nil
			}
		}
	}

	args := []string{"diff", "--no-color", "--find-renames"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", gitPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = gitRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		result.Error = detail
		return result, nil
	}

	patch := string(out)
	if patch == "" {
		// Deleted staged/unstaged may still produce empty if path wrong; try once without path filter hints.
		result.Patch = ""
		return result, nil
	}
	if strings.Contains(patch, "Binary files ") || strings.Contains(patch, "GIT binary patch") {
		result.Binary = true
	}
	result.Patch, result.Truncated = truncatePatch(patch, MaxGitDiffBytes)
	return result, nil
}

func synthesizeAddPatch(path, content string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + path + " b/" + path + "\n")
	b.WriteString("new file mode 100644\n")
	b.WriteString("--- /dev/null\n")
	b.WriteString("+++ b/" + path + "\n")
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	// Drop trailing empty from Split on final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	b.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))
	for _, line := range lines {
		b.WriteByte('+')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func truncatePatch(patch string, max int) (string, bool) {
	if max <= 0 || len(patch) <= max {
		return patch, false
	}
	cut := patch[:max]
	// Prefer cutting at a line boundary.
	if i := strings.LastIndexByte(cut, '\n'); i > max/2 {
		cut = cut[:i+1]
	}
	return cut + "\n... (diff truncated)\n", true
}

func isLikelyBinary(data []byte) bool {
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func (m *ProjectManager) CheckoutGitBranch(ctx context.Context, projectID, branch string) (*GitBranches, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, fmt.Errorf("branch required")
	}
	if strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, " \t\r\n~^:?*[\\") {
		return nil, fmt.Errorf("invalid branch name")
	}
	gitRoot, _, code, msg, err := m.ensureGitRepo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return nil, fmt.Errorf("%s", msg)
	}

	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = gitRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return m.ListGitBranches(ctx, projectID)
}

func parseGitStatus(output []byte, gitRoot, projectRoot, prefix string) *GitChanges {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	result := &GitChanges{}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			branchInfo := strings.TrimPrefix(line, "## ")
			result.Branch = strings.Split(branchInfo, "...")[0]
			if idx := strings.Index(branchInfo, "..."); idx >= 0 {
				result.HasRemote = true
				tail := branchInfo[idx+3:]
				if i := strings.Index(tail, "["); i >= 0 && strings.HasSuffix(tail, "]") && len(tail) >= i+2 {
					result.Ahead, result.Behind = parseAheadBehind(tail[i+1 : len(tail)-1])
				}
			}
			continue
		}
		if len(line) < 3 {
			continue
		}

		statusX := line[0:1]
		statusY := line[1:2]
		rest := strings.TrimSpace(line[2:])

		stagedStatus := string(statusX)
		unstagedStatus := string(statusY)

		parseChange := func(status string, staged bool) *GitFileChange {
			var file, origFile string
			if status == "R" || status == "C" {
				parts := strings.SplitN(rest, " -> ", 2)
				if len(parts) == 2 {
					origFile = parts[0]
					file = parts[1]
				} else {
					file = rest
				}
			} else {
				file = rest
			}
			return &GitFileChange{
				Status:   status,
				File:     file,
				OrigFile: origFile,
				Staged:   staged,
			}
		}

		if stagedStatus != " " {
			// Untracked `??` is never staged; porcelain uses '?' in both columns.
			if !(stagedStatus == "?" && unstagedStatus == "?") {
				change := parseChange(stagedStatus, true)
				if changeInRoot(change.File, gitRoot, projectRoot) {
					if prefix != "" {
						change.File = strings.TrimPrefix(change.File, prefix)
						if change.OrigFile != "" {
							change.OrigFile = strings.TrimPrefix(change.OrigFile, prefix)
						}
					}
					result.Changes = append(result.Changes, change)
				}
			}
		}

		if unstagedStatus != " " && (unstagedStatus != stagedStatus || (stagedStatus == "?" && unstagedStatus == "?")) {
			status := unstagedStatus
			if stagedStatus == "?" && unstagedStatus == "?" {
				status = "??"
			}
			change := parseChange(status, false)
			if changeInRoot(change.File, gitRoot, projectRoot) {
				if prefix != "" {
					change.File = strings.TrimPrefix(change.File, prefix)
					if change.OrigFile != "" {
						change.OrigFile = strings.TrimPrefix(change.OrigFile, prefix)
					}
				}
				result.Changes = append(result.Changes, change)
			}
		}
	}

	return result
}

func changeInRoot(file, gitRoot, projectRoot string) bool {
	abs := filepath.Join(gitRoot, file)
	abs = filepath.Clean(abs)
	return strings.HasPrefix(abs, projectRoot) && (abs == projectRoot || strings.HasPrefix(abs, projectRoot+string(filepath.Separator)))
}

// parseAheadBehind extracts ahead/behind counts from "ahead 2, behind 1".
func parseAheadBehind(s string) (ahead, behind int) {
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) != 2 {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "ahead":
			ahead = n
		case "behind":
			behind = n
		}
	}
	return ahead, behind
}

package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	CodeGraphServerID = "codegraph"
	codeGraphBinName  = "codegraph"
)

// CodeGraphIndexState is the per-workdir index lifecycle for the builtin expert.
type CodeGraphIndexState string

const (
	CodeGraphIdle     CodeGraphIndexState = "idle"
	CodeGraphIndexing CodeGraphIndexState = "indexing"
	CodeGraphReady    CodeGraphIndexState = "ready"
	CodeGraphFailed   CodeGraphIndexState = "failed"
)

type codeGraphJob struct {
	state CodeGraphIndexState
	err   string
}

var (
	codeGraphMu        sync.Mutex
	codeGraphJobs      = map[string]*codeGraphJob{}
	codeGraphExtractMu sync.Mutex
)

var codeGraphHomeBinDir = userHomeDanmoBin

// ResolveCodeGraphBin returns the path to the bundled/local codegraph CLI
// (sunerpy/codegraph-rust). If only a compressed archive is present, it is
// extracted into ~/.danmo-work/bin on first resolve.
// Order: WORK_CODEGRAPH_BIN → home bin (extract if needed) → executable-dir sibling → PATH.
func ResolveCodeGraphBin() string {
	if p := strings.TrimSpace(os.Getenv("WORK_CODEGRAPH_BIN")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if bin, err := ensureCodeGraphBinExtracted(); err == nil && bin != "" {
		return bin
	}
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), codeGraphExecutableName())
		if st, err := os.Stat(sibling); err == nil && !st.IsDir() {
			return sibling
		}
	}
	if p, err := exec.LookPath(codeGraphBinName); err == nil {
		return p
	}
	return ""
}

func codeGraphExecutableName() string {
	if runtime.GOOS == "windows" {
		return codeGraphBinName + ".exe"
	}
	return codeGraphBinName
}

func codeGraphArchiveName() string {
	if runtime.GOOS == "windows" {
		return "codegraph.zip"
	}
	return "codegraph.tar.gz"
}

func userHomeDanmoBin() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return filepath.Join(".", ".danmo-work", "bin")
	}
	return filepath.Join(h, ".danmo-work", "bin")
}

func codeGraphVersionFile(dir string) string {
	return filepath.Join(dir, "CODEGRAPH_VERSION")
}

func readCodeGraphVersion(dir string) string {
	b, err := os.ReadFile(codeGraphVersionFile(dir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func codeGraphBinUsable(binPath string) bool {
	st, err := os.Stat(binPath)
	if err != nil || st.IsDir() {
		return false
	}
	// Reject leftover npm/shell wrappers from the old Colby bundle.
	if runtime.GOOS != "windows" {
		f, err := os.Open(binPath)
		if err != nil {
			return false
		}
		var hdr [2]byte
		_, _ = f.Read(hdr[:])
		_ = f.Close()
		if string(hdr[:]) == "#!" {
			return false
		}
	}
	return true
}

// findCodeGraphArchive returns a path to codegraph.tar.gz / codegraph.zip.
func findCodeGraphArchive() string {
	name := codeGraphArchiveName()
	candidates := []string{
		filepath.Join(codeGraphHomeBinDir(), name),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, name),
			filepath.Join(dir, "codegraph", name),
			filepath.Join(dir, "resources", "codegraph", name),
		)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return p
		}
	}
	return ""
}

// ensureCodeGraphBinExtracted returns ~/.danmo-work/bin/codegraph, extracting
// from a local archive when the unpacked binary is missing or is a stale wrapper.
func ensureCodeGraphBinExtracted() (string, error) {
	codeGraphExtractMu.Lock()
	defer codeGraphExtractMu.Unlock()

	home := codeGraphHomeBinDir()
	dest := filepath.Join(home, codeGraphExecutableName())
	if codeGraphBinUsable(dest) {
		return dest, nil
	}

	archive := findCodeGraphArchive()
	if archive == "" {
		if st, err := os.Stat(dest); err == nil && !st.IsDir() {
			return dest, nil
		}
		return "", fmt.Errorf("codegraph archive not found")
	}

	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", err
	}
	if err := extractCodeGraphArchive(archive, dest); err != nil {
		return "", err
	}
	// Keep VERSION next to the binary when we extracted from home archive.
	if filepath.Dir(archive) == home {
		// already there
	} else if ver := readCodeGraphVersion(filepath.Dir(archive)); ver != "" {
		_ = os.WriteFile(codeGraphVersionFile(home), []byte(ver+"\n"), 0o644)
	}
	log.Printf("[codegraph] extracted %s → %s", archive, dest)
	return dest, nil
}

func extractCodeGraphArchive(archive, destBin string) error {
	tmpDir, err := os.MkdirTemp("", "codegraph-extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var found string
	switch {
	case strings.HasSuffix(strings.ToLower(archive), ".zip"):
		found, err = extractCodeGraphZip(archive, tmpDir)
	default:
		found, err = extractCodeGraphTarGz(archive, tmpDir)
	}
	if err != nil {
		return err
	}
	if found == "" {
		return fmt.Errorf("codegraph binary not found in %s", archive)
	}

	tmpDest := destBin + ".tmp"
	_ = os.Remove(tmpDest)
	in, err := os.Open(found)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDest)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpDest)
		return err
	}
	if err := os.Rename(tmpDest, destBin); err != nil {
		_ = os.Remove(tmpDest)
		return err
	}
	return nil
}

func extractCodeGraphTarGz(archive, outDir string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var found string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != "codegraph" && base != "codegraph.exe" {
			continue
		}
		dest := filepath.Join(outDir, base)
		if err := writeRegularFile(dest, tr, hdr.FileInfo().Mode()); err != nil {
			return "", err
		}
		found = dest
		// Prefer the first match; archives usually have one binary.
		break
	}
	return found, nil
}

func extractCodeGraphZip(archive, outDir string) (string, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	var found string
	for _, zf := range zr.File {
		base := filepath.Base(zf.Name)
		if base != "codegraph" && base != "codegraph.exe" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		dest := filepath.Join(outDir, base)
		err = writeRegularFile(dest, rc, 0o755)
		_ = rc.Close()
		if err != nil {
			return "", err
		}
		found = dest
		break
	}
	return found, nil
}

func writeRegularFile(dest string, r io.Reader, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o755
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

// CodeGraphIndexDir is <workDir>/.codegraph.
func CodeGraphIndexDir(workDir string) string {
	return filepath.Join(filepath.Clean(workDir), ".codegraph")
}

func codeGraphIndexReady(workDir string) bool {
	st, err := os.Stat(CodeGraphIndexDir(workDir))
	return err == nil && st.IsDir()
}

// CodeGraphIndexStatus returns the known state for workDir without starting init.
func CodeGraphIndexStatus(workDir string) CodeGraphIndexState {
	workDir = filepath.Clean(workDir)
	if workDir == "" || workDir == "." {
		return CodeGraphFailed
	}
	if codeGraphIndexReady(workDir) {
		return CodeGraphReady
	}
	codeGraphMu.Lock()
	defer codeGraphMu.Unlock()
	if job, ok := codeGraphJobs[workDir]; ok {
		return job.state
	}
	return CodeGraphIdle
}

// EnsureCodeGraphIndex starts async `codegraph init` on first use when needed.
// Never blocks on init completion. Concurrent callers for the same path single-flight.
func EnsureCodeGraphIndex(workDir string) CodeGraphIndexState {
	workDir = filepath.Clean(workDir)
	if workDir == "" || workDir == "." {
		return CodeGraphFailed
	}
	if codeGraphIndexReady(workDir) {
		return CodeGraphReady
	}
	bin := ResolveCodeGraphBin()
	if bin == "" {
		codeGraphMu.Lock()
		codeGraphJobs[workDir] = &codeGraphJob{state: CodeGraphFailed, err: "codegraph binary not found"}
		codeGraphMu.Unlock()
		return CodeGraphFailed
	}

	codeGraphMu.Lock()
	if job, ok := codeGraphJobs[workDir]; ok {
		switch job.state {
		case CodeGraphIndexing, CodeGraphFailed:
			st := job.state
			codeGraphMu.Unlock()
			return st
		case CodeGraphReady:
			// Stale in-memory ready without on-disk index — restart init below.
		}
	}
	codeGraphJobs[workDir] = &codeGraphJob{state: CodeGraphIndexing}
	codeGraphMu.Unlock()

	go runCodeGraphInit(bin, workDir)
	return CodeGraphIndexing
}

func runCodeGraphInit(bin, workDir string) {
	ctxTimeout := 30 * time.Minute
	done := make(chan error, 1)
	go func() {
		cmd := exec.Command(bin, "init", workDir)
		cmd.Dir = workDir
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			done <- fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
			return
		}
		done <- nil
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(ctxTimeout):
		err = fmt.Errorf("codegraph init timed out after %s", ctxTimeout)
	}

	codeGraphMu.Lock()
	defer codeGraphMu.Unlock()
	job := codeGraphJobs[workDir]
	if job == nil {
		job = &codeGraphJob{}
		codeGraphJobs[workDir] = job
	}
	if err != nil {
		job.state = CodeGraphFailed
		job.err = err.Error()
		log.Printf("[codegraph] init failed for %s: %v", workDir, err)
		return
	}
	if codeGraphIndexReady(workDir) {
		job.state = CodeGraphReady
		job.err = ""
		log.Printf("[codegraph] init ready for %s", workDir)
		return
	}
	job.state = CodeGraphFailed
	job.err = "init finished but .codegraph missing"
	log.Printf("[codegraph] init incomplete for %s: %s", workDir, job.err)
}

// CodeGraphIndexHint returns a short prefix for the subagent goal.
func CodeGraphIndexHint(state CodeGraphIndexState, workDir string) string {
	switch state {
	case CodeGraphReady:
		return fmt.Sprintf("[codegraph-index: ready] projectPath=%s — prefer mcp_codegraph_* tools.", workDir)
	case CodeGraphIndexing:
		return fmt.Sprintf("[codegraph-index: indexing] projectPath=%s — index is building; do NOT wait; degrade to read_file/grep this turn.", workDir)
	case CodeGraphFailed:
		return fmt.Sprintf("[codegraph-index: failed] projectPath=%s — no usable index; degrade to read_file/grep.", workDir)
	default:
		return fmt.Sprintf("[codegraph-index: idle] projectPath=%s — check status; degrade if not ready.", workDir)
	}
}

// CodeGraphMCPEnv is the env block for the builtin stdio connector.
// sunerpy/codegraph-rust lists only a default subset in tools/list unless
// CODEGRAPH_MCP_TOOLS is set; keep explore/impact/callers/status (+ search/node).
func CodeGraphMCPEnv() string {
	return "CODEGRAPH_MCP_TOOLS=explore,impact,callers,status,search,node"
}

package service

import (
	"fmt"
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
	codeGraphMu   sync.Mutex
	codeGraphJobs = map[string]*codeGraphJob{}
)

var codeGraphHomeBinDir = userHomeDanmoBin

// ResolveCodeGraphBin returns the path to the bundled/local codegraph CLI.
// Order: WORK_CODEGRAPH_BIN → ~/.danmo-work/bin/codegraph → executable-dir sibling → PATH.
func ResolveCodeGraphBin() string {
	if p := strings.TrimSpace(os.Getenv("WORK_CODEGRAPH_BIN")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	homeBin := filepath.Join(codeGraphHomeBinDir(), codeGraphExecutableName())
	if st, err := os.Stat(homeBin); err == nil && !st.IsDir() {
		return homeBin
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

func userHomeDanmoBin() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return filepath.Join(".", ".danmo-work", "bin")
	}
	return filepath.Join(h, ".danmo-work", "bin")
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
func CodeGraphMCPEnv() string {
	return "CODEGRAPH_MCP_TOOLS=explore,impact,callers,status"
}

package builtin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ResolveRipgrepBin returns the path to a usable ripgrep binary, or "".
// Order: WORK_RIPGREP_BIN → ~/.danmo-work/bin/rg → executable siblings
// (dev tree, bundled desktop resources) → PATH.
func ResolveRipgrepBin() string {
	if p := strings.TrimSpace(os.Getenv("WORK_RIPGREP_BIN")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	name := "rg"
	if runtime.GOOS == "windows" {
		name = "rg.exe"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		p := filepath.Join(home, ".danmo-work", "bin", name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, p := range []string{
			filepath.Join(dir, name),
			filepath.Join(dir, "rg", name),
			filepath.Join(dir, "resources", "rg", name),
			// Tauri resource layout next to the sidecar / app exe.
			filepath.Join(dir, "..", "resources", "rg", name),
		} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	if p, err := exec.LookPath("rg"); err == nil {
		return p
	}
	return ""
}

// rgJSONEntry is one --json line emitted by ripgrep.
type rgJSONEntry struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
	} `json:"data"`
}

type grepOpts struct {
	pattern         string
	root            string
	include         string
	contextLines    int
	caseInsensitive bool
	maxResults      int
	respectIgnore   bool
}

// runRipgrep executes rg --json and returns matches (same shape as the Go
// fallback) plus the total match count. The returned error is non-nil only
// for execution failures (invalid regex, spawn failure); no matches is not
// an error. The caller falls back to the Go implementation on error.
func runRipgrep(ctx context.Context, bin string, opts grepOpts) ([]grepMatch, int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}

	args := []string{"--json", "--no-heading", "--no-messages", "-e", opts.pattern}
	for _, excl := range defaultExcludeDirs {
		args = append(args, "--glob", "!**/"+excl+"/**")
	}
	if opts.include != "" {
		args = append(args, "--glob", opts.include)
	}
	if opts.contextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", opts.contextLines))
	}
	if opts.caseInsensitive {
		args = append(args, "-i")
	}
	// Fallback walks hidden files; keep that parity (rg skips them by default).
	args = append(args, "--hidden")
	if !opts.respectIgnore {
		args = append(args, "--no-ignore")
	}
	args = append(args, opts.root)

	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}

	var results []grepMatch
	count := 0
	var lastIdx = -1        // index in results of the most recent match in this file
	var pendingCtx []string // context lines emitted before the next match
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if count >= opts.maxResults {
			_ = cmd.Process.Kill()
			break
		}
		var e rgJSONEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		switch e.Type {
		case "begin":
			lastIdx = -1
			pendingCtx = nil
		case "match":
			if e.Data.Lines.Text == "" {
				// Binary or encoding-skipped content has no inline text.
				continue
			}
			results = append(results, grepMatch{
				File:    e.Data.Path.Text,
				Line:    e.Data.LineNumber,
				Content: strings.TrimRight(e.Data.Lines.Text, "\r\n"),
				Context: pendingCtx,
			})
			pendingCtx = nil
			lastIdx = len(results) - 1
			count++
		case "context":
			if opts.contextLines <= 0 {
				continue
			}
			line := strings.TrimRight(e.Data.Lines.Text, "\r\n")
			prefix := "  "
			if strings.TrimSpace(line) != "" {
				prefix = "│ "
			}
			if lastIdx >= 0 {
				results[lastIdx].Context = append(results[lastIdx].Context, prefix+line)
			} else {
				pendingCtx = append(pendingCtx, prefix+line)
			}
		}
	}
	// Drain so rg does not block on a full pipe before Wait.
	_, _ = io.Copy(io.Discard, stdout)

	waitErr := cmd.Wait()
	if count >= opts.maxResults {
		// We killed rg on purpose once the cap was reached.
		waitErr = nil
	}
	if waitErr != nil {
		// Invalid regex (exit 2) or execution failure → Go fallback runs.
		return nil, 0, waitErr
	}
	return results, count, nil
}

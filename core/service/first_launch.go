package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"danmo-work/core/paths"
)

const (
	firstLaunchDirName    = "first_launch"
	firstLaunchStampName  = "stamp"
	firstLaunchScriptSH   = "post-install.sh"
	firstLaunchScriptPS1  = "post-install.ps1"
	envFirstLaunchScript  = "WORK_FIRST_LAUNCH_SCRIPT"
	envFirstLaunchDisable = "WORK_FIRST_LAUNCH_DISABLE"
)

var (
	firstLaunchOnce sync.Once
	// firstLaunchHomeDir is overridable in tests (defaults to WORK_HOME / ~/.danmo-work).
	firstLaunchHomeDir = func() string {
		return paths.Home()
	}
)

// StartFirstLaunchAsync runs the platform post-install script once in the
// background when the staged script content differs from the last stamp.
// Safe to call multiple times; only the first call schedules work.
// onDone is invoked after the script finishes (success or skip); may be nil.
func StartFirstLaunchAsync(onDone func()) {
	if strings.TrimSpace(os.Getenv(envFirstLaunchDisable)) != "" {
		return
	}
	firstLaunchOnce.Do(func() {
		go func() {
			defer func() {
				if onDone != nil {
					onDone()
				}
			}()
			if err := runFirstLaunch(); err != nil {
				log.Printf("[first-launch] %v", err)
			}
		}()
	})
}

func runFirstLaunch() error {
	script, err := resolveFirstLaunchScript()
	if err != nil || script == "" {
		if err != nil {
			return err
		}
		log.Printf("[first-launch] no post-install script staged — skip")
		return nil
	}
	sum, err := fileSHA256(script)
	if err != nil {
		return fmt.Errorf("hash script: %w", err)
	}
	if ver := readCodeGraphVersion(filepath.Join(firstLaunchHomeDir(), "bin")); ver != "" {
		sum = sum + ":" + ver
	}
	stampPath := filepath.Join(firstLaunchHomeDir(), firstLaunchDirName, firstLaunchStampName)
	if prev, _ := os.ReadFile(stampPath); strings.TrimSpace(string(prev)) == sum {
		log.Printf("[first-launch] stamp matches — skip (%s)", filepath.Base(script))
		return nil
	}

	log.Printf("[first-launch] running %s", script)
	if err := execFirstLaunchScript(script); err != nil {
		return fmt.Errorf("run %s: %w", script, err)
	}
	if err := os.MkdirAll(filepath.Dir(stampPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(stampPath, []byte(sum+"\n"), 0o644); err != nil {
		return err
	}
	log.Printf("[first-launch] completed (%s)", filepath.Base(script))
	return nil
}

func resolveFirstLaunchScript() (string, error) {
	if p := strings.TrimSpace(os.Getenv(envFirstLaunchScript)); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("%s=%q not found", envFirstLaunchScript, p)
	}
	want := firstLaunchScriptSH
	if runtime.GOOS == "windows" {
		want = firstLaunchScriptPS1
	}
	var candidates []string
	homeFL := filepath.Join(firstLaunchHomeDir(), firstLaunchDirName)
	candidates = append(candidates, filepath.Join(homeFL, want))
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, firstLaunchDirName, want),
			filepath.Join(dir, "resources", firstLaunchDirName, want),
			filepath.Join(dir, "..", "Resources", firstLaunchDirName, want), // macOS .app
			filepath.Join(dir, "..", "resources", firstLaunchDirName, want),
		)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", nil
}

func execFirstLaunchScript(script string) error {
	home := firstLaunchHomeDir()
	env := append(os.Environ(),
		"DANMO_HOME="+home,
		"WORK_HOME="+home,
	)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" || strings.HasSuffix(strings.ToLower(script), ".ps1") {
		cmd = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	} else {
		cmd = exec.Command("/bin/bash", script)
	}
	cmd.Dir = home
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		log.Printf("[first-launch] output:\n%s", strings.TrimSpace(string(out)))
	}
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ResetFirstLaunchForTest clears the once-guard (tests only).
func ResetFirstLaunchForTest() {
	firstLaunchOnce = sync.Once{}
}

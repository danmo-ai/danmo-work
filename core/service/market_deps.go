package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/paths"
)

const (
	connectorDepsDirName = "deps"
	// connectorDepsTimeout covers heavy installs (e.g. playwright browsers).
	connectorDepsTimeout = 30 * time.Minute
)

// marketDepsHome is overridable in tests (defaults to ~/.danmo-work).
var marketDepsHome = paths.Home

// ResolveConnectorDepsScript picks the host platform deps script for a connector package.
//
// Rules:
//   - If catalog deps map has an entry for this platform → use that relative path.
//   - Else if package has deps/ → require deps/darwin.sh | deps/linux.sh | deps/windows.ps1.
//   - If deps/ exists but the platform script is missing → error.
//   - If no deps map and no deps/ directory → skip (ok=false).
func ResolveConnectorDepsScript(packageDir string, catalogDeps map[string]string) (rel string, abs string, ok bool, err error) {
	platform := connectorDepsPlatform()
	packageDir = filepath.Clean(packageDir)

	if catalogDeps != nil {
		if p, hit := catalogDeps[platform]; hit {
			p = strings.TrimSpace(p)
			if p == "" {
				return "", "", false, fmt.Errorf("deps[%s] is empty", platform)
			}
			abs, err := resolvePackageRelativeScript(packageDir, p)
			if err != nil {
				return "", "", false, err
			}
			return filepath.ToSlash(p), abs, true, nil
		}
		// Catalog listed other platforms but not this one → fail.
		if len(catalogDeps) > 0 {
			return "", "", false, fmt.Errorf("connector deps: no script for platform %q (have %v)", platform, depsPlatformKeys(catalogDeps))
		}
	}

	depsRoot := filepath.Join(packageDir, connectorDepsDirName)
	st, serr := os.Stat(depsRoot)
	if serr != nil {
		if os.IsNotExist(serr) {
			return "", "", false, nil
		}
		return "", "", false, serr
	}
	if !st.IsDir() {
		return "", "", false, fmt.Errorf("%s is not a directory", connectorDepsDirName)
	}

	rel = defaultDepsRelPath(platform)
	abs = filepath.Join(packageDir, filepath.FromSlash(rel))
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return "", "", false, fmt.Errorf("connector deps: %s missing for platform %q", rel, platform)
		}
		return "", "", false, err
	}
	return rel, abs, true, nil
}

func defaultDepsRelPath(platform string) string {
	if platform == "windows" {
		return connectorDepsDirName + "/windows.ps1"
	}
	return connectorDepsDirName + "/" + platform + ".sh"
}

func defaultUninstallDepsRelPath(platform string) string {
	if platform == "windows" {
		return connectorDepsDirName + "/uninstall-windows.ps1"
	}
	return connectorDepsDirName + "/uninstall-" + platform + ".sh"
}

func connectorDepsPlatform() string {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return runtime.GOOS
	default:
		return runtime.GOOS
	}
}

func depsPlatformKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func resolvePackageRelativeScript(packageDir, rel string) (string, error) {
	rel = filepath.Clean(filepath.FromSlash(rel))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("deps script path %q escapes package", rel)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("deps script path must be package-relative, got %q", rel)
	}
	abs := filepath.Join(packageDir, rel)
	// Ensure abs is still under packageDir.
	relCheck, err := filepath.Rel(packageDir, abs)
	if err != nil || strings.HasPrefix(relCheck, "..") {
		return "", fmt.Errorf("deps script path %q escapes package", rel)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("deps script %q: %w", rel, err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("deps script %q is a directory", rel)
	}
	return abs, nil
}

// ResolveConnectorUninstallScript picks an optional cleanup script.
// Missing uninstall script is a skip (ok=false), not an error — unless catalog
// explicitly lists uninstallDeps for other platforms but not this one.
func ResolveConnectorUninstallScript(packageDir string, catalogUninstall map[string]string) (rel string, abs string, ok bool, err error) {
	platform := connectorDepsPlatform()
	packageDir = filepath.Clean(packageDir)

	if catalogUninstall != nil {
		if p, hit := catalogUninstall[platform]; hit {
			p = strings.TrimSpace(p)
			if p == "" {
				return "", "", false, fmt.Errorf("uninstallDeps[%s] is empty", platform)
			}
			abs, err := resolvePackageRelativeScript(packageDir, p)
			if err != nil {
				return "", "", false, err
			}
			return filepath.ToSlash(p), abs, true, nil
		}
		if len(catalogUninstall) > 0 {
			return "", "", false, fmt.Errorf("connector uninstallDeps: no script for platform %q (have %v)", platform, depsPlatformKeys(catalogUninstall))
		}
	}

	rel = defaultUninstallDepsRelPath(platform)
	abs = filepath.Join(packageDir, filepath.FromSlash(rel))
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return rel, abs, true, nil
}

// RunConnectorDepsScript executes a package-relative deps script.
// cwd is the connector package directory. Env includes DANMO_HOME, CONNECTOR_ID, GOARCH.
func RunConnectorDepsScript(ctx context.Context, packageDir, scriptAbs, connectorID string) (logOut string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, connectorDepsTimeout)
	defer cancel()

	home := marketDepsHome()
	env := append(os.Environ(),
		"DANMO_HOME="+home,
		"WORK_HOME="+home,
		"CONNECTOR_ID="+connectorID,
		"GOARCH="+runtime.GOARCH,
		"GOOS="+runtime.GOOS,
	)

	var cmd *exec.Cmd
	lower := strings.ToLower(scriptAbs)
	if runtime.GOOS == "windows" || strings.HasSuffix(lower, ".ps1") {
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptAbs)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/bash", scriptAbs)
	}
	cmd.Dir = packageDir
	cmd.Env = env
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()
	logOut = strings.TrimSpace(buf.String())
	if ctx.Err() == context.DeadlineExceeded {
		return logOut, fmt.Errorf("deps script timed out after %s", connectorDepsTimeout)
	}
	if runErr != nil {
		if logOut != "" {
			return logOut, fmt.Errorf("deps script failed: %w\n%s", runErr, logOut)
		}
		return logOut, fmt.Errorf("deps script failed: %w", runErr)
	}
	return logOut, nil
}

// RunConnectorDepsForPackage resolves and runs install deps for a connector package if present.
func RunConnectorDepsForPackage(ctx context.Context, packageDir string, item domain.MarketItem) (scriptRel, logOut string, err error) {
	rel, abs, ok, rerr := ResolveConnectorDepsScript(packageDir, item.Deps)
	if rerr != nil {
		return "", "", rerr
	}
	if !ok {
		return "", "", nil
	}
	logOut, err = RunConnectorDepsScript(ctx, packageDir, abs, item.ID)
	return rel, logOut, err
}

// RunConnectorUninstallForPackage resolves and runs optional uninstall deps.
func RunConnectorUninstallForPackage(ctx context.Context, packageDir string, item domain.MarketItem) (scriptRel, logOut string, err error) {
	rel, abs, ok, rerr := ResolveConnectorUninstallScript(packageDir, item.UninstallDeps)
	if rerr != nil {
		return "", "", rerr
	}
	if !ok {
		return "", "", nil
	}
	logOut, err = RunConnectorDepsScript(ctx, packageDir, abs, item.ID)
	return rel, logOut, err
}

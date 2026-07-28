package container

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"danmo-work/core/paths"
)

const (
	defaultGitHubRepo = "danmo-ai/danmo-work"
	envDirName        = "env"
)

// EnvDir is ~/.danmo-work/env (created if needed).
func EnvDir() string {
	dir := filepath.Join(paths.Home(), envDirName)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// LinuxArch returns amd64/arm64 for the env tar asset name.
func LinuxArch() string {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return runtime.GOARCH
	default:
		return runtime.GOARCH
	}
}

// AssetFileName is danmo-work-env-linux-<arch>.tar
func AssetFileName(arch string) string {
	if arch == "" {
		arch = LinuxArch()
	}
	return "danmo-work-env-linux-" + arch + ".tar"
}

// InstallPath is the canonical download destination under ~/.danmo-work/env/.
func InstallPath(arch string) string {
	return filepath.Join(EnvDir(), AssetFileName(arch))
}

// GitHubRepo returns owner/name (WORK_GITHUB_REPO or danmo-ai/danmo-work).
func GitHubRepo() string {
	if r := strings.TrimSpace(os.Getenv("WORK_GITHUB_REPO")); r != "" {
		return r
	}
	return defaultGitHubRepo
}

// ReleaseDownloadURL builds the GitHub Releases asset URL for the env tar.
// version is the app version (e.g. 0.9.2 or v0.9.2); "dev"/empty uses latest.
func ReleaseDownloadURL(version, arch string) string {
	repo := GitHubRepo()
	name := AssetFileName(arch)
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	if v == "" || v == "dev" || strings.Contains(v, "dirty") {
		return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, name)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, v, name)
}

// TarInfo describes local install + download URL for Settings.
type TarInfo struct {
	Present     bool   `json:"present"`
	Path        string `json:"path,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	Arch        string `json:"arch"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
}

// InspectTar reports whether the env tar is installed under ~/.danmo-work/env
// (or already discoverable via ResolveTarPath).
func InspectTar(version string) TarInfo {
	arch := LinuxArch()
	info := TarInfo{
		Arch:        arch,
		DownloadURL: ReleaseDownloadURL(version, arch),
		AssetName:   AssetFileName(arch),
	}
	if p := ResolveTarPath(""); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			info.Present = true
			info.Path = p
			info.Bytes = st.Size()
			return info
		}
	}
	p := InstallPath(arch)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		info.Present = true
		info.Path = p
		info.Bytes = st.Size()
	} else {
		info.Path = p
	}
	return info
}

// DownloadEnvTar fetches the Release asset into ~/.danmo-work/env/.
func DownloadEnvTar(ctx context.Context, version string) (TarInfo, error) {
	arch := LinuxArch()
	url := ReleaseDownloadURL(version, arch)
	dest := InstallPath(arch)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return TarInfo{}, err
	}

	tmp := dest + ".partial"
	_ = os.Remove(tmp)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return TarInfo{}, err
	}
	req.Header.Set("User-Agent", "danmo-work-env-tar")
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return TarInfo{}, fmt.Errorf("download env tar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TarInfo{}, fmt.Errorf("download env tar: HTTP %d for %s", resp.StatusCode, url)
	}

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return TarInfo{}, err
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return TarInfo{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return TarInfo{}, closeErr
	}
	if n < 1024 {
		_ = os.Remove(tmp)
		return TarInfo{}, fmt.Errorf("download env tar: file too small (%d bytes)", n)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return TarInfo{}, err
	}
	// Convenience alias for ResolveTarPath.
	alias := filepath.Join(EnvDir(), "danmo-work-env.tar")
	_ = os.Remove(alias)
	_ = os.Symlink(dest, alias)
	if _, err := os.Stat(alias); err != nil {
		// Windows / no symlink: copy is fine but expensive; hardlink or skip.
		_ = copyFile(dest, alias)
	}

	info := InspectTar(version)
	info.Present = true
	info.Path = dest
	info.Bytes = n
	info.DownloadURL = url
	return info, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

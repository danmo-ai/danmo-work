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

// SupportedLinuxArches are Release env-tar architectures.
var SupportedLinuxArches = []string{"amd64", "arm64"}

// EnvDir is ~/.danmo-work/env (created if needed).
func EnvDir() string {
	dir := filepath.Join(paths.Home(), envDirName)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// LinuxArch returns amd64/arm64 for the env tar asset name (host GOARCH).
func LinuxArch() string {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return runtime.GOARCH
	default:
		return "amd64"
	}
}

// NormalizeArch maps aliases to amd64|arm64; empty → host arch.
func NormalizeArch(arch string) string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "":
		return LinuxArch()
	case "amd64", "x86_64", "x64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	default:
		return strings.ToLower(strings.TrimSpace(arch))
	}
}

// AssetFileName is danmo-work-env-linux-<arch>.tar
func AssetFileName(arch string) string {
	return "danmo-work-env-linux-" + NormalizeArch(arch) + ".tar"
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

// TarInfo describes one arch's local install + download URL for Settings.
type TarInfo struct {
	Present     bool   `json:"present"`
	Path        string `json:"path,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	Arch        string `json:"arch"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
	Recommended bool   `json:"recommended,omitempty"`
}

// InspectTar reports host-arch install status (and discoverable ResolveTarPath).
func InspectTar(version string) TarInfo {
	return InspectTarArch(version, LinuxArch())
}

// InspectTarArch reports install status for one linux arch.
func InspectTarArch(version, arch string) TarInfo {
	arch = NormalizeArch(arch)
	info := TarInfo{
		Arch:        arch,
		DownloadURL: ReleaseDownloadURL(version, arch),
		AssetName:   AssetFileName(arch),
		Recommended: arch == LinuxArch(),
		Path:        InstallPath(arch),
	}
	p := InstallPath(arch)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		info.Present = true
		info.Path = p
		info.Bytes = st.Size()
		return info
	}
	// Host arch: also accept generic ResolveTarPath / alias.
	if arch == LinuxArch() {
		if rp := ResolveTarPath(""); rp != "" {
			if st, err := os.Stat(rp); err == nil && !st.IsDir() {
				info.Present = true
				info.Path = rp
				info.Bytes = st.Size()
			}
		}
	}
	return info
}

// ListTarVariants returns amd64 + arm64 download options.
func ListTarVariants(version string) []TarInfo {
	out := make([]TarInfo, 0, len(SupportedLinuxArches))
	for _, a := range SupportedLinuxArches {
		out = append(out, InspectTarArch(version, a))
	}
	return out
}

// DownloadEnvTar fetches the Release asset for arch into ~/.danmo-work/env/.
// Empty arch uses the host architecture.
func DownloadEnvTar(ctx context.Context, version, arch string) (TarInfo, error) {
	arch = NormalizeArch(arch)
	switch arch {
	case "amd64", "arm64":
	default:
		return TarInfo{}, fmt.Errorf("download env tar: unsupported arch %q (amd64|arm64)", arch)
	}
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
	// Host-arch download also refreshes the generic alias used by ResolveTarPath.
	if arch == LinuxArch() {
		alias := filepath.Join(EnvDir(), "danmo-work-env.tar")
		_ = os.Remove(alias)
		if err := os.Symlink(dest, alias); err != nil {
			_ = copyFile(dest, alias)
		}
	}

	info := InspectTarArch(version, arch)
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

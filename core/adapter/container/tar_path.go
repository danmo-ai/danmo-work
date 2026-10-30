package container

import (
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
)

// ResolveTarPath finds the optional env image tar (user-downloaded Release
// asset or local make build-env-tar). Not shipped inside app packages. Order:
//  1. explicit override
//  2. WORK_ENV_TAR
//  3. ~/.danmo-work/env/ host-arch specific, then alias
//  4. $DQ_ROOT/out/env or ./out/env (dev builds only, host-arch first)
//
// Resolves the cli profile. Use ResolveTarPathProfile for desktop.
func ResolveTarPath(override string) string {
	return ResolveTarPathProfile(override, domain.SandboxProfileCLI)
}

// ResolveTarPathProfile finds the env tar for a sandbox image profile.
func ResolveTarPathProfile(override, profile string) string {
	profile = NormalizeProfile(profile)
	if p := strings.TrimSpace(override); p != "" {
		if fileExists(p) {
			return p
		}
		return ""
	}
	if p := strings.TrimSpace(os.Getenv("WORK_ENV_TAR")); p != "" && fileExists(p) && profile == domain.SandboxProfileCLI {
		return p
	}
	if profile == domain.SandboxProfileDesktop {
		if p := strings.TrimSpace(os.Getenv("WORK_ENV_DESKTOP_TAR")); p != "" && fileExists(p) {
			return p
		}
	}
	arch := LinuxArch()
	candidates := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		base := filepath.Join(home, ".danmo-work", "env")
		candidates = append(candidates, archTarNamesProfile(base, profile, arch)...)
	}
	if root := strings.TrimSpace(os.Getenv("DQ_ROOT")); root != "" {
		candidates = append(candidates, archTarNamesProfile(filepath.Join(root, "out", "env"), profile, arch)...)
	}
	candidates = append(candidates, archTarNamesProfile(filepath.Join("out", "env"), profile, arch)...)
	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func archTarNames(dir, arch string) []string {
	return archTarNamesProfile(dir, domain.SandboxProfileCLI, arch)
}

func archTarNamesProfile(dir, profile, arch string) []string {
	switch NormalizeProfile(profile) {
	case domain.SandboxProfileDesktop:
		return []string{
			filepath.Join(dir, "danmo-work-env-desktop-linux-"+arch+".tar.gz"),
			filepath.Join(dir, "danmo-work-env-desktop.tar.gz"),
			filepath.Join(dir, "danmo-work-env-desktop-linux-"+arch+".tar"),
			filepath.Join(dir, "danmo-work-env-desktop.tar"),
		}
	default:
		return []string{
			filepath.Join(dir, "danmo-work-env-linux-"+arch+".tar.gz"),
			filepath.Join(dir, "danmo-work-env.tar.gz"),
			// legacy plain-tar installs (pre-gzip assets)
			filepath.Join(dir, "danmo-work-env-linux-"+arch+".tar"),
			filepath.Join(dir, "danmo-work-env.tar"),
		}
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// DefaultImageTag returns the canonical local tag for a profile.
func DefaultImageTag(profile string) string {
	switch NormalizeProfile(profile) {
	case domain.SandboxProfileDesktop:
		return domain.EnvImageTagDesktop
	default:
		return domain.EnvImageTagCLI
	}
}

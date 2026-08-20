//go:build !linux && !darwin && !windows

package computer

func newBackend() backend {
	return stubBackend{reason: "unsupported operating system for desktop control"}
}

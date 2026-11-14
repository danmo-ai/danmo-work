//go:build linux

package computer

func newBackend() backend { return &linuxBackend{} }

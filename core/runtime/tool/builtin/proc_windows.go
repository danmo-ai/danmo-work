//go:build windows

package builtin

import "os/exec"

// setProcGroup is a no-op on Windows: exec.CommandContext already terminates
// the child process, and WaitDelay (set by the caller) unblocks output
// collection when descendants keep the pipes open.
func setProcGroup(_ *exec.Cmd) {}

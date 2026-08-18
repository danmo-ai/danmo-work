//go:build !windows

package builtin

import (
	"os/exec"
	"syscall"
)

// setProcGroup runs the command in its own process group and kills the whole
// group on context cancellation. Without this, exec.CommandContext only kills
// the direct shell child: grandchildren survive the timeout and keep the
// stdout/stderr pipes open, so CombinedOutput blocks indefinitely.
func setProcGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

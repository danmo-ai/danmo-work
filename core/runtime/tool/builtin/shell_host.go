package builtin

import (
	"context"
	"fmt"
	"time"

	"danmo-work/core/port"
	"danmo-work/core/runtime/sandbox"
)

func hostRunShell(ctx context.Context, opts port.SandboxRunOptions) ([]byte, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd, err := sandbox.HostShellCommand(ctx, opts.Command)
	if err != nil {
		return nil, err
	}
	cmd.Dir = opts.WorkDir
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	setProcGroup(cmd)
	// Even after the process (group) is killed, Wait blocks while inherited
	// pipes are held open by stray descendants; WaitDelay forces it to return.
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("command timed out after %s", timeout)
	}
	return out, err
}

package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type ExecShell struct {
	Sandbox port.Sandbox
	// Runner routes to LocalOS sandbox or per-project OCI container.
	// When nil, falls back to Sandbox (or host).
	Runner port.ExecutionBackend
}

func (h *ExecShell) Name() string                { return "exec_shell" }
func (h *ExecShell) RiskLevel() domain.RiskLevel { return domain.RiskHigh }
func (h *ExecShell) Describe(args map[string]any) string {
	cmd, _ := args["command"].(string)
	if cmd == "" {
		return "exec_shell"
	}
	if len(cmd) > 100 {
		cmd = cmd[:100] + "..."
	}
	return cmd
}
func (h *ExecShell) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "exec_shell",
		Description: "Execute a shell command and return its stdout/stderr. HIGH RISK — requires user approval.\n\n" +
			"**Important**: Commands run in the project root. Under OCI container mode the project is bind-mounted at the same absolute path as on the host, so paths match file tools (read_file/write/edit). Prefer relative paths from the project root.\n\n" +
			"- Use only for builds, tests, git operations, or commands with no tool alternative.\n" +
			"- Do NOT use for reading/writing files — use read_file, write, edit, or apply_patch instead.\n" +
			"- Do NOT use for searching file contents — use grep or glob instead.\n" +
			"- Avoid destructive commands unless the user explicitly requests them.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to execute"},
				"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (default: 30)"},
			},
			"required": []string{"command"},
		},
	}
}

func (h *ExecShell) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	cmdStr, _ := input["command"].(string)
	if cmdStr == "" {
		return domain.ToolResult{}, fmt.Errorf("command is required")
	}
	timeout := 30 * time.Second
	if t, ok := input["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	} else if t, ok := input["timeout"].(int); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}

	allowNet := boolFromInput(input, "__sandbox_allow_network")
	workDir := workDirFromInput(input)
	projectID, _ := input["__project_id"].(string)

	var out []byte
	var err error
	if h.Runner != nil {
		opts := port.ExecRunOptions{
			ProjectID:    projectID,
			Command:      cmdStr,
			WorkDir:      workDir,
			Timeout:      timeout,
			AllowNetwork: allowNet,
		}
		if h.Sandbox != nil {
			if u := strings.TrimSpace(h.Sandbox.ProxyURL()); u != "" {
				opts.AllowlistProxy = strings.TrimPrefix(u, "http://")
				opts.AllowlistProxy = strings.TrimPrefix(opts.AllowlistProxy, "https://")
			}
		}
		out, err = h.Runner.Run(ctx, opts)
	} else {
		opts := port.SandboxRunOptions{
			Command:      cmdStr,
			WorkDir:      workDir,
			Timeout:      timeout,
			AllowNetwork: allowNet,
		}
		if h.Sandbox != nil {
			out, err = h.Sandbox.Run(ctx, opts)
		} else {
			out, err = hostRunShell(ctx, opts)
		}
	}

	content := strings.TrimSpace(string(out))
	if err != nil {
		if content == "" {
			return domain.ToolResult{}, fmt.Errorf("command failed: %w", err)
		}
		return domain.ToolResult{}, fmt.Errorf("%s\ncommand failed: %w", content, err)
	}
	return domain.ToolResult{Content: content}, nil
}

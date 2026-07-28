package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
)

// PrepareMCPStdio configures env and optional bwrap net-deny wrap for MCP stdio (S2).
// Returns the executable, args, and env to use for the child process.
func (m *Manager) PrepareMCPStdio(srv domain.MCPServer, baseEnv []string) (command string, args []string, env []string) {
	command = srv.Command
	args = splitShellArgs(srv.Args)
	env, wrapDeny := m.ApplyMCPEnv(baseEnv, srv.Network)
	if !wrapDeny || !lookPath("bwrap") {
		return command, args, env
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}
	bwrapArgs := []string{
		"--die-with-parent",
		"--unshare-net",
		"--unshare-pid",
		"--unshare-ipc",
		"--proc", "/proc",
		"--dev", "/dev",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind-try", "/lib64", "/lib64",
		"--ro-bind-try", "/sbin", "/sbin",
		"--ro-bind-try", "/etc", "/etc",
		"--bind", home, home,
		"--tmpfs", "/tmp",
		"--chdir", home,
		"--",
		command,
	}
	bwrapArgs = append(bwrapArgs, args...)
	return "bwrap", bwrapArgs, env
}

func splitShellArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// MCP Args is space-separated (same as adapter/mcp splitArgs).
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// LookPathBwrap reports whether bubblewrap is available (tests / status).
func LookPathBwrap() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// AbsHome is a small helper for tests.
func AbsHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return filepath.Clean(h)
}

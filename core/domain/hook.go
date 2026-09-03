package domain

// Codex-style lifecycle hooks declared by a plugin's hooks.json (v1).
//
// v1 scope: context-injection events only, command handlers only, and only
// builtin plugins are executed (market plugin hooks are ignored + logged).
//
// Protocol (aligned with Codex hooks):
//   - stdin:  JSON {"event","agent_id","session_id","project_id","workdir","goal"}
//   - stdout: JSON {"additionalContext": "..."} — empty/missing means no injection
//   - timeout / non-zero exit / invalid output: logged and skipped, never blocks a turn.

// Context-injection hook events supported in v1.
const (
	HookEventUserPromptSubmit = "userPromptSubmit" // main-agent turn
	HookEventSubagentStart    = "subagentStart"    // delegated sub-agent turn
)

// HookHandler is one command handler inside a matcher group.
type HookHandler struct {
	Type       string `json:"type"` // only "command" in v1
	Command    string `json:"command"`
	TimeoutSec int    `json:"timeoutSec,omitempty"` // 0 → default
}

// HookMatcherGroup groups handlers under one matcher. Matcher matches the
// agent id (exact, or "a|b" alternation); empty means all agents.
type HookMatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookHandler `json:"hooks"`
}

// HooksConfig is the on-disk hooks.json format.
type HooksConfig struct {
	Version int                            `json:"version"`
	Hooks   map[string][]HookMatcherGroup  `json:"hooks"`
}

// ResolvedHook is a hook handler after plugin scan: command placeholders
// ${PLUGIN_DIR} / ${WORKDIR} are substituted at execution time.
type ResolvedHook struct {
	PluginName string
	PluginRoot string
	Event      string
	Matcher    string
	Command    string
	TimeoutSec int
}

// HookStdinPayload is sent to a command hook's stdin.
type HookStdinPayload struct {
	Event     string `json:"event"`
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	WorkDir   string `json:"workdir"`
	Goal      string `json:"goal"`
}

// HookStdoutPayload is read from a command hook's stdout.
type HookStdoutPayload struct {
	AdditionalContext string `json:"additionalContext,omitempty"`
}

const (
	// HookDefaultTimeoutSec caps one hook command's wall time.
	HookDefaultTimeoutSec = 10
	// HookMaxAdditionalContextRunes caps one hook's injected context so a
	// misbehaving hook cannot flood the prompt.
	HookMaxAdditionalContextRunes = 4000
)

// HookMatcherMatches reports whether matcher selects agentID.
// Empty matcher matches all; "a|b" matches either.
func HookMatcherMatches(matcher, agentID string) bool {
	if matcher == "" {
		return true
	}
	for _, alt := range splitAlternation(matcher) {
		if alt == agentID {
			return true
		}
	}
	return false
}

func splitAlternation(matcher string) []string {
	var out []string
	cur := ""
	for _, r := range matcher {
		if r == '|' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

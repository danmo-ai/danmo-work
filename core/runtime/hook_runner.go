package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"danmo-work/core/domain"
	"danmo-work/core/runtime/sandbox"
	"danmo-work/core/service"
)

// SetPluginHooks wires Codex-style plugin context hooks (builtin plugins only,
// resolved by PluginManager.Init). Called once at bootstrap.
func (e *Engine) SetPluginHooks(hooks []domain.ResolvedHook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pluginHooks = hooks
}

// hookContextText runs command hooks matching (event, agent.ID) and joins the
// non-empty additionalContext outputs. Call-time only, never persisted, and a
// failing hook never blocks a turn — timeouts/errors are logged and skipped.
func (e *Engine) hookContextText(ctx context.Context, agent domain.Agent, event, sessionID, projectID, workDir, goal string) string {
	e.mu.Lock()
	hooks := e.pluginHooks
	e.mu.Unlock()
	matched := service.HooksForAgent(hooks, event, agent.ID)
	if len(matched) == 0 {
		return ""
	}
	var parts []string
	for _, h := range matched {
		if txt := runContextHook(ctx, h, agent.ID, sessionID, projectID, workDir, goal); txt != "" {
			parts = append(parts, txt)
		}
	}
	return strings.Join(parts, "\n\n")
}

// runContextHook executes one command hook and returns its additionalContext,
// wrapped with provenance tags. Returns "" on any failure.
func runContextHook(ctx context.Context, h domain.ResolvedHook, agentID, sessionID, projectID, workDir, goal string) string {
	command := strings.ReplaceAll(h.Command, "${PLUGIN_DIR}", h.PluginRoot)
	command = strings.ReplaceAll(command, "${WORKDIR}", workDir)

	timeout := h.TimeoutSec
	if timeout <= 0 {
		timeout = domain.HookDefaultTimeoutSec
	}
	hctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	payload, err := json.Marshal(domain.HookStdinPayload{
		Event: h.Event, AgentID: agentID,
		SessionID: sessionID, ProjectID: projectID,
		WorkDir: workDir, Goal: goal,
	})
	if err != nil {
		return ""
	}

	cmd, err := sandbox.HostShellCommand(hctx, command)
	if err != nil {
		log.Printf("[hooks] %s/%s: shell resolve: %v", h.PluginName, h.Event, err)
		return ""
	}
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[hooks] %s/%s: command failed: %v (%s)", h.PluginName, h.Event, err, strings.TrimSpace(stderr.String()))
		return ""
	}

	var out domain.HookStdoutPayload
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		log.Printf("[hooks] %s/%s: invalid stdout JSON: %v", h.PluginName, h.Event, err)
		return ""
	}
	text := strings.TrimSpace(out.AdditionalContext)
	if text == "" {
		return ""
	}
	if utf8.RuneCountInString(text) > domain.HookMaxAdditionalContextRunes {
		runes := []rune(text)
		text = string(runes[:domain.HookMaxAdditionalContextRunes]) + "…"
	}
	return `<plugin-context source="` + h.PluginName + "/" + h.Event + `">` + "\n" + text + "\n</plugin-context>"
}

// joinEphemeral concatenates context segments, skipping empties.
func joinEphemeral(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

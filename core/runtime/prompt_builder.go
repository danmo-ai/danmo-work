package runtime

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"danmo-work/core/domain"
)

func buildSystemPrompt(agentPersona string, skillList []domain.Skill, agentList []domain.Agent, canDelegate bool, checkpoint string, activeTodos string, fileChanges string, sandboxStatus domain.SandboxStatus) string {
	var b strings.Builder
	b.WriteString(agentPersona)

	meta := buildSkillMetadata(skillList)
	if meta != "" {
		b.WriteString("\n\n")
		b.WriteString(meta)
	}
	if canDelegate {
		b.WriteString("\n\n")
		b.WriteString(buildDelegationPolicy())
		if agentMeta := buildAgentMetadata(agentList); agentMeta != "" {
			b.WriteString("\n\n")
			b.WriteString(agentMeta)
		}
	}
	if checkpoint != "" {
		b.WriteString("\n\n")
		b.WriteString("<compaction-checkpoint>\n")
		b.WriteString(checkpoint)
		b.WriteString("\n</compaction-checkpoint>")
	}
	if activeTodos != "" {
		b.WriteString("\n\n")
		b.WriteString(activeTodos)
	}
	if fileChanges != "" {
		b.WriteString("\n\n")
		b.WriteString(fileChanges)
	}
	b.WriteString("\n\n")
	b.WriteString(buildAskUserPolicy())
	b.WriteString("\n\n")
	b.WriteString(buildKnowledgePolicy())
	b.WriteString("\n\n")
	b.WriteString(buildMemoryPolicy())
	b.WriteString("\n\n")
	b.WriteString(buildTableStorePolicy())
	b.WriteString("\n\n")
	b.WriteString(buildMCPToolNamingPolicy())
	b.WriteString("\n\n")
	b.WriteString(buildRuntimeEnvironment(sandboxStatus))

	return b.String()
}

func buildAskUserPolicy() string {
	return `<ask-user-policy>
When you need clarification, a decision, missing inputs, or approval before proceeding, you MUST call the ask_user tool. Do NOT ask the user in a plain assistant message — that ends the turn without waiting for an answer.

Prefer ask_user with options (and defaultOption when one choice is recommended) or form_fields for structured input. Free-text question-only is fine when choices cannot be enumerated.

Use a normal message only to report progress, results, or blockers that do not require a reply to continue.
</ask-user-policy>`
}

func buildDelegationPolicy() string {
	return `<delegation-policy>
You can delegate via delegate_agent. Peers are listed in <available_agents>.

When to delegate:
- Split independent work across subagents; launch parallel calls when scopes do not overlap
- Prefer delegation when it reduces total effort or improves quality; act directly on small steps where coordination would add overhead

How to delegate:
- Give a clear goal plus context (known facts, constraints, expected output)
- Assign complete subtasks, not single actions — let subagents choose their tools
- Read each subagent report before the next decision; treat reports as primary evidence
- Verify load-bearing claims against EVIDENCE; if uncited, retry with a refined goal or investigate yourself
- Only ask subagents to implement after scope and approach are understood

Do NOT:
- Re-delegate the same insufficient task unchanged (refine, pick another agent, or do it yourself)
- Duplicate overlapping scopes across parallel subagents
</delegation-policy>`
}

func buildKnowledgePolicy() string {
	return `<knowledge-policy>
Use search_kb / list_kb_docs / get_kb_doc for human-curated knowledge bases bound to this agent. Relevant chapters may also be auto-injected each turn — still call tools when you need more or a different query.

When to search_kb:
- Need facts, procedures, or product/docs from the attached knowledge bases
- Auto-injected snippets are insufficient or off-topic — re-query with clearer keywords

When to list_kb_docs / get_kb_doc:
- Browse which manuals exist, then pull a full Markdown document by doc_id
- Prefer get_kb_doc after list_kb_docs or a search hit that names a document

Do NOT:
- Treat knowledge as Memory (preferences) or Table Store (business rows)
- Invent doc_ids — use ids from list_kb_docs or search results
- Dump entire knowledge bases into the reply; quote only what you need
</knowledge-policy>`
}

func buildMemoryPolicy() string {
	return `<memory-policy>
Use memory_update / memory_read for durable facts that should survive across sessions. Do not dump all memories into replies; call tools when needed.

When to memory_update:
- User states a lasting preference, correction, or "remember this"
- Stable project conventions, architecture decisions, or verified project facts
- Agent-specific working style that should persist for this agent role

Do NOT memory_update:
- One-off task details or transient progress (use todowrite)
- Large code dumps, secrets/credentials, or file contents that can be re-read from the repo

Scope selection:
- user — cross-project preferences and habits
- project — conventions and decisions for the current project only
- agent — facts specific to this agent role

When to memory_read:
- Near the start of a new session or when prior preferences may matter
- User refers to something said earlier / "as we discussed"
- Before style, architecture, or convention-sensitive decisions

Writing style: short stable keys (e.g. preferred_language); one to a few factual sentences; update the same key instead of duplicating.
</memory-policy>`
}

func buildTableStorePolicy() string {
	return `<table-store-policy>
Use table_upsert / table_get / table_query / table_delete / table_list for queryable business rows that must survive across sessions (digests, counters, cursors, structured logs).

When to use table_*:
- Daily/weekly digests or automation outputs you may filter later
- Idempotent records keyed by date or external id
- Small structured JSON documents (not long prose)

Do NOT use table_*:
- Lasting preferences or conventions (use memory_*)
- Long-form reports (use write to project files; store a path in the row if needed)
- Secrets/credentials, binaries, or full mailbox dumps

Scopes match memory: user | project | agent. Prefer stable keys and small limits on table_query.
</table-store-policy>`
}

func buildMCPToolNamingPolicy() string {
	return `<mcp-tool-naming>
MCP tools are exposed to you as function names: mcp_<server>_<tool>
- <server> and <tool> are lowercased; non-alphanumeric characters become underscores (e.g. "My Server!" + "do.thing" → mcp_my_server_do_thing).
- Descriptions are prefixed with [MCP:<server display name>].
- Skills or docs may mention a short tool name (e.g. echo, search). When calling, use the full mcp_* name from your tool list whose suffix matches that short name (after mcp_<server>_).
- Prefer the exact name from the tool list. Do not invent mcp_* names that are not listed.
- If several servers expose the same short tool name, pick the server that matches the task (see [MCP:…] prefix / server slug).
</mcp-tool-naming>`
}

// buildRuntimeEnvironment returns a block describing the runtime OS / shell environment.
// Injected into the system prompt; shell fields come from the same resolve path as exec_shell.
func buildRuntimeEnvironment(st domain.SandboxStatus) string {
	osName := runtime.GOOS
	osLabel := osName
	switch osName {
	case "darwin":
		osLabel = "macOS"
	case "linux":
		osLabel = "Linux"
	case "windows":
		osLabel = "Windows"
	}
	sep := string(filepath.Separator)

	shell := st.Shell
	if shell == "" {
		shell = "sh"
		if osName == "windows" {
			shell = "cmd"
		}
	}

	var b strings.Builder
	b.WriteString("<runtime-environment>\n")
	b.WriteString("OS: " + osName + " (" + osLabel + ")\n")
	b.WriteString("Path separator: " + sep + "\n")
	b.WriteString("Shell: " + shell + "\n")
	if st.ShellPath != "" {
		b.WriteString("Shell path: " + st.ShellPath + "\n")
	}
	if st.Shell == "bash (WSL2)" {
		b.WriteString("via: wsl -e bash -lc\n")
	}
	if st.Backend != "" {
		b.WriteString("Sandbox backend: " + string(st.Backend) + "\n")
	}
	switch {
	case st.Shell == "cmd (Coreutils)" || (st.Shell == "cmd" && st.CoreutilsBin != ""):
		b.WriteString("Note: exec_shell uses cmd.exe with Microsoft Coreutils on PATH (ls/cat/grep/find/…). Prefer POSIX utility names and simple pipelines. Use NUL instead of /dev/null. Bash-only syntax requires shell=bash (Git for Windows) or runtime.sandbox.backend=wsl2.\n")
		if st.CoreutilsBin != "" {
			b.WriteString("Coreutils bin: " + st.CoreutilsBin + "\n")
		}
	case st.Shell == "cmd":
		b.WriteString("Note: neither bundled/system Coreutils nor Git Bash detected; exec_shell uses cmd.exe syntax. Install Microsoft Coreutils, Git for Windows, or set runtime.sandbox.backend=wsl2 for bash.\n")
	case strings.HasPrefix(st.Shell, "bash"):
		b.WriteString("Note: exec_shell invokes the Shell above under the OS sandbox when enabled (workspace-write by default). Prefer POSIX shell syntax. Avoid cmd.exe builtins unless necessary.\n")
	default:
		b.WriteString("Note: exec_shell runs under the OS sandbox when enabled (workspace-write by default).\n")
	}
	b.WriteString("</runtime-environment>")
	return b.String()
}

func buildSkillMetadata(skills []domain.Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<available_skills>\n")
	b.WriteString("  <!-- Use read_skill with the skill id from <path>: read_skill(path=\"<path>\") -->\n")
	b.WriteString("  <!-- Resource files: read_skill(path=\"<path>/references/file.md\") — never bare references/… -->\n")
	for _, sk := range skills {
		fmt.Fprintf(&b, "  <skill>\n")
		fmt.Fprintf(&b, "    <path>%s</path>\n", escapeXML(sk.ID))
		if sk.Name != "" && sk.Name != sk.ID {
			fmt.Fprintf(&b, "    <name>%s</name>\n", escapeXML(sk.Name))
		}
		if cat := skillCategory(sk); cat != "" {
			fmt.Fprintf(&b, "    <category>%s</category>\n", escapeXML(cat))
		}
		fmt.Fprintf(&b, "    <description>%s</description>\n", escapeXML(sk.Description))
		if sk.SystemHint != "" {
			fmt.Fprintf(&b, "    <hint>%s</hint>\n", escapeXML(sk.SystemHint))
		}
		fmt.Fprintf(&b, "  </skill>\n")
	}
	b.WriteString("</available_skills>")
	return b.String()
}

func buildAgentMetadata(agents []domain.Agent) string {
	if len(agents) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<available_agents>\n")
	b.WriteString("  <!-- Delegate work to these agents with delegate_agent(agent_id=..., goal=...) -->\n")
	for _, a := range agents {
		fmt.Fprintf(&b, "  <agent>\n")
		fmt.Fprintf(&b, "    <id>%s</id>\n", escapeXML(a.ID))
		fmt.Fprintf(&b, "    <name>%s</name>\n", escapeXML(a.Name))
		fmt.Fprintf(&b, "    <description>%s</description>\n", escapeXML(a.Description))
		fmt.Fprintf(&b, "  </agent>\n")
	}
	b.WriteString("</available_agents>")
	return b.String()
}

func skillToolSchemas(skills []domain.Skill, toolBindings []domain.ToolBinding) []domain.ToolSchema {
	bindings := make(map[string]domain.ToolBinding, len(toolBindings))
	for _, tb := range toolBindings {
		bindings[tb.ToolID] = tb
	}
	toolSet := map[string]struct{}{}
	for _, sk := range skills {
		for _, tid := range sk.ToolIDs {
			toolSet[tid] = struct{}{}
		}
	}
	var schemas []domain.ToolSchema
	for tid := range toolSet {
		if tb, ok := bindings[tid]; ok {
			schemas = append(schemas, domain.ToolSchema{
				Name:        tb.ToolID,
				Description: tb.ToolID,
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
				RiskLevel:   tb.RiskLevel,
			})
		}
	}
	return schemas
}

// skillCategory returns metadata.category when set (general | coding | work).
func skillCategory(sk domain.Skill) string {
	if sk.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(sk.Metadata["category"])
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

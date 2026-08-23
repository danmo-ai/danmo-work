package runtime

import (
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestBuildSkillMetadataIncludesCategory(t *testing.T) {
	out := buildSkillMetadata([]domain.Skill{
		{
			ID:          "writing-plans",
			Name:        "writing-plans",
			Description: "File-level implementation plans",
			Metadata:    map[string]string{"category": "coding"},
		},
		{
			ID:          "brainstorming",
			Name:        "brainstorming",
			Description: "Clarify requirements before building",
			Metadata:    map[string]string{"category": "work"},
		},
		{
			ID:          "skill-creator",
			Name:        "skill-creator",
			Description: "Create new skills",
			Metadata:    map[string]string{"category": "general"},
			SystemHint:  "use for skill authoring",
		},
		{
			ID:          "legacy",
			Name:        "legacy",
			Description: "No category",
		},
		{
			ID:          "tlc__pr-review",
			Name:        "PR Review",
			Description: "Review pull requests",
		},
	})

	if !strings.Contains(out, "<category>coding</category>") {
		t.Fatalf("missing coding category:\n%s", out)
	}
	if !strings.Contains(out, "<category>work</category>") {
		t.Fatalf("missing work category:\n%s", out)
	}
	if !strings.Contains(out, "<category>general</category>") {
		t.Fatalf("missing general category:\n%s", out)
	}
	if !strings.Contains(out, "<path>legacy</path>\n    <description>") {
		t.Fatalf("legacy skill should omit category:\n%s", out)
	}
	if !strings.Contains(out, "<hint>use for skill authoring</hint>") {
		t.Fatalf("missing system hint:\n%s", out)
	}
	if !strings.Contains(out, "<path>tlc__pr-review</path>") {
		t.Fatalf("path must use skill id:\n%s", out)
	}
	if !strings.Contains(out, "<name>PR Review</name>") {
		t.Fatalf("display name should appear when distinct from id:\n%s", out)
	}
	if strings.Contains(out, "<name>legacy</name>") {
		t.Fatalf("name omitted when equal to id:\n%s", out)
	}
	if !strings.Contains(out, `read_skill(path="<path>")`) {
		t.Fatalf("comment should refer to <path> id:\n%s", out)
	}
}

func TestBuildSkillMetadataEmpty(t *testing.T) {
	if got := buildSkillMetadata(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildSystemPromptPolicies(t *testing.T) {
	peers := []domain.Agent{{ID: "explorer", Name: "Explorer", Description: "Explore code"}}

	withDelegate := buildSystemPrompt("persona", nil, peers, true, "", "", "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if !strings.Contains(withDelegate, "<ask-user-policy>") {
		t.Fatal("expected ask-user-policy")
	}
	if !strings.Contains(withDelegate, "<delegation-policy>") {
		t.Fatal("expected delegation-policy when canDelegate")
	}
	if !strings.Contains(withDelegate, "Explicit summon (pass-through)") {
		t.Fatal("expected explicit summon pass-through rules in delegation-policy")
	}
	if !strings.Contains(withDelegate, "<available_agents>") || !strings.Contains(withDelegate, "explorer") {
		t.Fatalf("expected available_agents roster:\n%s", withDelegate)
	}

	noDelegate := buildSystemPrompt("persona", nil, peers, false, "", "", "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if strings.Contains(noDelegate, "<delegation-policy>") || strings.Contains(noDelegate, "<available_agents>") {
		t.Fatal("delegation blocks must not appear when canDelegate=false")
	}
	if !strings.Contains(noDelegate, "<ask-user-policy>") {
		t.Fatal("ask-user-policy is global")
	}
	if !strings.Contains(noDelegate, "<mcp-tool-naming>") || !strings.Contains(noDelegate, "mcp_<server>_<tool>") {
		t.Fatal("mcp-tool-naming policy is global")
	}
	if !strings.Contains(noDelegate, "<project-context-policy>") {
		t.Fatal("project-context-policy is global")
	}
	if !strings.Contains(noDelegate, "AGENTS.md") || !strings.Contains(noDelegate, "README.md") {
		t.Fatalf("project-context-policy should guide reading AGENTS.md / README.md:\n%s", noDelegate)
	}

	// CanDelegate with empty peer list still gets the policy (no roster).
	emptyPeers := buildSystemPrompt("persona", nil, nil, true, "", "", "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if !strings.Contains(emptyPeers, "<delegation-policy>") {
		t.Fatal("expected delegation-policy even with no peers")
	}
	if strings.Contains(emptyPeers, "<available_agents>\n") {
		t.Fatal("available_agents roster should be omitted when peer list empty")
	}
}

func TestBuildSystemPromptOmitsPlanMode(t *testing.T) {
	sys := buildSystemPrompt("persona", nil, nil, false, "", "", "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if strings.Contains(sys, "<plan-mode>") {
		t.Fatal("plan-mode belongs in turn-context, not the system prompt")
	}
}

func TestBuildRuntimeEnvironmentContainer(t *testing.T) {
	envSt := domain.EnvironmentStatus{
		Backend: domain.EnvironmentBackendContainer,
		Engine:  "podman",
		Image:   "localhost/danmo-work-env:bundled",
	}
	got := buildRuntimeEnvironment(domain.SandboxStatus{}, envSt)
	if !strings.Contains(got, "OCI container") {
		t.Fatalf("expected container block:\n%s", got)
	}
	if !strings.Contains(got, "podman") || !strings.Contains(got, "localhost/danmo-work-env:bundled") {
		t.Fatalf("expected engine+image:\n%s", got)
	}
	if !strings.Contains(got, "git, curl, jq") || !strings.Contains(got, "apk add --no-cache") {
		t.Fatalf("expected preinstalled list + apk hint:\n%s", got)
	}

	local := buildRuntimeEnvironment(domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if strings.Contains(local, "OCI container") {
		t.Fatal("container block must not appear for local backend")
	}
}

func TestBuildSkillMetadataSortsByID(t *testing.T) {
	out := buildSkillMetadata([]domain.Skill{
		{ID: "zeta", Description: "z"},
		{ID: "alpha", Description: "a"},
		{ID: "mu", Description: "m"},
	})
	alpha := strings.Index(out, "<path>alpha</path>")
	mu := strings.Index(out, "<path>mu</path>")
	zeta := strings.Index(out, "<path>zeta</path>")
	if alpha < 0 || mu < 0 || zeta < 0 || !(alpha < mu && mu < zeta) {
		t.Fatalf("skills should be sorted by id, got:\n%s", out)
	}
}

func TestBuildAgentMetadataSortsByID(t *testing.T) {
	out := buildAgentMetadata([]domain.Agent{
		{ID: "writer", Name: "Writer", Description: "w"},
		{ID: "explorer", Name: "Explorer", Description: "e"},
	})
	exp := strings.Index(out, "<id>explorer</id>")
	wr := strings.Index(out, "<id>writer</id>")
	if exp < 0 || wr < 0 || exp > wr {
		t.Fatalf("agents should be sorted by id, got:\n%s", out)
	}
}

func TestBuildSystemPromptCheckpointAfterPolicies(t *testing.T) {
	todos := "<active-todos>\n1. [in_progress] A (high)\n</active-todos>"
	files := "<session-file-changes>\n- update core/foo.go (edit) turns=t1\n</session-file-changes>"
	sys := buildSystemPrompt("persona", nil, nil, false, `{"summary":"x"}`, todos, files, domain.SandboxStatus{}, domain.EnvironmentStatus{})
	policy := strings.Index(sys, "<ask-user-policy>")
	runtime := strings.Index(sys, "<runtime-environment>")
	cp := strings.Index(sys, "<compaction-checkpoint>")
	todoIdx := strings.Index(sys, "<active-todos>")
	fileIdx := strings.Index(sys, "<session-file-changes>")
	if policy < 0 || runtime < 0 || cp < 0 || todoIdx < 0 || fileIdx < 0 {
		t.Fatalf("missing blocks:\n%s", sys)
	}
	if !(policy < runtime && runtime < cp && cp < todoIdx && todoIdx < fileIdx) {
		t.Fatalf("snapshot blocks should follow static policies, got policy=%d runtime=%d cp=%d todos=%d files=%d", policy, runtime, cp, todoIdx, fileIdx)
	}
	if strings.Contains(sys, "<turn-context>") || strings.Contains(sys, "<plan-mode>") {
		t.Fatalf("turn-context/plan-mode must not be in system prompt:\n%s", sys)
	}
}

func TestBuildSystemPromptDeterministic(t *testing.T) {
	skills := []domain.Skill{
		{ID: "b", Description: "b"},
		{ID: "a", Description: "a"},
	}
	agents := []domain.Agent{
		{ID: "z", Name: "Z", Description: "z"},
		{ID: "y", Name: "Y", Description: "y"},
	}
	a := buildSystemPrompt("p", skills, agents, true, "cp", "", "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	b := buildSystemPrompt("p", []domain.Skill{skills[1], skills[0]}, []domain.Agent{agents[1], agents[0]}, true, "cp", "", "", domain.SandboxStatus{}, domain.EnvironmentStatus{})
	if a != b {
		t.Fatal("same resources in different order must produce identical system prompt")
	}
}

func TestSkillToolSchemasSortedByID(t *testing.T) {
	skills := []domain.Skill{
		{ID: "s1", ToolIDs: []string{"write", "read_file"}},
	}
	bindings := []domain.ToolBinding{
		{ToolID: "write"},
		{ToolID: "read_file"},
	}
	got := skillToolSchemas(skills, bindings)
	if len(got) != 2 || got[0].Name != "read_file" || got[1].Name != "write" {
		t.Fatalf("expected read_file then write, got %+v", got)
	}
}

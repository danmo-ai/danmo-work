package runtime

import (
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestBuildSkillMetadataIncludesCategory(t *testing.T) {
	out := buildSkillMetadata([]domain.Skill{
		{
			Name:        "writing-plans",
			Description: "File-level implementation plans",
			Metadata:    map[string]string{"category": "coding"},
		},
		{
			Name:        "brainstorming",
			Description: "Clarify requirements before building",
			Metadata:    map[string]string{"category": "work"},
		},
		{
			Name:        "skill-creator",
			Description: "Create new skills",
			Metadata:    map[string]string{"category": "general"},
			SystemHint:  "use for skill authoring",
		},
		{
			Name:        "legacy",
			Description: "No category",
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
}

func TestBuildSkillMetadataEmpty(t *testing.T) {
	if got := buildSkillMetadata(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildSystemPromptPolicies(t *testing.T) {
	peers := []domain.Agent{{ID: "explorer", Name: "Explorer", Description: "Explore code"}}

	withDelegate := buildSystemPrompt("persona", nil, peers, true, "", "", "", domain.SandboxStatus{})
	if !strings.Contains(withDelegate, "<ask-user-policy>") {
		t.Fatal("expected ask-user-policy")
	}
	if !strings.Contains(withDelegate, "<delegation-policy>") {
		t.Fatal("expected delegation-policy when canDelegate")
	}
	if !strings.Contains(withDelegate, "<available_agents>") || !strings.Contains(withDelegate, "explorer") {
		t.Fatalf("expected available_agents roster:\n%s", withDelegate)
	}

	noDelegate := buildSystemPrompt("persona", nil, peers, false, "", "", "", domain.SandboxStatus{})
	if strings.Contains(noDelegate, "<delegation-policy>") || strings.Contains(noDelegate, "<available_agents>") {
		t.Fatal("delegation blocks must not appear when canDelegate=false")
	}
	if !strings.Contains(noDelegate, "<ask-user-policy>") {
		t.Fatal("ask-user-policy is global")
	}

	// CanDelegate with empty peer list still gets the policy (no roster).
	emptyPeers := buildSystemPrompt("persona", nil, nil, true, "", "", "", domain.SandboxStatus{})
	if !strings.Contains(emptyPeers, "<delegation-policy>") {
		t.Fatal("expected delegation-policy even with no peers")
	}
	if strings.Contains(emptyPeers, "<available_agents>\n") {
		t.Fatal("available_agents roster should be omitted when peer list empty")
	}
}

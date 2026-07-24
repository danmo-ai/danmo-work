package builtin

import (
	"context"
	"testing"

	"danmo-work/core/domain"
)

func TestReadSkillPrefersTurnFS(t *testing.T) {
	h := &ReadSkill{}
	h.SetTurnFS(
		map[string]domain.Skill{
			"demo": {ID: "demo", Name: "demo", Body: "fs-body"},
		},
		map[string][]domain.SkillFile{
			"demo": {{SkillID: "demo", Path: "references/note.md", Content: []byte("fs-ref")}},
		},
	)

	got, err := h.Execute(context.Background(), map[string]any{"path": "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "fs-body" {
		t.Fatalf("body=%q", got.Content)
	}

	got, err = h.Execute(context.Background(), map[string]any{"path": "demo/references/note.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "fs-ref" {
		t.Fatalf("ref=%q", got.Content)
	}
}

func TestReadSkillFSOnlyNotInDB(t *testing.T) {
	h := &ReadSkill{} // no Skills manager
	h.SetTurnFS(map[string]domain.Skill{
		"proj": {ID: "proj", Body: "hello"},
	}, nil)
	got, err := h.Execute(context.Background(), map[string]any{"path": "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello" {
		t.Fatalf("got %q", got.Content)
	}
}

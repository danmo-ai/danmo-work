package service

import (
	"context"
	"errors"
	"testing"

	"danmo-work/core/domain"
)

type memSkillRepo struct {
	byID map[string]domain.Skill
}

func newMemSkillRepo() *memSkillRepo {
	return &memSkillRepo{byID: make(map[string]domain.Skill)}
}

func (r *memSkillRepo) List(ctx context.Context) ([]domain.Skill, error) {
	out := make([]domain.Skill, 0, len(r.byID))
	for _, s := range r.byID {
		out = append(out, s)
	}
	return out, nil
}

func (r *memSkillRepo) Get(ctx context.Context, id string) (domain.Skill, error) {
	s, ok := r.byID[id]
	if !ok {
		return domain.Skill{}, errors.New("not found")
	}
	return s, nil
}

func (r *memSkillRepo) Upsert(ctx context.Context, s domain.Skill) error {
	r.byID[s.ID] = s
	return nil
}

func (r *memSkillRepo) Delete(ctx context.Context, id string) error {
	delete(r.byID, id)
	return nil
}

type memSkillFileRepo struct {
	byID map[string]domain.SkillFile
}

func newMemSkillFileRepo() *memSkillFileRepo {
	return &memSkillFileRepo{byID: make(map[string]domain.SkillFile)}
}

func (r *memSkillFileRepo) ListBySkill(ctx context.Context, skillID string) ([]domain.SkillFile, error) {
	var out []domain.SkillFile
	for _, f := range r.byID {
		if f.SkillID == skillID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *memSkillFileRepo) Get(ctx context.Context, skillID, path string) (domain.SkillFile, error) {
	f, ok := r.byID[skillID+":"+path]
	if !ok {
		return domain.SkillFile{}, errors.New("not found")
	}
	return f, nil
}

func (r *memSkillFileRepo) Upsert(ctx context.Context, f domain.SkillFile) error {
	if f.ID == "" {
		f.ID = f.SkillID + ":" + f.Path
	}
	r.byID[f.ID] = f
	return nil
}

func (r *memSkillFileRepo) Delete(ctx context.Context, skillID, path string) error {
	delete(r.byID, skillID+":"+path)
	return nil
}

func (r *memSkillFileRepo) DeleteBySkill(ctx context.Context, skillID string) error {
	for id, f := range r.byID {
		if f.SkillID == skillID {
			delete(r.byID, id)
		}
	}
	return nil
}

func TestEnsureFileDoesNotOverwrite(t *testing.T) {
	ctx := context.Background()
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())
	_ = skills.Upsert(ctx, domain.Skill{ID: "demo", Name: "demo", Builtin: true})

	original := domain.SkillFile{
		SkillID: "demo",
		Path:    "scripts/run.sh",
		Content: []byte("echo user"),
	}
	if err := skills.UpsertFile(ctx, original); err != nil {
		t.Fatal(err)
	}

	tmpl := domain.SkillFile{
		SkillID: "demo",
		Path:    "scripts/run.sh",
		Content: []byte("echo factory"),
	}
	if err := skills.EnsureFile(ctx, tmpl); err != nil {
		t.Fatal(err)
	}

	got, err := skills.File(ctx, "demo", "scripts/run.sh")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Content) != "echo user" {
		t.Fatalf("EnsureFile overwrote content: got %q", got.Content)
	}
}

func TestEnsureFileSeedsMissing(t *testing.T) {
	ctx := context.Background()
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())
	_ = skills.Upsert(ctx, domain.Skill{ID: "demo", Name: "demo", Builtin: true})

	tmpl := domain.SkillFile{
		SkillID: "demo",
		Path:    "references/guide.md",
		Content: []byte("# guide"),
	}
	if err := skills.EnsureFile(ctx, tmpl); err != nil {
		t.Fatal(err)
	}
	got, err := skills.File(ctx, "demo", "references/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Content) != "# guide" {
		t.Fatalf("expected seeded content, got %q", got.Content)
	}
}

func TestDivergedFromTemplate(t *testing.T) {
	ctx := context.Background()
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())

	tmpl := domain.Skill{
		ID:          "demo",
		Name:        "demo",
		Description: "factory",
		Body:        "body v2",
		Builtin:     true,
	}
	tmplFiles := []domain.SkillFile{{
		SkillID: "demo",
		Path:    "scripts/run.sh",
		Content: []byte("echo v2"),
	}}
	skills.SetTemplateLoader(func(id string) (*domain.Skill, error) {
		if id != "demo" {
			return nil, errors.New("missing")
		}
		cp := tmpl
		return &cp, nil
	})
	skills.SetFileTemplateLoader(func(id string) ([]domain.SkillFile, error) {
		return tmplFiles, nil
	})

	stored := tmpl
	stored.Body = "body v1"
	_ = skills.Upsert(ctx, stored)
	_ = skills.UpsertFile(ctx, domain.SkillFile{
		SkillID: "demo",
		Path:    "scripts/run.sh",
		Content: []byte("echo v2"),
	})

	got, err := skills.Get(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !got.TemplateDiverged {
		t.Fatal("expected templateDiverged when body differs")
	}

	_ = skills.Upsert(ctx, tmpl)
	got, err = skills.Get(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.TemplateDiverged {
		t.Fatal("expected no divergence when content matches template")
	}

	_ = skills.UpsertFile(ctx, domain.SkillFile{
		SkillID: "demo",
		Path:    "scripts/run.sh",
		Content: []byte("echo user"),
	})
	got, err = skills.Get(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !got.TemplateDiverged {
		t.Fatal("expected templateDiverged when resource file differs")
	}

	reset, err := skills.ResetFromTemplate(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if reset.TemplateDiverged {
		t.Fatal("reset skill should not be diverged")
	}
	got, err = skills.Get(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.TemplateDiverged {
		t.Fatal("after reset, skill should match template")
	}
	file, err := skills.File(ctx, "demo", "scripts/run.sh")
	if err != nil {
		t.Fatal(err)
	}
	if string(file.Content) != "echo v2" {
		t.Fatalf("reset should restore file content, got %q", file.Content)
	}
}

func TestUpsertPreservesBuiltin(t *testing.T) {
	ctx := context.Background()
	skills := NewSkillManager(newMemSkillRepo(), newMemSkillFileRepo())
	_ = skills.Upsert(ctx, domain.Skill{ID: "demo", Name: "demo", Builtin: true, Body: "a"})
	_ = skills.Upsert(ctx, domain.Skill{ID: "demo", Name: "demo", Builtin: false, Body: "b"})
	got, err := skills.Get(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Builtin {
		t.Fatal("builtin flag must be preserved on update")
	}
	if got.Body != "b" {
		t.Fatalf("body not updated: %q", got.Body)
	}
}

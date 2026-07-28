package service

import (
	"strings"
	"testing"
)

func TestNormalizeSkillBodyRefs(t *testing.T) {
	const id = "debugging"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "backtick bare reference",
			in:   "See `references/patterns.md` for details.",
			want: "See `debugging/references/patterns.md` for details.",
		},
		{
			name: "markdown link",
			in:   "Open [guide](references/guide.md) now.",
			want: "Open [guide](debugging/references/guide.md) now.",
		},
		{
			name: "dot slash",
			in:   "Run `./scripts/check.sh` please.",
			want: "Run `debugging/scripts/check.sh` please.",
		},
		{
			name: "already prefixed",
			in:   "See `debugging/references/patterns.md`.",
			want: "See `debugging/references/patterns.md`.",
		},
		{
			name: "other skill prefix left alone",
			in:   "See `other/references/patterns.md`.",
			want: "See `other/references/patterns.md`.",
		},
		{
			name: "url left alone",
			in:   "Docs at [x](https://example.com/references/x.md).",
			want: "Docs at [x](https://example.com/references/x.md).",
		},
		{
			name: "absolute left alone",
			in:   "File `/references/x.md` is absolute.",
			want: "File `/references/x.md` is absolute.",
		},
		{
			name: "dir-only mention left alone",
			in:   "Put docs under `references/` please.",
			want: "Put docs under `references/` please.",
		},
		{
			name: "assets",
			in:   "Icon: ![i](assets/icon.png)",
			want: "Icon: ![i](debugging/assets/icon.png)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSkillBodyRefs(tt.in, id)
			if got != tt.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestNormalizeSkillBodyRefsAfterIDChange(t *testing.T) {
	in := "See `pr-review/references/guide.md` and `references/extra.md`."
	got := NormalizeSkillBodyRefsAfterIDChange(in, "tlc__pr-review", "pr-review")
	if !strings.Contains(got, "`tlc__pr-review/references/guide.md`") {
		t.Fatalf("expected remapped prefix, got %q", got)
	}
	if !strings.Contains(got, "`tlc__pr-review/references/extra.md`") {
		t.Fatalf("expected bare ref prefixed with new id, got %q", got)
	}
	if strings.Contains(got, "`pr-review/references/") {
		t.Fatalf("old id prefix should be gone: %q", got)
	}
}

func TestNormalizeSkillBodyRefsIdempotent(t *testing.T) {
	in := "See `references/a.md`."
	once := NormalizeSkillBodyRefs(in, "demo")
	twice := NormalizeSkillBodyRefs(once, "demo")
	if once != twice {
		t.Fatalf("not idempotent:\n%s\nvs\n%s", once, twice)
	}
}

package service

import "testing"

func TestPathsFromOfficeEdit(t *testing.T) {
	in := "[office-edit]\naction: polish\npath: docs/a.md\nkind: doc\n"
	got := PathsFromOfficeEdit(in)
	if len(got) != 1 || got[0] != "docs/a.md" {
		t.Fatalf("got %#v", got)
	}
	if PathsFromOfficeEdit("hello") != nil {
		t.Fatal("expected nil")
	}
}

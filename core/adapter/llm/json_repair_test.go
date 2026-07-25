package llm

import (
	"encoding/json"
	"testing"
)

func TestRepairJSONObjectUnescapedQuotes(t *testing.T) {
	// Model forgot to escape quotes inside content.
	raw := []byte(`{"path":"a.txt","content":"he said "hello" then left"}`)
	out, err := repairJSONObject(raw)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	var args map[string]any
	if err := json.Unmarshal(out, &args); err != nil {
		t.Fatalf("unmarshal repaired: %v\nrepaired=%s", err, out)
	}
	if args["path"] != "a.txt" {
		t.Errorf("path: got %#v", args["path"])
	}
	if args["content"] != `he said "hello" then left` {
		t.Errorf("content: got %#v", args["content"])
	}
}

func TestRepairJSONObjectRawNewlines(t *testing.T) {
	raw := []byte("{\"path\":\"a.txt\",\"content\":\"line1\nline2\ttab\"}")
	out, err := repairJSONObject(raw)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	var args map[string]any
	if err := json.Unmarshal(out, &args); err != nil {
		t.Fatalf("unmarshal repaired: %v\nrepaired=%s", err, out)
	}
	if args["content"] != "line1\nline2\ttab" {
		t.Errorf("content: got %#v", args["content"])
	}
}

func TestRepairJSONObjectTrailingComma(t *testing.T) {
	raw := []byte(`{"path":"a.txt","content":"x",}`)
	out, err := repairJSONObject(raw)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	var args map[string]any
	if err := json.Unmarshal(out, &args); err != nil {
		t.Fatalf("unmarshal repaired: %v", err)
	}
	if args["path"] != "a.txt" {
		t.Errorf("path: got %#v", args["path"])
	}
}

func TestRepairJSONObjectValidUnchanged(t *testing.T) {
	raw := []byte(`{"path":"a.txt","content":"he said \"hello\""}`)
	out, err := repairJSONObject(raw)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	var args map[string]any
	if err := json.Unmarshal(out, &args); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if args["content"] != `he said "hello"` {
		t.Errorf("content: got %#v", args["content"])
	}
}

func TestRepairJSONObjectWriteFileLike(t *testing.T) {
	// Typical write_file damage: code snippet with quotes + newline.
	raw := []byte("{\"path\":\"main.go\",\"content\":\"package main\n\nfunc main() {\n  fmt.Println(\"hi\")\n}\"}")
	out, err := repairJSONObject(raw)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	var args map[string]any
	if err := json.Unmarshal(out, &args); err != nil {
		t.Fatalf("unmarshal repaired: %v\nrepaired=%s", err, out)
	}
	want := "package main\n\nfunc main() {\n  fmt.Println(\"hi\")\n}"
	if args["content"] != want {
		t.Errorf("content:\n got %#v\nwant %#v", args["content"], want)
	}
}

func TestParseArgsRepairsBrokenStringArguments(t *testing.T) {
	// Outer response encodes arguments as a JSON string whose decoded value is
	// invalid JSON due to unescaped inner quotes.
	inner := `{"question":"say "hi" please"}`
	wrapped, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	args, err := parseArgs(wrapped)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if args["question"] != `say "hi" please` {
		t.Errorf("question: got %#v", args["question"])
	}
}

func TestParseArgsStillFailsOnGarbage(t *testing.T) {
	_, err := parseArgs(json.RawMessage(`"not-an-object"`))
	if err == nil {
		t.Fatal("expected error for non-object arguments")
	}
}

func TestIsLikelyStringEnd(t *testing.T) {
	cases := []struct {
		s    string
		next int
		want bool
	}{
		{`"x",`, 3, true},
		{`"x"}`, 3, true},
		{`"x" ]`, 3, true},
		{`"x":`, 3, true},
		{`"x"`, 3, true},
		{`"say "hi"`, 5, false}, // quote before hi
	}
	for _, tc := range cases {
		if got := isLikelyStringEnd(tc.s, tc.next); got != tc.want {
			t.Errorf("isLikelyStringEnd(%q, %d)=%v want %v", tc.s, tc.next, got, tc.want)
		}
	}
}

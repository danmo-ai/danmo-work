package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
)

// mockStdioServer is a tiny MCP server script used for Dial/List/Call tests.
const mockStdioServer = `#!/usr/bin/env python3
import json, sys

def read_msg():
    line = sys.stdin.readline()
    if not line:
        return None
    return json.loads(line)

def write_msg(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

while True:
    msg = read_msg()
    if msg is None:
        break
    method = msg.get("method")
    mid = msg.get("id")
    if method == "initialize":
        write_msg({"jsonrpc":"2.0","id":mid,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"mock","version":"1"}}})
    elif method == "notifications/initialized":
        pass
    elif method == "tools/list":
        write_msg({"jsonrpc":"2.0","id":mid,"result":{"tools":[{"name":"echo","description":"echo args","inputSchema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}}]}})
    elif method == "tools/call":
        args = (msg.get("params") or {}).get("arguments") or {}
        text = args.get("text", "")
        write_msg({"jsonrpc":"2.0","id":mid,"result":{"content":[{"type":"text","text":"echo:"+text}]}})
    elif mid is not None:
        write_msg({"jsonrpc":"2.0","id":mid,"error":{"code":-32601,"message":"unknown"}})
`

func TestStdioListAndCall(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 required")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "mock_mcp.py")
	if err := os.WriteFile(script, []byte(mockStdioServer), 0o755); err != nil {
		t.Fatal(err)
	}
	d := NewDialer()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sess, err := d.Dial(ctx, domain.MCPServer{
		Transport: "stdio",
		Command:   "python3",
		Args:      script,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer sess.Close()

	tools, err := sess.ListTools(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools = %+v", tools)
	}
	if tools[0].InputSchema == nil {
		t.Fatal("expected inputSchema")
	}

	out, err := sess.CallTool(ctx, "echo", map[string]any{"text": "hi"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if out != "echo:hi" {
		t.Fatalf("out = %q", out)
	}
}

func TestExposedNameSanitized(t *testing.T) {
	name := domain.ExposedMCPToolName("My Server!", "do.thing")
	if name != "mcp_my_server_do_thing" {
		t.Fatalf("got %s", name)
	}
}

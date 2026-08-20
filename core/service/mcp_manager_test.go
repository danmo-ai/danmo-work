package service

import (
	"context"
	"sync"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type mockMCPSession struct {
	tools []port.MCPToolInfo
}

func (s *mockMCPSession) ListTools(_ context.Context) ([]port.MCPToolInfo, error) {
	return s.tools, nil
}

func (s *mockMCPSession) CallTool(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}

func (s *mockMCPSession) Close() error { return nil }

type mockMCPDialer struct {
	tools []port.MCPToolInfo
}

func (d *mockMCPDialer) Dial(_ context.Context, _ domain.MCPServer) (port.MCPSession, error) {
	return &mockMCPSession{tools: d.tools}, nil
}

type recordingMCPSync struct {
	mu    sync.Mutex
	calls map[string][]domain.MCPToolBinding
}

func newRecordingMCPSync() *recordingMCPSync {
	return &recordingMCPSync{calls: make(map[string][]domain.MCPToolBinding)}
}

func (r *recordingMCPSync) ReplaceMCPServer(serverID string, tools []domain.MCPToolBinding, _ bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := append([]domain.MCPToolBinding(nil), tools...)
	r.calls[serverID] = cp
}

func (r *recordingMCPSync) RemoveMCPServer(serverID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.calls, serverID)
}

func (r *recordingMCPSync) bindings(serverID string) []domain.MCPToolBinding {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.MCPToolBinding(nil), r.calls[serverID]...)
}

func testMCPManager(t *testing.T, dialer port.MCPDialer, syncer port.MCPToolSync) *MCPManager {
	t.Helper()
	m := NewMCPManager(t.TempDir())
	m.SetDialer(dialer)
	if syncer != nil {
		m.SetToolSync(syncer)
	}
	return m
}

func TestAutoDiscoverAllWithoutSyncer(t *testing.T) {
	dialer := &mockMCPDialer{tools: []port.MCPToolInfo{{Name: "ping", Description: "ping"}}}
	m := testMCPManager(t, dialer, nil)
	ctx := context.Background()

	m.mu.Lock()
	m.servers["srv-a"] = domain.MCPServer{
		ID: "srv-a", Name: "srv-a", Transport: "stdio", Command: "echo", Enabled: true,
	}
	m.mu.Unlock()

	m.AutoDiscoverAll(ctx)
}

func TestAutoDiscoverAllSyncsTools(t *testing.T) {
	dialer := &mockMCPDialer{tools: []port.MCPToolInfo{
		{Name: "search", Description: "search repos"},
	}}
	syncer := newRecordingMCPSync()
	m := testMCPManager(t, dialer, syncer)
	ctx := context.Background()

	m.mu.Lock()
	m.servers["github"] = domain.MCPServer{
		ID: "github", Name: "github", Transport: "streamable-http",
		URL: "https://example.com/mcp", Enabled: true,
	}
	m.mu.Unlock()

	m.AutoDiscoverAll(ctx)

	bindings := syncer.bindings("github")
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].ToolName != "search" {
		t.Fatalf("unexpected tool: %+v", bindings[0])
	}
	if bindings[0].ExposedName != domain.ExposedMCPToolName("github", "search") {
		t.Fatalf("unexpected exposed name: %q", bindings[0].ExposedName)
	}
}

func TestCreateRefreshesTools(t *testing.T) {
	dialer := &mockMCPDialer{tools: []port.MCPToolInfo{
		{Name: "list_issues", Description: "list issues"},
	}}
	syncer := newRecordingMCPSync()
	m := testMCPManager(t, dialer, syncer)
	ctx := context.Background()

	srv, err := m.Create(ctx, domain.UpsertMCPServerRequest{
		ID:        "my-mcp",
		Name:      "my-mcp",
		Transport: "stdio",
		Command:   "mock-mcp",
		Enabled:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(srv.DiscoveredTools) != 1 {
		t.Fatalf("expected discovered tools on create, got %+v", srv.DiscoveredTools)
	}
	bindings := syncer.bindings("my-mcp")
	if len(bindings) != 1 || bindings[0].ToolName != "list_issues" {
		t.Fatalf("unexpected sync bindings: %+v", bindings)
	}
}

func TestCreateDisabledSkipsRefresh(t *testing.T) {
	dialer := &mockMCPDialer{tools: []port.MCPToolInfo{{Name: "ping"}}}
	syncer := newRecordingMCPSync()
	m := testMCPManager(t, dialer, syncer)
	ctx := context.Background()

	srv, err := m.Create(ctx, domain.UpsertMCPServerRequest{
		ID: "off", Name: "off", Transport: "stdio", Command: "mock", Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(srv.DiscoveredTools) != 0 {
		t.Fatalf("disabled create should not discover tools, got %+v", srv.DiscoveredTools)
	}
	if len(syncer.bindings("off")) != 0 {
		t.Fatalf("disabled create should not sync tool bindings")
	}
}

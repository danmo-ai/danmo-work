package service

import (
	"context"
	"fmt"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type MCPManager struct {
	repo port.MCPServerRepo
}

func NewMCPManager(repo port.MCPServerRepo) *MCPManager {
	return &MCPManager{repo: repo}
}

func (m *MCPManager) List(ctx context.Context) ([]domain.MCPServer, error) {
	return m.repo.List(ctx)
}

func (m *MCPManager) Get(ctx context.Context, id string) (domain.MCPServer, error) {
	return m.repo.Get(ctx, id)
}

func (m *MCPManager) Create(ctx context.Context, req domain.UpsertMCPServerRequest) (domain.MCPServer, error) {
	if req.Name == "" {
		return domain.MCPServer{}, fmt.Errorf("MCP server name is required")
	}
	s := domain.MCPServer{
		ID:           fmt.Sprintf("mcp-%d", time.Now().UnixNano()),
		Name:         req.Name,
		Description:  req.Description,
		Transport:    req.Transport,
		Command:      req.Command,
		Args:         req.Args,
		URL:          req.URL,
		Env:          req.Env,
		Headers:      req.Headers,
		EnabledTools: req.EnabledTools,
		ToolTimeout:  req.ToolTimeout,
		Status:       "disconnected",
		Enabled:      req.Enabled,
	}
	if s.Transport == "" {
		if s.Command != "" {
			s.Transport = "stdio"
		} else if s.URL != "" {
			s.Transport = "streamable-http"
		}
	}
	if s.ToolTimeout <= 0 {
		s.ToolTimeout = 300
	}
	if len(s.EnabledTools) == 0 {
		s.EnabledTools = []string{"*"}
	}
	if err := m.repo.Upsert(ctx, s); err != nil {
		return domain.MCPServer{}, err
	}
	return s, nil
}

func (m *MCPManager) Update(ctx context.Context, id string, req domain.UpsertMCPServerRequest) (domain.MCPServer, error) {
	existing, err := m.repo.Get(ctx, id)
	if err != nil {
		return domain.MCPServer{}, fmt.Errorf("MCP server not found: %w", err)
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.Transport = req.Transport
	existing.Command = req.Command
	existing.Args = req.Args
	existing.URL = req.URL
	existing.Env = req.Env
	existing.Headers = req.Headers
	if len(req.EnabledTools) > 0 {
		existing.EnabledTools = req.EnabledTools
	}
	if req.ToolTimeout > 0 {
		existing.ToolTimeout = req.ToolTimeout
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	existing.Enabled = req.Enabled
	if err := m.repo.Upsert(ctx, existing); err != nil {
		return domain.MCPServer{}, err
	}
	return existing, nil
}

func (m *MCPManager) Delete(ctx context.Context, id string) error {
	return m.repo.Delete(ctx, id)
}

// RefreshTools connects to the MCP server and discovers available tools.
// It merges the discovered tools with existing enabled state.
func (m *MCPManager) RefreshTools(ctx context.Context, id string) ([]domain.MCPToolDef, error) {
	srv, err := m.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MCP server not found: %w", err)
	}

	timeout := time.Duration(srv.ToolTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	discovered, err := discoverTools(ctx, srv)
	if err != nil {
		return nil, fmt.Errorf("discover tools: %w", err)
	}

	// Merge with existing enabled state
	prevEnabled := make(map[string]bool)
	for _, t := range srv.DiscoveredTools {
		prevEnabled[t.Name] = t.Enabled
	}
	for i := range discovered {
		if wasEnabled, ok := prevEnabled[discovered[i].Name]; ok {
			discovered[i].Enabled = wasEnabled
		}
	}

	srv.DiscoveredTools = discovered

	// Update enabledTools to reflect newly enabled tools
	enabledSet := make(map[string]bool)
	for _, t := range discovered {
		if t.Enabled {
			enabledSet[t.Name] = true
		}
	}
	if len(enabledSet) > 0 {
		names := make([]string, 0, len(enabledSet))
		for name := range enabledSet {
			names = append(names, name)
		}
		srv.EnabledTools = names
	} else {
		srv.EnabledTools = []string{"*"}
	}

	if err := m.repo.Upsert(ctx, srv); err != nil {
		return nil, err
	}
	return discovered, nil
}

// ToggleTool enables or disables a single discovered tool.
func (m *MCPManager) ToggleTool(ctx context.Context, id string, toolName string, enabled bool) (domain.MCPServer, error) {
	srv, err := m.repo.Get(ctx, id)
	if err != nil {
		return domain.MCPServer{}, fmt.Errorf("MCP server not found: %w", err)
	}

	found := false
	for i, t := range srv.DiscoveredTools {
		if t.Name == toolName {
			srv.DiscoveredTools[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return domain.MCPServer{}, fmt.Errorf("tool not found: %s", toolName)
	}

	// Rebuild enabledTools list
	enabledNames := make([]string, 0)
	for _, t := range srv.DiscoveredTools {
		if t.Enabled {
			enabledNames = append(enabledNames, t.Name)
		}
	}
	if len(enabledNames) > 0 {
		srv.EnabledTools = enabledNames
	} else {
		srv.EnabledTools = []string{"*"}
	}

	if err := m.repo.Upsert(ctx, srv); err != nil {
		return domain.MCPServer{}, err
	}
	return srv, nil
}

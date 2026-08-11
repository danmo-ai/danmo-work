package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

const mcpSchemaURL = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"

type mcpSpecServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    string            `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
}

type mcpSpecDoc struct {
	Schema     string                   `json:"$schema"`
	MCPServers map[string]mcpSpecServer `json:"mcpServers"`
}

// MCPManager persists MCP server config (mcp.json), keeps live sessions, and syncs tools
// into the runtime catalog via MCPToolSync.
type MCPManager struct {
	mcpFile string
	servers map[string]domain.MCPServer
	secrets port.SecretStore
	dialer  port.MCPDialer
	syncer  port.MCPToolSync

	mu       sync.Mutex
	sessions map[string]port.MCPSession
}

func NewMCPManager(dataDir string) *MCPManager {
	m := &MCPManager{
		mcpFile:  filepath.Join(dataDir, "mcp.json"),
		servers:  make(map[string]domain.MCPServer),
		sessions: make(map[string]port.MCPSession),
	}
	m.loadFromDisk()
	return m
}

func (m *MCPManager) SetDialer(d port.MCPDialer) { m.dialer = d }

func (m *MCPManager) SetSecretStore(s port.SecretStore) { m.secrets = s }

func (m *MCPManager) SetToolSync(s port.MCPToolSync) { m.syncer = s }

func (m *MCPManager) loadFromDisk() {
	data, err := os.ReadFile(m.mcpFile)
	if err != nil {
		return
	}
	var doc mcpSpecDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return
	}
	for id, srv := range doc.MCPServers {
		server := mcpSpecToDomain(id, srv)
		normalizeMCPServer(&server)
		m.servers[id] = server
	}
}

func (m *MCPManager) saveToDisk() error {
	doc := mcpSpecDoc{
		Schema:     mcpSchemaURL,
		MCPServers: make(map[string]mcpSpecServer),
	}
	for id, srv := range m.servers {
		doc.MCPServers[id] = domainToMCPSpec(srv)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.mcpFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.mcpFile, append(data, '\n'), 0o644)
}

func mcpSpecToDomain(id string, spec mcpSpecServer) domain.MCPServer {
	var args string
	if spec.Args != "" {
		args = spec.Args
	}
	var env string
	if len(spec.Env) > 0 {
		parts := make([]string, 0, len(spec.Env))
		for k, v := range spec.Env {
			parts = append(parts, k+"="+v)
		}
		env = strings.Join(parts, "\n")
	}
	return domain.MCPServer{
		ID:          id,
		Name:        id,
		Transport:   spec.Type,
		Command:     spec.Command,
		Args:        args,
		URL:         spec.URL,
		Env:         env,
		Headers:     spec.Headers,
		Enabled:     true,
		AmbientMount: true,
	}
}

func domainToMCPSpec(srv domain.MCPServer) mcpSpecServer {
	spec := mcpSpecServer{
		Type:    srv.Transport,
		Command: srv.Command,
		Args:    srv.Args,
		URL:     srv.URL,
		Headers: srv.Headers,
	}
	if srv.Env != "" {
		envMap := make(map[string]string)
		for _, line := range strings.Split(srv.Env, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				envMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if len(envMap) > 0 {
			spec.Env = envMap
		}
	}
	return spec
}

// SyncBuiltinMCP adds built-in MCP server entries that are missing from the user's mcp.json.
// Never overwrites existing entries with the same ID.
func (m *MCPManager) SyncBuiltinMCP() error {
	builtins := map[string]mcpSpecServer{}

	danmoMakeURL := ResolveDanmoMakeMCPURL()
	if danmoMakeURL != "" {
		builtins["danmo-make"] = mcpSpecServer{
			Type: "streamable-http",
			URL:  danmoMakeURL,
		}
	}

	builtins["github"] = mcpSpecServer{
		Type: "streamable-http",
		URL:  "https://api.githubcopilot.com/mcp/",
	}

	m.mu.Lock()
	changed := false
	for id, spec := range builtins {
		if _, exists := m.servers[id]; !exists {
			m.servers[id] = mcpSpecToDomain(id, spec)
			changed = true
			log.Printf("[mcp] added builtin server %q", id)
		}
	}
	saveErr := m.saveToDisk()
	m.mu.Unlock()

	if changed {
		log.Printf("[mcp] builtin sync: %d new server(s)", 1)
	}
	return saveErr
}

func (m *MCPManager) List(ctx context.Context) ([]domain.MCPServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]domain.MCPServer, 0, len(m.servers))
	for _, s := range m.servers {
		list = append(list, s)
	}
	return list, nil
}

func (m *MCPManager) Get(ctx context.Context, id string) (domain.MCPServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[id]
	if !ok {
		return domain.MCPServer{}, fmt.Errorf("MCP server %q not found", id)
	}
	return s, nil
}

func (m *MCPManager) Create(ctx context.Context, req domain.UpsertMCPServerRequest) (domain.MCPServer, error) {
	if req.Name == "" {
		return domain.MCPServer{}, fmt.Errorf("MCP server name is required")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = fmt.Sprintf("mcp-%d", time.Now().UnixNano())
	}
	s := domain.MCPServer{
		ID:                id,
		Name:              req.Name,
		Description:       req.Description,
		Transport:         req.Transport,
		Command:           req.Command,
		Args:              req.Args,
		URL:               req.URL,
		Env:               req.Env,
		Headers:           req.Headers,
		Auth:              req.Auth,
		SecretHeadersRef:  req.SecretHeadersRef,
		OAuthClientID:     req.OAuthClientID,
		OAuthAuthorizeURL: req.OAuthAuthorizeURL,
		OAuthTokenURL:     req.OAuthTokenURL,
		OAuthScopes:       req.OAuthScopes,
		CatalogID:         req.CatalogID,
		MarketSource:      req.MarketSource,
		EnabledTools:      req.EnabledTools,
		ToolTimeout:       req.ToolTimeout,
		Status:            "disconnected",
		Enabled:           req.Enabled,
		Network:           req.Network,
		AmbientMount:      true,
	}
	if req.AmbientMount != nil {
		s.AmbientMount = *req.AmbientMount
	}
	normalizeMCPServer(&s)
	if err := m.storeHeaderSecrets(ctx, s.ID, req.HeaderSecrets, &s); err != nil {
		return domain.MCPServer{}, err
	}
	m.mu.Lock()
	m.servers[id] = s
	err := m.saveToDisk()
	m.mu.Unlock()
	if err != nil {
		return domain.MCPServer{}, err
	}
	_ = m.syncServer(ctx, s)
	return s, nil
}

func (m *MCPManager) Update(ctx context.Context, id string, req domain.UpsertMCPServerRequest) (domain.MCPServer, error) {
	m.mu.Lock()
	existing, ok := m.servers[id]
	if !ok {
		m.mu.Unlock()
		return domain.MCPServer{}, fmt.Errorf("MCP server not found")
	}
	m.mu.Unlock()
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
	if req.Auth != "" {
		existing.Auth = req.Auth
	}
	if req.SecretHeadersRef != nil {
		existing.SecretHeadersRef = req.SecretHeadersRef
	}
	if req.OAuthClientID != "" {
		existing.OAuthClientID = req.OAuthClientID
	}
	if req.OAuthAuthorizeURL != "" {
		existing.OAuthAuthorizeURL = req.OAuthAuthorizeURL
	}
	if req.OAuthTokenURL != "" {
		existing.OAuthTokenURL = req.OAuthTokenURL
	}
	if req.OAuthScopes != "" {
		existing.OAuthScopes = req.OAuthScopes
	}
	if req.CatalogID != "" {
		existing.CatalogID = req.CatalogID
	}
	if req.MarketSource != "" {
		existing.MarketSource = req.MarketSource
	}
	if len(req.EnabledTools) > 0 {
		existing.EnabledTools = req.EnabledTools
	}
	if req.ToolTimeout > 0 {
		existing.ToolTimeout = req.ToolTimeout
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	if req.Network != "" {
		existing.Network = req.Network
	}
	if req.AmbientMount != nil {
		existing.AmbientMount = *req.AmbientMount
	}
	existing.Enabled = req.Enabled
	normalizeMCPServer(&existing)
	if err := m.storeHeaderSecrets(ctx, existing.ID, req.HeaderSecrets, &existing); err != nil {
		return domain.MCPServer{}, err
	}
	m.closeSession(id)
	m.mu.Lock()
	m.servers[id] = existing
	err := m.saveToDisk()
	m.mu.Unlock()
	if err != nil {
		return domain.MCPServer{}, err
	}
	_ = m.syncServer(ctx, existing)
	return existing, nil
}

func (m *MCPManager) Delete(ctx context.Context, id string) error {
	m.closeSession(id)
	if m.secrets != nil {
		_ = m.secrets.DeletePrefix(ctx, secretPrefixMCP(id))
	}
	if m.syncer != nil {
		m.syncer.RemoveMCPServer(id)
	}
	m.mu.Lock()
	delete(m.servers, id)
	err := m.saveToDisk()
	m.mu.Unlock()
	return err
}

func (m *MCPManager) FindByCatalogID(ctx context.Context, catalogID string) (domain.MCPServer, bool, error) {
	catalogID = strings.TrimSpace(catalogID)
	if catalogID == "" {
		return domain.MCPServer{}, false, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		if s.ID == catalogID || s.CatalogID == catalogID {
			return s, true, nil
		}
	}
	return domain.MCPServer{}, false, nil
}

func (m *MCPManager) RefreshTools(ctx context.Context, id string) ([]domain.MCPToolDef, error) {
	srv, err := m.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MCP server not found: %w", err)
	}
	timeout := time.Duration(srv.ToolTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sess, err := m.openSession(ctx, srv)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	discoveredInfo, err := sess.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover tools: %w", err)
	}

	prevEnabled := make(map[string]bool)
	for _, t := range srv.DiscoveredTools {
		prevEnabled[t.Name] = t.Enabled
	}
	discovered := make([]domain.MCPToolDef, len(discoveredInfo))
	for i, t := range discoveredInfo {
		enabled := true
		if was, ok := prevEnabled[t.Name]; ok {
			enabled = was
		}
		discovered[i] = domain.MCPToolDef{
			Name:        t.Name,
			Description: t.Description,
			Enabled:     enabled,
			InputSchema: t.InputSchema,
		}
	}
	srv.DiscoveredTools = discovered
	srv.Status = "connected"
	rebuildEnabledTools(&srv)
	m.mu.Lock()
	m.servers[id] = srv
	m.mu.Unlock()
	_ = m.syncServer(ctx, srv)
	return discovered, nil
}

func (m *MCPManager) ToggleTool(ctx context.Context, id string, toolName string, enabled bool) (domain.MCPServer, error) {
	srv, err := m.Get(ctx, id)
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
	rebuildEnabledTools(&srv)
	m.mu.Lock()
	m.servers[id] = srv
	m.mu.Unlock()
	_ = m.syncServer(ctx, srv)
	return srv, nil
}

func (m *MCPManager) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (string, error) {
	srv, err := m.Get(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("MCP server not found: %w", err)
	}
	if !srv.Enabled {
		return "", fmt.Errorf("MCP server %s is disabled", srv.Name)
	}
	timeout := time.Duration(srv.ToolTimeout) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sess, err := m.openSession(ctx, srv)
	if err != nil {
		return "", err
	}
	return sess.CallTool(ctx, toolName, args)
}

func (m *MCPManager) SyncAll(ctx context.Context) error {
	servers, err := m.List(ctx)
	if err != nil {
		return err
	}
	for _, srv := range servers {
		_ = m.syncServer(ctx, srv)
	}
	return nil
}

func (m *MCPManager) syncServer(ctx context.Context, srv domain.MCPServer) error {
	if m.syncer == nil {
		return nil
	}
	if !srv.Enabled {
		m.syncer.RemoveMCPServer(srv.ID)
		m.closeSession(srv.ID)
		return nil
	}
	bindings := m.bindingsFor(srv)
	m.syncer.ReplaceMCPServer(srv.ID, bindings, srv.AmbientMount)
	return nil
}

func (m *MCPManager) bindingsFor(srv domain.MCPServer) []domain.MCPToolBinding {
	var out []domain.MCPToolBinding
	for _, t := range srv.DiscoveredTools {
		if !t.Enabled {
			continue
		}
		out = append(out, domain.MCPToolBinding{
			ServerID:    srv.ID,
			ServerName:  srv.Name,
			ToolName:    t.Name,
			ExposedName: domain.ExposedMCPToolName(srv.Name, t.Name),
			Description: t.Description,
			InputSchema: t.InputSchema,
			RiskLevel:   domain.RiskExternal,
		})
	}
	return out
}

func (m *MCPManager) openSession(ctx context.Context, srv domain.MCPServer) (port.MCPSession, error) {
	if m.dialer == nil {
		return nil, fmt.Errorf("MCP dialer not configured")
	}
	m.mu.Lock()
	if sess, ok := m.sessions[srv.ID]; ok && sess != nil {
		m.mu.Unlock()
		return sess, nil
	}
	m.mu.Unlock()

	resolved := m.resolveAuthHeaders(ctx, srv)
	srv.Headers = resolved

	sess, err := m.dialer.Dial(ctx, srv)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if old, ok := m.sessions[srv.ID]; ok && old != nil {
		_ = old.Close()
	}
	m.sessions[srv.ID] = sess
	m.mu.Unlock()
	return sess, nil
}

func (m *MCPManager) closeSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[id]; ok && sess != nil {
		_ = sess.Close()
		delete(m.sessions, id)
	}
}

func (m *MCPManager) resolveAuthHeaders(ctx context.Context, srv domain.MCPServer) map[string]string {
	out := map[string]string{}
	for k, v := range srv.Headers {
		out[k] = v
	}
	if m.secrets == nil {
		return out
	}
	if srv.Auth == domain.MCPAuthOAuth || srv.OAuthStatus == "connected" {
		if tok, err := m.secrets.Get(ctx, secretKeyMCPOAuthAccess(srv.ID)); err == nil && tok != "" {
			out["Authorization"] = "Bearer " + tok
		}
	}
	for header, key := range srv.SecretHeadersRef {
		if key == "" {
			key = secretKeyMCPHeader(srv.ID, header)
		}
		if val, err := m.secrets.Get(ctx, key); err == nil && val != "" {
			out[header] = val
		}
	}
	return out
}

func (m *MCPManager) storeHeaderSecrets(ctx context.Context, serverID string, secrets map[string]string, srv *domain.MCPServer) error {
	if len(secrets) == 0 || m.secrets == nil {
		return nil
	}
	if srv.SecretHeadersRef == nil {
		srv.SecretHeadersRef = map[string]string{}
	}
	if srv.Auth == "" || srv.Auth == domain.MCPAuthNone {
		srv.Auth = domain.MCPAuthHeaders
	}
	for header, value := range secrets {
		if header == "" || value == "" {
			continue
		}
		key := secretKeyMCPHeader(serverID, header)
		if err := m.secrets.Put(ctx, key, value); err != nil {
			return err
		}
		srv.SecretHeadersRef[header] = key
		if srv.Headers != nil {
			delete(srv.Headers, header)
		}
	}
	return nil
}

func (m *MCPManager) BeginOAuth(ctx context.Context, id, redirectURI string) (authorizeURL string, state string, err error) {
	srv, err := m.Get(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("MCP server not found: %w", err)
	}
	if srv.OAuthAuthorizeURL == "" {
		return "", "", fmt.Errorf("oauth authorize URL not configured")
	}
	state = fmt.Sprintf("mcp-%s-%d", id, time.Now().UnixNano())
	if m.secrets != nil {
		_ = m.secrets.Put(ctx, secretKeyMCPOAuthState(id), state)
		if redirectURI != "" {
			_ = m.secrets.Put(ctx, secretKeyMCPOAuthRedirect(id), redirectURI)
		}
	}
	srv.Auth = domain.MCPAuthOAuth
	srv.OAuthStatus = "pending"
	m.mu.Lock()
	m.servers[id] = srv
	m.mu.Unlock()

	sep := "?"
	if containsQuery(srv.OAuthAuthorizeURL) {
		sep = "&"
	}
	url := srv.OAuthAuthorizeURL + sep + "response_type=code&state=" + state
	if srv.OAuthClientID != "" {
		url += "&client_id=" + srv.OAuthClientID
	}
	if redirectURI != "" {
		url += "&redirect_uri=" + redirectURI
	}
	if srv.OAuthScopes != "" {
		url += "&scope=" + srv.OAuthScopes
	}
	return url, state, nil
}

func (m *MCPManager) CompleteOAuth(ctx context.Context, id, code, state, accessToken string) (domain.MCPServer, error) {
	srv, err := m.Get(ctx, id)
	if err != nil {
		return domain.MCPServer{}, fmt.Errorf("MCP server not found: %w", err)
	}
	if m.secrets != nil && state != "" {
		expected, _ := m.secrets.Get(ctx, secretKeyMCPOAuthState(id))
		if expected != "" && expected != state {
			return domain.MCPServer{}, fmt.Errorf("oauth state mismatch")
		}
	}
	token := accessToken
	if token == "" && code != "" && srv.OAuthTokenURL != "" {
		token = code
	}
	if token == "" {
		return domain.MCPServer{}, fmt.Errorf("access token or code required")
	}
	if m.secrets != nil {
		if err := m.secrets.Put(ctx, secretKeyMCPOAuthAccess(id), token); err != nil {
			return domain.MCPServer{}, err
		}
	}
	srv.Auth = domain.MCPAuthOAuth
	srv.OAuthStatus = "connected"
	m.closeSession(id)
	m.mu.Lock()
	m.servers[id] = srv
	m.mu.Unlock()
	_ = m.syncServer(ctx, srv)
	return srv, nil
}

func normalizeMCPServer(s *domain.MCPServer) {
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
	if s.Auth == "" {
		s.Auth = domain.MCPAuthNone
	}
}

func rebuildEnabledTools(srv *domain.MCPServer) {
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
}

func containsQuery(u string) bool {
	for i := 0; i < len(u); i++ {
		if u[i] == '?' {
			return true
		}
	}
	return false
}

func secretPrefixMCP(serverID string) string { return "mcp/" + serverID + "/" }

func secretKeyMCPHeader(serverID, header string) string {
	return secretPrefixMCP(serverID) + "header/" + header
}

func secretKeyMCPOAuthAccess(serverID string) string {
	return secretPrefixMCP(serverID) + "oauth/access_token"
}

func secretKeyMCPOAuthState(serverID string) string {
	return secretPrefixMCP(serverID) + "oauth/state"
}

func secretKeyMCPOAuthRedirect(serverID string) string {
	return secretPrefixMCP(serverID) + "oauth/redirect"
}

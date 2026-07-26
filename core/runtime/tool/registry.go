package tool

import (
	"sync"

	"danmo-work/core/domain"
)

type Registry struct {
	mu         sync.RWMutex
	byName     map[string]Handler
	order      []Handler
	mcpServers map[string][]Handler
	mounted    map[string]struct{}
}

func NewRegistry(handlers ...Handler) *Registry {
	r := &Registry{
		byName:     make(map[string]Handler),
		mcpServers: make(map[string][]Handler),
		mounted:    make(map[string]struct{}),
	}
	for _, h := range handlers {
		r.Register(h)
	}
	return r
}

func (r *Registry) Register(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerLocked(h)
}

func (r *Registry) registerLocked(h Handler) {
	if _, ok := r.byName[h.Name()]; !ok {
		r.order = append(r.order, h)
	}
	r.byName[h.Name()] = h
}

func (r *Registry) RegisterServer(serverID string, tools ...Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcpServers[serverID] = append(r.mcpServers[serverID], tools...)
}

// ReplaceServer replaces all tools for a server id (empty clears).
func (r *Registry) ReplaceServer(serverID string, tools ...Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(tools) == 0 {
		delete(r.mcpServers, serverID)
		return
	}
	r.mcpServers[serverID] = append([]Handler(nil), tools...)
}

// RemoveServer drops a server's tool list from the catalog.
func (r *Registry) RemoveServer(serverID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.mcpServers, serverID)
}

// MountAllMCP mounts every registered MCP server's tools into the active set.
func (r *Registry) MountAllMCP() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for serverID, handlers := range r.mcpServers {
		r.mounted[serverID] = struct{}{}
		for _, h := range handlers {
			r.registerLocked(h)
		}
	}
}

func (r *Registry) CopyMCPServersFrom(src *Registry) {
	if src == nil {
		return
	}
	src.mu.RLock()
	defer src.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, handlers := range src.mcpServers {
		r.mcpServers[id] = handlers
	}
}

func (r *Registry) Mount(serverID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mounted[serverID] = struct{}{}
	for _, h := range r.mcpServers[serverID] {
		r.registerLocked(h)
	}
}

func (r *Registry) MountFromBindings(bindings []domain.ToolBinding) {
	for _, b := range bindings {
		if b.MCPServer != "" {
			r.Mount(b.MCPServer)
		}
	}
}

func (r *Registry) Get(name string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.byName[name]
	return h, ok
}

func (r *Registry) List() []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Handler, len(r.order))
	copy(out, r.order)
	return out
}

func (r *Registry) Schemas() []domain.ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.ToolSchema, 0, len(r.order))
	for _, h := range r.order {
		s := h.Schema()
		s.RiskLevel = h.RiskLevel()
		out = append(out, s)
	}
	return out
}

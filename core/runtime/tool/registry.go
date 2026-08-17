package tool

import (
	"sort"
	"sync"

	"danmo-work/core/domain"
)

type Registry struct {
	mu         sync.RWMutex
	byName     map[string]Handler
	order      []Handler
	mcpServers map[string][]Handler
	// mcpAmbient[id]==false → skip MountAllMCP (bound-only connectors).
	mcpAmbient map[string]bool
	mounted    map[string]struct{}
}

func NewRegistry(handlers ...Handler) *Registry {
	r := &Registry{
		byName:     make(map[string]Handler),
		mcpServers: make(map[string][]Handler),
		mcpAmbient: make(map[string]bool),
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
	r.RegisterServerAmbient(serverID, true, tools...)
}

// RegisterServerAmbient registers tools and ambient eligibility for a server id.
func (r *Registry) RegisterServerAmbient(serverID string, ambientMount bool, tools ...Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcpServers[serverID] = append(r.mcpServers[serverID], tools...)
	r.mcpAmbient[serverID] = ambientMount
}

// ReplaceServer replaces all tools for a server id (empty clears). Ambient defaults true.
func (r *Registry) ReplaceServer(serverID string, tools ...Handler) {
	r.ReplaceServerAmbient(serverID, true, tools...)
}

// ReplaceServerAmbient replaces tools and sets whether MountAllMCP includes this server.
func (r *Registry) ReplaceServerAmbient(serverID string, ambientMount bool, tools ...Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(tools) == 0 {
		delete(r.mcpServers, serverID)
		delete(r.mcpAmbient, serverID)
		return
	}
	r.mcpServers[serverID] = append([]Handler(nil), tools...)
	r.mcpAmbient[serverID] = ambientMount
}

// RemoveServer drops a server's tool list from the catalog.
func (r *Registry) RemoveServer(serverID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.mcpServers, serverID)
	delete(r.mcpAmbient, serverID)
}

// MountAllMCP mounts ambient-eligible MCP servers (skips ambientMount=false).
func (r *Registry) MountAllMCP() {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.mcpServers))
	for serverID := range r.mcpServers {
		ids = append(ids, serverID)
	}
	sort.Strings(ids)
	for _, serverID := range ids {
		handlers := r.mcpServers[serverID]
		if ambient, ok := r.mcpAmbient[serverID]; ok && !ambient {
			continue
		}
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
	for id, ambient := range src.mcpAmbient {
		r.mcpAmbient[id] = ambient
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

// MountServers mounts tools for the given MCP server ids (exact match only).
func (r *Registry) MountServers(serverIDs []string) {
	for _, id := range serverIDs {
		if id == "" {
			continue
		}
		r.Mount(id)
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

// Filter keeps only tools whose names are in the allowed set, dropping all
// others from the registry. It also prunes MCP servers whose tools are no
// longer represented so subsequent Mount* calls cannot re-add them.
func (r *Registry) Filter(allowed map[string]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	newOrder := r.order[:0]
	newByName := make(map[string]Handler, len(r.byName))
	for _, h := range r.order {
		if _, ok := allowed[h.Name()]; ok {
			newOrder = append(newOrder, h)
			newByName[h.Name()] = h
		}
	}
	r.order = newOrder
	r.byName = newByName

	for sid, handlers := range r.mcpServers {
		filtered := make([]Handler, 0, len(handlers))
		for _, h := range handlers {
			if _, ok := allowed[h.Name()]; ok {
				filtered = append(filtered, h)
			}
		}
		if len(filtered) == 0 {
			delete(r.mcpServers, sid)
			delete(r.mcpAmbient, sid)
			delete(r.mounted, sid)
		} else {
			r.mcpServers[sid] = filtered
		}
	}
}

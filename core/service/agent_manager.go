package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type AgentManager struct {
	store          port.AgentRepo
	mu             sync.RWMutex
	cache          map[string]*domain.Agent
	templateLoader func(id string) (*domain.Agent, error)
}

func NewAgentManager(store port.AgentRepo) *AgentManager {
	return &AgentManager{store: store, cache: make(map[string]*domain.Agent)}
}

func (m *AgentManager) SetTemplateLoader(fn func(id string) (*domain.Agent, error)) {
	m.templateLoader = fn
}

func (m *AgentManager) enrich(a *domain.Agent) {
	if a == nil {
		return
	}
	body, meta := DecodeAgentSystemPrompt(a.SystemPrompt)
	a.SystemPrompt = body
	a.MarketSource = marketSourceFromMeta(meta)
	if m.templateLoader != nil {
		if _, err := m.templateLoader(a.ID); err == nil {
			a.Builtin = true
		}
	}
}

func (m *AgentManager) prepareStore(a domain.Agent) domain.Agent {
	body, meta := DecodeAgentSystemPrompt(a.SystemPrompt)
	if meta == nil {
		meta = map[string]string{}
	}
	if a.MarketSource != "" {
		meta["market.source"] = a.MarketSource
	}
	a.SystemPrompt = EncodeAgentSystemPrompt(body, meta)
	a.Builtin = false
	a.MarketSource = ""
	return a
}

func (m *AgentManager) Get(ctx context.Context, id string) (*domain.Agent, error) {
	m.mu.RLock()
	if a, ok := m.cache[id]; ok {
		m.mu.RUnlock()
		cp := *a
		m.enrich(&cp)
		return &cp, nil
	}
	m.mu.RUnlock()
	a, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.cache[id] = &a
	m.mu.Unlock()
	cp := a
	m.enrich(&cp)
	return &cp, nil
}

func (m *AgentManager) List(ctx context.Context) ([]domain.Agent, error) {
	list, err := m.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Agent, len(list))
	for i := range list {
		out[i] = list[i]
		m.enrich(&out[i])
	}
	return out, nil
}

func (m *AgentManager) Upsert(ctx context.Context, a domain.Agent) error {
	stored := m.prepareStore(a)
	if err := m.store.Upsert(ctx, stored); err != nil {
		return err
	}
	m.mu.Lock()
	m.cache[a.ID] = &stored
	m.mu.Unlock()
	return nil
}

func (m *AgentManager) Delete(ctx context.Context, id string) error {
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.cache, id)
	m.mu.Unlock()
	return nil
}

func (m *AgentManager) ResetFromTemplate(ctx context.Context, id string) (*domain.Agent, error) {
	if m.templateLoader == nil {
		return nil, fmt.Errorf("template loader not configured")
	}
	_, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent %q not found, cannot reset: %w", id, err)
	}
	tmpl, err := m.templateLoader(id)
	if err != nil {
		return nil, fmt.Errorf("no template found for agent %q: %w", id, err)
	}
	tmpl.MarketSource = ""
	body, _ := DecodeAgentSystemPrompt(tmpl.SystemPrompt)
	stored := *tmpl
	stored.SystemPrompt = strings.TrimSpace(body)
	if err := m.store.Upsert(ctx, stored); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.cache[id] = &stored
	m.mu.Unlock()
	out := stored
	m.enrich(&out)
	return &out, nil
}

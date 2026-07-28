package market

import (
	"strings"

	"danmo-work/core/adapter/market/clawhub"
	gitmarket "danmo-work/core/adapter/market/git"
	"danmo-work/core/adapter/market/techleads"
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// Registry builds Market adapters from config sources (git, clawhub, …).
type Registry struct {
	markets []port.Market
	byID    map[string]port.Market
}

func NewRegistry(sources []domain.MarketSource) *Registry {
	r := &Registry{byID: make(map[string]port.Market)}
	_ = r.Reload(sources)
	return r
}

func (r *Registry) List() []port.Market {
	out := make([]port.Market, len(r.markets))
	copy(out, r.markets)
	return out
}

func (r *Registry) Get(sourceID string) (port.Market, bool) {
	m, ok := r.byID[sourceID]
	return m, ok
}

func (r *Registry) Reload(sources []domain.MarketSource) error {
	r.markets = nil
	r.byID = make(map[string]port.Market)
	for _, src := range sources {
		if !src.Enabled || src.ID == "" {
			continue
		}
		m := newAdapter(src)
		if m == nil {
			continue
		}
		r.markets = append(r.markets, m)
		r.byID[src.ID] = m
	}
	return nil
}

func newAdapter(src domain.MarketSource) port.Market {
	kind := strings.ToLower(strings.TrimSpace(src.Kind))
	switch kind {
	case "", "git":
		return gitmarket.New(src)
	case "clawhub":
		return clawhub.New(src)
	case "techleads", "tlc", "tech-leads-club":
		return techleads.New(src)
	default:
		return nil
	}
}

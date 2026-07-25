package service

import (
	"context"
	"fmt"
	"sync"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// WeixinPeerStore adapts Weixin SQLite repos to ChannelPeerStore.
type WeixinPeerStore struct {
	store port.Repository
}

func NewWeixinPeerStore(store port.Repository) *WeixinPeerStore {
	return &WeixinPeerStore{store: store}
}

func (s *WeixinPeerStore) GetProjectID(ctx context.Context, channel port.ChannelType, accountID string) (string, error) {
	if channel != port.ChannelWeixin {
		return "", fmt.Errorf("weixin peer store: unsupported channel %s", channel)
	}
	acc, err := s.store.WeixinAccounts().Get(ctx, accountID)
	if err != nil {
		return "", err
	}
	return acc.ProjectID, nil
}

func (s *WeixinPeerStore) GetBinding(ctx context.Context, channel port.ChannelType, accountID, peerID string) (string, map[string]string, error) {
	if channel != port.ChannelWeixin {
		return "", nil, fmt.Errorf("weixin peer store: unsupported channel %s", channel)
	}
	b, err := s.store.WeixinBindings().GetByPeer(ctx, accountID, peerID)
	if err != nil {
		return "", nil, err
	}
	meta := map[string]string{}
	if b.ContextToken != "" {
		meta["context_token"] = b.ContextToken
	}
	return b.SessionID, meta, nil
}

func (s *WeixinPeerStore) UpsertBinding(ctx context.Context, channel port.ChannelType, accountID, peerID, sessionID string, meta map[string]string) error {
	if channel != port.ChannelWeixin {
		return fmt.Errorf("weixin peer store: unsupported channel %s", channel)
	}
	tok := ""
	if meta != nil {
		tok = meta["context_token"]
	}
	return s.store.WeixinBindings().Upsert(ctx, domain.WeixinBinding{
		AccountID:    accountID,
		PeerUserID:   peerID,
		SessionID:    sessionID,
		ContextToken: tok,
	})
}

func (s *WeixinPeerStore) UpdateBindingMeta(ctx context.Context, channel port.ChannelType, accountID, peerID string, meta map[string]string) error {
	if channel != port.ChannelWeixin {
		return fmt.Errorf("weixin peer store: unsupported channel %s", channel)
	}
	if meta == nil {
		return nil
	}
	if tok, ok := meta["context_token"]; ok {
		return s.store.WeixinBindings().UpdateContextToken(ctx, accountID, peerID, tok)
	}
	return nil
}

// MultiplexPeerStore routes ChannelPeerStore calls by channel type.
type MultiplexPeerStore struct {
	byType map[port.ChannelType]port.ChannelPeerStore
}

func NewMultiplexPeerStore(stores map[port.ChannelType]port.ChannelPeerStore) *MultiplexPeerStore {
	return &MultiplexPeerStore{byType: stores}
}

func (m *MultiplexPeerStore) store(ch port.ChannelType) (port.ChannelPeerStore, error) {
	s, ok := m.byType[ch]
	if !ok || s == nil {
		return nil, fmt.Errorf("no peer store for channel %s", ch)
	}
	return s, nil
}

func (m *MultiplexPeerStore) GetProjectID(ctx context.Context, channel port.ChannelType, accountID string) (string, error) {
	s, err := m.store(channel)
	if err != nil {
		return "", err
	}
	return s.GetProjectID(ctx, channel, accountID)
}

func (m *MultiplexPeerStore) GetBinding(ctx context.Context, channel port.ChannelType, accountID, peerID string) (string, map[string]string, error) {
	s, err := m.store(channel)
	if err != nil {
		return "", nil, err
	}
	return s.GetBinding(ctx, channel, accountID, peerID)
}

func (m *MultiplexPeerStore) UpsertBinding(ctx context.Context, channel port.ChannelType, accountID, peerID, sessionID string, meta map[string]string) error {
	s, err := m.store(channel)
	if err != nil {
		return err
	}
	return s.UpsertBinding(ctx, channel, accountID, peerID, sessionID, meta)
}

func (m *MultiplexPeerStore) UpdateBindingMeta(ctx context.Context, channel port.ChannelType, accountID, peerID string, meta map[string]string) error {
	s, err := m.store(channel)
	if err != nil {
		return err
	}
	return s.UpdateBindingMeta(ctx, channel, accountID, peerID, meta)
}

// ConfigChannelDefaults reads defaults from YAML channel sections.
type ConfigChannelDefaults struct {
	config *ConfigManager
}

func NewConfigChannelDefaults(config *ConfigManager) *ConfigChannelDefaults {
	return &ConfigChannelDefaults{config: config}
}

func (d *ConfigChannelDefaults) ChannelDefaults(ctx context.Context, channel port.ChannelType) (port.ChannelDefaults, error) {
	cfg, err := d.config.Get(ctx)
	if err != nil {
		return port.ChannelDefaults{}, err
	}
	switch channel {
	case port.ChannelWeixin:
		wx := cfg.Channels.Weixin
		return port.ChannelDefaults{
			AgentID:     wx.DefaultAgentID,
			ModelID:     wx.DefaultModelID,
			AutoApprove: wx.AutoApprove,
		}, nil
	case port.ChannelFeishu:
		fs := cfg.Channels.Feishu
		return port.ChannelDefaults{
			AgentID:     fs.DefaultAgentID,
			ModelID:     fs.DefaultModelID,
			AutoApprove: fs.AutoApprove,
		}, nil
	case port.ChannelWecom:
		wc := cfg.Channels.Wecom
		return port.ChannelDefaults{
			AgentID:     wc.DefaultAgentID,
			ModelID:     wc.DefaultModelID,
			AutoApprove: wc.AutoApprove,
		}, nil
	case port.ChannelQQ:
		qc := cfg.Channels.QQ
		return port.ChannelDefaults{
			AgentID:     qc.DefaultAgentID,
			ModelID:     qc.DefaultModelID,
			AutoApprove: qc.AutoApprove,
		}, nil
	default:
		return port.ChannelDefaults{}, fmt.Errorf("unknown channel %s", channel)
	}
}

// ChannelManager registers channel runtimes (WebSocket / long-poll).
type ChannelManager struct {
	mu       sync.RWMutex
	runtimes map[port.ChannelType]port.ChannelRuntime
	Ingress  port.ChannelIngress
}

func NewChannelManager(ingress port.ChannelIngress) *ChannelManager {
	return &ChannelManager{
		runtimes: make(map[port.ChannelType]port.ChannelRuntime),
		Ingress:  ingress,
	}
}

func (m *ChannelManager) RegisterRuntime(r port.ChannelRuntime) {
	if r == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runtimes[r.Type()] = r
}

func (m *ChannelManager) Runtime(t port.ChannelType) port.ChannelRuntime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runtimes[t]
}

func (m *ChannelManager) SyncAll(ctx context.Context) error {
	m.mu.RLock()
	rts := make([]port.ChannelRuntime, 0, len(m.runtimes))
	for _, r := range m.runtimes {
		rts = append(rts, r)
	}
	m.mu.RUnlock()
	var first error
	for _, r := range rts {
		if err := r.SyncFromConfig(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *ChannelManager) StopAll() {
	m.mu.RLock()
	rts := make([]port.ChannelRuntime, 0, len(m.runtimes))
	for _, r := range m.runtimes {
		rts = append(rts, r)
	}
	m.mu.RUnlock()
	for _, r := range rts {
		r.Stop()
	}
}

package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/adapter/wecom"
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// WecomPeerStore uses config project_id + generic channel_bindings table.
type WecomPeerStore struct {
	store  port.Repository
	config *ConfigManager
}

func NewWecomPeerStore(store port.Repository, config *ConfigManager) *WecomPeerStore {
	return &WecomPeerStore{store: store, config: config}
}

func (s *WecomPeerStore) GetProjectID(ctx context.Context, channel port.ChannelType, accountID string) (string, error) {
	if channel != port.ChannelWecom {
		return "", fmt.Errorf("wecom peer store: unsupported channel %s", channel)
	}
	cfg, err := s.config.Get(ctx)
	if err != nil {
		return "", err
	}
	return cfg.Channels.Wecom.ProjectID, nil
}

func (s *WecomPeerStore) GetBinding(ctx context.Context, channel port.ChannelType, accountID, peerID string) (string, map[string]string, error) {
	b, err := s.store.ChannelBindings().GetByPeer(ctx, string(channel), accountID, peerID)
	if err != nil {
		return "", nil, err
	}
	return b.SessionID, b.Meta, nil
}

func (s *WecomPeerStore) UpsertBinding(ctx context.Context, channel port.ChannelType, accountID, peerID, sessionID string, meta map[string]string) error {
	return s.store.ChannelBindings().Upsert(ctx, domain.ChannelBinding{
		ChannelType: string(channel),
		AccountID:   accountID,
		PeerID:      peerID,
		SessionID:   sessionID,
		Meta:        meta,
	})
}

func (s *WecomPeerStore) UpdateBindingMeta(ctx context.Context, channel port.ChannelType, accountID, peerID string, meta map[string]string) error {
	_, existing, err := s.GetBinding(ctx, channel, accountID, peerID)
	if err != nil {
		return err
	}
	merged := map[string]string{}
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range meta {
		merged[k] = v
	}
	return s.store.ChannelBindings().UpdateMeta(ctx, string(channel), accountID, peerID, merged)
}

// WecomBridge runs WeCom AI Bot WebSocket and routes messages through ChannelIngress.
type WecomBridge struct {
	config   *ConfigManager
	ingress  port.ChannelIngress
	endpoint *WecomEndpoint

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewWecomBridge(config *ConfigManager, ingress port.ChannelIngress) *WecomBridge {
	ep := NewWecomEndpoint()
	b := &WecomBridge{config: config, ingress: ingress, endpoint: ep}
	if ingress != nil {
		ingress.RegisterEndpoint(ep)
	}
	return b
}

// Endpoint returns the registered ChannelEndpoint (tests / diagnostics).
func (b *WecomBridge) Endpoint() port.ChannelEndpoint { return b.endpoint }

func (b *WecomBridge) Type() port.ChannelType { return port.ChannelWecom }

func (b *WecomBridge) SyncFromConfig(ctx context.Context) error {
	cfg, err := b.config.Get(ctx)
	if err != nil {
		return err
	}
	wc := cfg.Channels.Wecom
	if !wc.Enabled {
		b.Stop()
		return nil
	}
	if err := validateWecomEnabled(wc); err != nil {
		b.Stop()
		return err
	}
	b.Stop()
	return b.Start(ctx)
}

func validateWecomEnabled(wc domain.ConfigWecomChannel) error {
	if wc.DefaultAgentID == "" {
		return fmt.Errorf("channels.wecom.default_agent_id required when enabled")
	}
	if strings.TrimSpace(wc.DefaultModelID) == "" || !strings.Contains(wc.DefaultModelID, "/") {
		return fmt.Errorf("channels.wecom.default_model_id required when enabled (provider/model)")
	}
	if strings.TrimSpace(wc.ProjectID) == "" {
		return fmt.Errorf("channels.wecom.project_id required when enabled")
	}
	if wc.BotID == "" || wc.Secret == "" {
		return fmt.Errorf("channels.wecom.bot_id/secret required when enabled")
	}
	return nil
}

func (b *WecomBridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return nil
	}
	cfg, err := b.config.Get(ctx)
	if err != nil {
		b.mu.Unlock()
		return err
	}
	wc := cfg.Channels.Wecom
	runCtx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	b.mu.Unlock()

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.runWSLoop(runCtx, wc)
	}()
	log.Printf("[wecom] websocket bridge started bot=%s", wc.BotID)
	return nil
}

func (b *WecomBridge) runWSLoop(ctx context.Context, wc domain.ConfigWecomChannel) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		var lc *wecom.LongConn
		lc = wecom.NewLongConn(wc, func(msgCtx context.Context, msg port.InboundMessage) error {
			return b.handleInbound(msgCtx, msg)
		})
		if b.endpoint != nil {
			b.endpoint.SetConn(lc)
		}
		err := lc.Run(ctx)
		if b.endpoint != nil {
			b.endpoint.ClearConn(lc)
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("[wecom] websocket exited: %v; reconnect in %s", err, backoff)
		} else {
			log.Printf("[wecom] websocket exited; reconnect in %s", backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
		if cfg, err := b.config.Get(ctx); err == nil {
			wc = cfg.Channels.Wecom
			if !wc.Enabled {
				return
			}
		}
	}
}

func (b *WecomBridge) handleInbound(ctx context.Context, msg port.InboundMessage) error {
	if b.ingress == nil {
		if b.endpoint != nil {
			_ = b.endpoint.Deliver(ctx, &msg, port.TextOutbound("企业微信通道未就绪"))
		}
		return fmt.Errorf("wecom ingress not configured")
	}
	reply, err := b.ingress.HandleInbound(ctx, msg)
	if err != nil {
		log.Printf("[wecom] handle inbound peer=%s: %v", msg.PeerID, err)
		if b.endpoint != nil {
			_ = b.endpoint.Deliver(ctx, &msg, port.TextOutbound("处理消息时出错："+err.Error()))
		}
		return err
	}
	// Ingress delivers via endpoint; reply non-empty only if endpoint missing.
	if strings.TrimSpace(reply) != "" && b.endpoint != nil {
		_ = b.endpoint.Deliver(ctx, &msg, port.TextOutbound(reply))
	}
	return nil
}

func (b *WecomBridge) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	if b.cancel != nil {
		b.cancel()
	}
	b.running = false
	b.mu.Unlock()
	b.wg.Wait()
	log.Printf("[wecom] websocket bridge stopped")
}

func (b *WecomBridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

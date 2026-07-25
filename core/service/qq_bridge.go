package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/adapter/qq"
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// QQPeerStore uses config project_id + generic channel_bindings table.
type QQPeerStore struct {
	store  port.Repository
	config *ConfigManager
}

func NewQQPeerStore(store port.Repository, config *ConfigManager) *QQPeerStore {
	return &QQPeerStore{store: store, config: config}
}

func (s *QQPeerStore) GetProjectID(ctx context.Context, channel port.ChannelType, accountID string) (string, error) {
	if channel != port.ChannelQQ {
		return "", fmt.Errorf("qq peer store: unsupported channel %s", channel)
	}
	cfg, err := s.config.Get(ctx)
	if err != nil {
		return "", err
	}
	return cfg.Channels.QQ.ProjectID, nil
}

func (s *QQPeerStore) GetBinding(ctx context.Context, channel port.ChannelType, accountID, peerID string) (string, map[string]string, error) {
	b, err := s.store.ChannelBindings().GetByPeer(ctx, string(channel), accountID, peerID)
	if err != nil {
		return "", nil, err
	}
	return b.SessionID, b.Meta, nil
}

func (s *QQPeerStore) UpsertBinding(ctx context.Context, channel port.ChannelType, accountID, peerID, sessionID string, meta map[string]string) error {
	return s.store.ChannelBindings().Upsert(ctx, domain.ChannelBinding{
		ChannelType: string(channel),
		AccountID:   accountID,
		PeerID:      peerID,
		SessionID:   sessionID,
		Meta:        meta,
	})
}

func (s *QQPeerStore) UpdateBindingMeta(ctx context.Context, channel port.ChannelType, accountID, peerID string, meta map[string]string) error {
	_, existing, err := s.GetBinding(ctx, channel, accountID, peerID)
	if err != nil {
		return err
	}
	return s.store.ChannelBindings().UpdateMeta(ctx, string(channel), accountID, peerID, mergeStringMap(existing, meta))
}

// QQBridge runs QQ Bot Gateway WebSocket and routes through ChannelIngress.
type QQBridge struct {
	config   *ConfigManager
	adapter  *qq.Adapter
	ingress  port.ChannelIngress
	endpoint *QQEndpoint

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewQQBridge(config *ConfigManager, adapter *qq.Adapter, ingress port.ChannelIngress) *QQBridge {
	ep := NewQQEndpoint(adapter)
	b := &QQBridge{config: config, adapter: adapter, ingress: ingress, endpoint: ep}
	if ingress != nil {
		ingress.RegisterEndpoint(ep)
	}
	return b
}

func (b *QQBridge) Endpoint() port.ChannelEndpoint { return b.endpoint }

func (b *QQBridge) Type() port.ChannelType { return port.ChannelQQ }

func (b *QQBridge) Adapter() *qq.Adapter { return b.adapter }

func (b *QQBridge) SyncFromConfig(ctx context.Context) error {
	cfg, err := b.config.Get(ctx)
	if err != nil {
		return err
	}
	qc := cfg.Channels.QQ
	b.adapter.UpdateConfig(qc)
	if !qc.Enabled {
		b.Stop()
		return nil
	}
	if err := validateQQEnabled(qc); err != nil {
		b.Stop()
		return err
	}
	b.Stop()
	return b.Start(ctx)
}

func validateQQEnabled(qc domain.ConfigQQChannel) error {
	if qc.DefaultAgentID == "" {
		return fmt.Errorf("channels.qq.default_agent_id required when enabled")
	}
	if strings.TrimSpace(qc.DefaultModelID) == "" || !strings.Contains(qc.DefaultModelID, "/") {
		return fmt.Errorf("channels.qq.default_model_id required when enabled (provider/model)")
	}
	if strings.TrimSpace(qc.ProjectID) == "" {
		return fmt.Errorf("channels.qq.project_id required when enabled")
	}
	if qc.AppID == "" || qc.ClientSecret == "" {
		return fmt.Errorf("channels.qq.app_id/client_secret required when enabled")
	}
	return nil
}

func (b *QQBridge) Start(ctx context.Context) error {
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
	qc := cfg.Channels.QQ
	runCtx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	b.mu.Unlock()

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.runWSLoop(runCtx, qc)
	}()
	log.Printf("[qq] gateway bridge started app=%s", qc.AppID)
	return nil
}

func (b *QQBridge) runWSLoop(ctx context.Context, qc domain.ConfigQQChannel) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		lc := qq.NewLongConn(b.adapter, qc, b.handleInbound, b.handleInteraction)
		err := lc.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("[qq] gateway exited: %v; reconnect in %s", err, backoff)
		} else {
			log.Printf("[qq] gateway exited; reconnect in %s", backoff)
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
			qc = cfg.Channels.QQ
			b.adapter.UpdateConfig(qc)
			if !qc.Enabled {
				return
			}
		}
	}
}

func (b *QQBridge) handleInbound(msg port.InboundMessage) error {
	if b.ingress == nil {
		return fmt.Errorf("qq ingress not configured")
	}
	ctx := context.Background()
	reply, err := b.ingress.HandleInbound(ctx, msg)
	if err != nil {
		log.Printf("[qq] handle inbound peer=%s: %v", msg.PeerID, err)
		errText := "处理消息时出错：" + err.Error()
		if b.endpoint != nil {
			_ = b.endpoint.Deliver(ctx, &msg, port.TextOutbound(errText))
			return err
		}
		reply = errText
	}
	if strings.TrimSpace(reply) == "" {
		return nil
	}
	return b.adapter.DeliverOutbound(ctx, &msg, port.TextOutbound(reply))
}

func (b *QQBridge) handleInteraction(ev port.InteractionEvent, interactionID string) error {
	ctx := context.Background()
	if b.adapter != nil && interactionID != "" {
		_ = b.adapter.AckInteraction(ctx, interactionID)
	}
	if b.ingress == nil {
		return fmt.Errorf("qq ingress not configured")
	}
	// Decode compact token from Raw (button_data).
	base := port.InboundMessage{
		Type:      ev.Type,
		AccountID: ev.AccountID,
		PeerID:    ev.PeerID,
		ChatID:    ev.ChatID,
		MessageID: ev.MessageID,
		Meta:      ev.Meta,
	}
	decoded, ok := InteractionFromCallback(base, ev.Raw)
	if !ok {
		log.Printf("[qq] interaction: unrecognized data %q", ev.Raw)
		return nil
	}
	return b.ingress.HandleInteraction(ctx, decoded)
}

func (b *QQBridge) Stop() {
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
	log.Printf("[qq] gateway bridge stopped")
}

func (b *QQBridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

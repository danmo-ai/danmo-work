package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/adapter/feishu"
	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// FeishuPeerStore uses config project_id + generic channel_bindings table.
type FeishuPeerStore struct {
	store  port.Repository
	config *ConfigManager
}

func NewFeishuPeerStore(store port.Repository, config *ConfigManager) *FeishuPeerStore {
	return &FeishuPeerStore{store: store, config: config}
}

func (s *FeishuPeerStore) GetProjectID(ctx context.Context, channel port.ChannelType, accountID string) (string, error) {
	if channel != port.ChannelFeishu {
		return "", fmt.Errorf("feishu peer store: unsupported channel %s", channel)
	}
	cfg, err := s.config.Get(ctx)
	if err != nil {
		return "", err
	}
	return cfg.Channels.Feishu.ProjectID, nil
}

func (s *FeishuPeerStore) GetBinding(ctx context.Context, channel port.ChannelType, accountID, peerID string) (string, map[string]string, error) {
	b, err := s.store.ChannelBindings().GetByPeer(ctx, string(channel), accountID, peerID)
	if err != nil {
		return "", nil, err
	}
	return b.SessionID, b.Meta, nil
}

func (s *FeishuPeerStore) UpsertBinding(ctx context.Context, channel port.ChannelType, accountID, peerID, sessionID string, meta map[string]string) error {
	return s.store.ChannelBindings().Upsert(ctx, domain.ChannelBinding{
		ChannelType: string(channel),
		AccountID:   accountID,
		PeerID:      peerID,
		SessionID:   sessionID,
		Meta:        meta,
	})
}

func (s *FeishuPeerStore) UpdateBindingMeta(ctx context.Context, channel port.ChannelType, accountID, peerID string, meta map[string]string) error {
	_, existing, err := s.GetBinding(ctx, channel, accountID, peerID)
	if err != nil {
		return err
	}
	return s.store.ChannelBindings().UpdateMeta(ctx, string(channel), accountID, peerID, mergeStringMap(existing, meta))
}

// FeishuBridge runs Feishu outbound WebSocket and routes messages through ChannelIngress.
type FeishuBridge struct {
	config   *ConfigManager
	adapter  *feishu.Adapter
	ingress  port.ChannelIngress
	endpoint *FeishuEndpoint

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewFeishuBridge(config *ConfigManager, adapter *feishu.Adapter, ingress port.ChannelIngress) *FeishuBridge {
	ep := NewFeishuEndpoint(adapter)
	b := &FeishuBridge{config: config, adapter: adapter, ingress: ingress, endpoint: ep}
	if ingress != nil {
		ingress.RegisterEndpoint(ep)
	}
	return b
}

// Endpoint returns the registered ChannelEndpoint (tests / diagnostics).
func (b *FeishuBridge) Endpoint() port.ChannelEndpoint { return b.endpoint }

func (b *FeishuBridge) Type() port.ChannelType { return port.ChannelFeishu }

func (b *FeishuBridge) Adapter() *feishu.Adapter { return b.adapter }

func (b *FeishuBridge) SyncFromConfig(ctx context.Context) error {
	cfg, err := b.config.Get(ctx)
	if err != nil {
		return err
	}
	fs := cfg.Channels.Feishu
	b.adapter.UpdateConfig(fs)
	if !fs.Enabled {
		b.Stop()
		return nil
	}
	if err := validateFeishuEnabled(fs); err != nil {
		b.Stop()
		return err
	}
	b.Stop()
	return b.Start(ctx)
}

func validateFeishuEnabled(fs domain.ConfigFeishuChannel) error {
	if fs.DefaultAgentID == "" {
		return fmt.Errorf("channels.feishu.default_agent_id required when enabled")
	}
	if strings.TrimSpace(fs.DefaultModelID) == "" || !strings.Contains(fs.DefaultModelID, "/") {
		return fmt.Errorf("channels.feishu.default_model_id required when enabled (provider/model)")
	}
	if strings.TrimSpace(fs.ProjectID) == "" {
		return fmt.Errorf("channels.feishu.project_id required when enabled")
	}
	if fs.AppID == "" || fs.AppSecret == "" {
		return fmt.Errorf("channels.feishu.app_id/app_secret required when enabled")
	}
	return nil
}

func (b *FeishuBridge) Start(ctx context.Context) error {
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
	fs := cfg.Channels.Feishu
	runCtx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	b.mu.Unlock()

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.runWSLoop(runCtx, fs)
	}()
	log.Printf("[feishu] websocket bridge started app=%s", fs.AppID)
	return nil
}

func (b *FeishuBridge) runWSLoop(ctx context.Context, fs domain.ConfigFeishuChannel) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		lc := feishu.NewLongConn(fs, b.handleInbound).WithCardAction(b.handleCardAction)
		err := lc.Run(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("[feishu] websocket exited: %v; reconnect in %s", err, backoff)
		} else {
			log.Printf("[feishu] websocket exited; reconnect in %s", backoff)
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
			fs = cfg.Channels.Feishu
			b.adapter.UpdateConfig(fs)
			if !fs.Enabled {
				return
			}
		}
	}
}

func (b *FeishuBridge) handleInbound(ctx context.Context, msg port.InboundMessage) error {
	if b.ingress == nil {
		return fmt.Errorf("feishu ingress not configured")
	}
	if b.adapter != nil && len(msg.Media) > 0 {
		if err := b.adapter.EnrichInboundMedia(ctx, &msg); err != nil {
			log.Printf("[feishu] media download peer=%s: %v", msg.PeerID, err)
		}
	}
	reply, err := b.ingress.HandleInbound(ctx, msg)
	if err != nil {
		log.Printf("[feishu] handle inbound peer=%s: %v", msg.PeerID, err)
		// Ingress owns delivery when endpoint is registered; still try to surface errors.
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
	// Backward-compatible path when endpoint was not registered.
	if serr := b.adapter.SendReply(ctx, &msg, port.OutboundReply{Content: reply}); serr != nil {
		log.Printf("[feishu] send reply peer=%s: %v", msg.PeerID, serr)
		return serr
	}
	return nil
}

func (b *FeishuBridge) handleCardAction(ctx context.Context, msg port.InboundMessage, token string, formValue map[string]any) error {
	if b.ingress == nil {
		return fmt.Errorf("feishu ingress not configured")
	}
	ev, ok := InteractionFromCallback(msg, token)
	if !ok {
		log.Printf("[feishu] card action: unrecognized token %q", token)
		return nil
	}
	if len(formValue) > 0 && ev.Kind == port.InteractionAsk {
		// Stash form JSON in Meta for ingress to format against pending fields.
		if ev.Meta == nil {
			ev.Meta = map[string]string{}
		}
		ev.Meta["form_json"] = string(mustJSONMap(formValue))
		ev.Option = "form"
	}
	return b.ingress.HandleInteraction(ctx, ev)
}

func mustJSONMap(v map[string]any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (b *FeishuBridge) Stop() {
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
	log.Printf("[feishu] websocket bridge stopped")
}

func (b *FeishuBridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

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
	return s.store.ChannelBindings().UpdateMeta(ctx, string(channel), accountID, peerID, meta)
}

// WecomBridge runs WeCom AI Bot WebSocket and routes messages through ChannelIngress.
type WecomBridge struct {
	config  *ConfigManager
	ingress port.ChannelIngress

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewWecomBridge(config *ConfigManager, ingress port.ChannelIngress) *WecomBridge {
	return &WecomBridge{config: config, ingress: ingress}
}

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
			return b.handleInbound(msgCtx, lc, msg)
		})
		err := lc.Run(ctx)
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

func (b *WecomBridge) handleInbound(ctx context.Context, lc *wecom.LongConn, msg port.InboundMessage) error {
	reqID := ""
	streamID := ""
	if msg.Meta != nil {
		reqID = msg.Meta["req_id"]
		streamID = msg.Meta["stream_id"]
	}
	finish := func(content string) {
		if lc == nil || reqID == "" || streamID == "" {
			return
		}
		if strings.TrimSpace(content) == "" {
			content = "（无文本回复）"
		}
		if err := lc.ReplyStream(reqID, streamID, content, true); err != nil {
			log.Printf("[wecom] reply finish: %v", err)
		}
	}
	if b.ingress == nil {
		finish("企业微信通道未就绪")
		return fmt.Errorf("wecom ingress not configured")
	}
	reply, err := b.ingress.HandleInbound(ctx, msg)
	if err != nil {
		log.Printf("[wecom] handle inbound peer=%s: %v", msg.PeerID, err)
		finish("处理消息时出错：" + err.Error())
		return err
	}
	finish(reply)
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

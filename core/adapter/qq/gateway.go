package qq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"

	"github.com/gorilla/websocket"
)

const (
	opDispatch      = 0
	opHeartbeat     = 1
	opIdentify      = 2
	opResume        = 6
	opReconnect     = 7
	opInvalidSession = 9
	opHello         = 10
	opHeartbeatACK  = 11

	intentGroupAndC2C = 1 << 25
	intentInteraction = 1 << 26
)

// LongConn runs the QQ Bot outbound WebSocket Gateway client.
type LongConn struct {
	adapter *Adapter
	cfg     domain.ConfigQQChannel
	onMsg   InboundHandler
	onInter InteractionHandler
	account string

	mu        sync.Mutex
	sessionID string
	lastSeq   int64
}

func NewLongConn(adapter *Adapter, cfg domain.ConfigQQChannel, onMsg InboundHandler, onInter InteractionHandler) *LongConn {
	acc := strings.TrimSpace(cfg.AppID)
	if acc == "" {
		acc = "qq-default"
	}
	return &LongConn{adapter: adapter, cfg: cfg, onMsg: onMsg, onInter: onInter, account: acc}
}

// Run blocks until ctx is cancelled or the connection fails.
func (lc *LongConn) Run(ctx context.Context) error {
	if lc.adapter == nil {
		return fmt.Errorf("qq gateway: adapter required")
	}
	url, err := lc.adapter.GatewayURL(ctx)
	if err != nil {
		return err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("qq gateway dial: %w", err)
	}
	defer conn.Close()

	log.Printf("[qq] gateway connected app=%s", lc.cfg.AppID)

	var heartbeatInterval time.Duration
	identified := false

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var env DispatchPayload
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.S > 0 {
			lc.mu.Lock()
			lc.lastSeq = env.S
			lc.mu.Unlock()
		}
		switch env.Op {
		case opHello:
			var hello struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			_ = json.Unmarshal(env.D, &hello)
			if hello.HeartbeatInterval <= 0 {
				hello.HeartbeatInterval = 41250
			}
			heartbeatInterval = time.Duration(hello.HeartbeatInterval) * time.Millisecond
			lc.mu.Lock()
			sid := lc.sessionID
			seq := lc.lastSeq
			lc.mu.Unlock()
			if sid != "" && seq > 0 {
				if err := lc.writeJSON(conn, map[string]any{
					"op": opResume,
					"d": map[string]any{
						"token":      "QQBot " + mustToken(ctx, lc.adapter),
						"session_id": sid,
						"seq":        seq,
					},
				}); err != nil {
					return err
				}
			} else {
				tok, err := lc.adapter.accessToken(ctx)
				if err != nil {
					return err
				}
				if err := lc.writeJSON(conn, map[string]any{
					"op": opIdentify,
					"d": map[string]any{
						"token":   "QQBot " + tok,
						"intents": intentGroupAndC2C | intentInteraction,
						"shard":   []int{0, 1},
					},
				}); err != nil {
					return err
				}
			}
			if !identified {
				go lc.heartbeatLoop(ctx, conn, heartbeatInterval)
				identified = true
			}
		case opDispatch:
			if env.T == "READY" {
				var ready struct {
					SessionID string `json:"session_id"`
				}
				_ = json.Unmarshal(env.D, &ready)
				lc.mu.Lock()
				lc.sessionID = ready.SessionID
				lc.mu.Unlock()
				log.Printf("[qq] gateway ready session=%s", ready.SessionID)
				continue
			}
			if env.T == "RESUMED" {
				log.Printf("[qq] gateway resumed")
				continue
			}
			msg, inter, interID := NormalizeDispatch(lc.account, env.T, env.D)
			if msg != nil && lc.onMsg != nil {
				if err := lc.onMsg(*msg); err != nil {
					log.Printf("[qq] inbound: %v", err)
				}
			}
			if inter != nil && lc.onInter != nil {
				// Fill Kind/Target from raw in service; adapter passes raw.
				if err := lc.onInter(*inter, interID); err != nil {
					log.Printf("[qq] interaction: %v", err)
				}
			}
		case opReconnect:
			return fmt.Errorf("qq gateway: reconnect requested")
		case opInvalidSession:
			lc.mu.Lock()
			lc.sessionID = ""
			lc.lastSeq = 0
			lc.mu.Unlock()
			return fmt.Errorf("qq gateway: invalid session")
		case opHeartbeatACK:
			// ok
		}
	}
}

func (lc *LongConn) heartbeatLoop(ctx context.Context, conn *websocket.Conn, interval time.Duration) {
	if interval <= 0 {
		interval = 40 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			lc.mu.Lock()
			seq := lc.lastSeq
			lc.mu.Unlock()
			var d any = nil
			if seq > 0 {
				d = seq
			}
			if err := lc.writeJSON(conn, map[string]any{"op": opHeartbeat, "d": d}); err != nil {
				return
			}
		}
	}
}

func (lc *LongConn) writeJSON(conn *websocket.Conn, v any) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}

func mustToken(ctx context.Context, a *Adapter) string {
	tok, err := a.accessToken(ctx)
	if err != nil {
		return ""
	}
	return tok
}

package connector

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"danmo-work/core/remote/tunnel"
)

var errDisconnected = errors.New("remote: disconnected")

// Status is a snapshot for local API / Settings.
type Status struct {
	Enabled     bool       `json:"enabled"`
	Connected   bool       `json:"connected"`
	HubURL      string     `json:"hubUrl"`
	LocalBase   string     `json:"localBase"`
	TLSInsecure bool       `json:"tlsInsecure"`
	DeviceID    string     `json:"deviceId,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
}

// Connector maintains the outbound WSS tunnel to danmo-hub.
type Connector struct {
	cfg      Config
	identity *Identity

	mu          sync.RWMutex
	connected   bool
	lastErr     string
	connectedAt *time.Time

	writer *connWriter

	streamsMu     sync.Mutex
	inbound       map[uint32]*inboundStream
	streamCancels map[uint32]context.CancelFunc

	parentCtx context.Context
	runCancel context.CancelFunc
	running   atomic.Bool
}

// New creates a connector (does not start).
func New(cfg Config) (*Connector, error) {
	cfg = cfg.WithEnv()
	id, err := LoadOrCreateIdentity(cfg.IdentityPath)
	if err != nil {
		return nil, err
	}
	return &Connector{
		cfg:           cfg,
		identity:      id,
		writer:        &connWriter{},
		inbound:       make(map[uint32]*inboundStream),
		streamCancels: make(map[uint32]context.CancelFunc),
	}, nil
}

// Identity returns the durable device credentials.
func (c *Connector) Identity() *Identity { return c.identity }

// Config returns the active config.
func (c *Connector) Config() Config { return c.cfg }

// Start begins the reconnect loop. Non-blocking. Parent is retained for Apply.
func (c *Connector) Start(parent context.Context) {
	c.parentCtx = parent
	c.startLoop()
}

// Stop cancels the reconnect loop (process shutdown).
func (c *Connector) Stop() {
	c.stopLoop()
}

// Apply updates runtime config and restarts the connector loop if needed.
func (c *Connector) Apply(cfg Config) {
	cfg = cfg.WithEnv()
	if cfg.LocalBase == "" {
		cfg.LocalBase = c.cfg.LocalBase
	}
	if cfg.AppVersion == "" {
		cfg.AppVersion = c.cfg.AppVersion
	}
	if cfg.IdentityPath == "" {
		cfg.IdentityPath = c.cfg.IdentityPath
	}
	c.stopLoop()
	c.cfg = cfg
	c.setConnected(false, "")
	c.startLoop()
}

func (c *Connector) startLoop() {
	if c.parentCtx == nil {
		log.Printf("[remote] no parent context; not starting")
		return
	}
	if !c.cfg.Enabled {
		log.Printf("[remote] connector disabled")
		return
	}
	if NormalizeConnectorURL(c.cfg.HubURL) == "" {
		log.Printf("[remote] enabled but hub_url empty; not starting")
		return
	}
	if c.running.Swap(true) {
		return
	}
	ctx, cancel := context.WithCancel(c.parentCtx)
	c.runCancel = cancel
	go func() {
		defer c.running.Store(false)
		c.loop(ctx)
	}()
}

func (c *Connector) stopLoop() {
	if c.runCancel != nil {
		c.runCancel()
		c.runCancel = nil
	}
	c.writer.mu.Lock()
	if c.writer.conn != nil {
		_ = c.writer.conn.Close()
		c.writer.conn = nil
	}
	c.writer.mu.Unlock()
	c.cancelAllStreams()
	// Wait briefly for loop exit.
	deadline := time.Now().Add(2 * time.Second)
	for c.running.Load() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
}

// GetStatus returns connection status for the local API.
func (c *Connector) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st := Status{
		Enabled:     c.cfg.Enabled,
		Connected:   c.connected,
		HubURL:      c.cfg.HubURL,
		LocalBase:   c.cfg.LocalBase,
		TLSInsecure: c.cfg.TLSInsecure,
		LastError:   c.lastErr,
	}
	if c.identity != nil {
		st.DeviceID = c.identity.DeviceID
	}
	if c.connectedAt != nil {
		t := *c.connectedAt
		st.ConnectedAt = &t
	}
	return st
}

// RequestPairingCode asks Hub for a short-lived pairing code (device auth).
func (c *Connector) RequestPairingCode(ctx context.Context) (code string, expiresIn int, err error) {
	base := HubHTTPSBase(c.cfg.HubURL)
	if base == "" {
		return "", 0, fmt.Errorf("hub url not configured")
	}
	body, _ := tunnel.MarshalPayload(map[string]string{
		"device_id":     c.identity.DeviceID,
		"device_secret": c.identity.DeviceSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/pair/code", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("pair/code: %s: %s", resp.Status, truncate(string(raw), 200))
	}
	var out struct {
		Code      string `json:"code"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := tunnel.UnmarshalPayload(raw, &out); err != nil {
		return "", 0, err
	}
	return out.Code, out.ExpiresIn, nil
}

// RevokeTokens asks Hub to revoke all tokens for this device.
func (c *Connector) RevokeTokens(ctx context.Context) error {
	base := HubHTTPSBase(c.cfg.HubURL)
	if base == "" {
		return fmt.Errorf("hub url not configured")
	}
	body, _ := tunnel.MarshalPayload(map[string]string{
		"device_id":     c.identity.DeviceID,
		"device_secret": c.identity.DeviceSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/pair/revoke", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pair/revoke: %s: %s", resp.Status, truncate(string(raw), 200))
	}
	return nil
}

func (c *Connector) httpClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if c.cfg.TLSInsecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // dev only
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: tr}
}

func (c *Connector) loop(ctx context.Context) {
	backoff := time.Duration(tunnel.MinBackoffSec) * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		c.setConnected(false, errString(err))
		log.Printf("[remote] disconnected: %v; retry in %s", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > time.Duration(tunnel.MaxBackoffSec)*time.Second {
			backoff = time.Duration(tunnel.MaxBackoffSec) * time.Second
		}
	}
}

func (c *Connector) session(ctx context.Context) error {
	url := NormalizeConnectorURL(c.cfg.HubURL)
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	if c.cfg.TLSInsecure {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}

	c.writer.mu.Lock()
	c.writer.conn = conn
	c.writer.mu.Unlock()
	defer func() {
		c.writer.mu.Lock()
		if c.writer.conn == conn {
			c.writer.conn = nil
		}
		c.writer.mu.Unlock()
		_ = conn.Close()
		c.cancelAllStreams()
	}()

	reg, _ := tunnel.MarshalPayload(tunnel.RegisterPayload{
		DeviceID:     c.identity.DeviceID,
		DeviceSecret: c.identity.DeviceSecret,
		AppVersion:   c.cfg.AppVersion,
	})
	if err := c.writer.writeFrame(tunnel.TypeRegister, 0, reg); err != nil {
		return err
	}

	// Wait for RegisterOK
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	f, err := readConnFrame(conn)
	if err != nil {
		return err
	}
	if f.Type == tunnel.TypeError {
		var ep tunnel.ErrorPayload
		_ = tunnel.UnmarshalPayload(f.Payload, &ep)
		return fmt.Errorf("register rejected: %s: %s", ep.Code, ep.Message)
	}
	if f.Type != tunnel.TypeRegisterOK {
		return fmt.Errorf("expected RegisterOK, got type=%d", f.Type)
	}
	_ = conn.SetReadDeadline(time.Time{})
	c.setConnected(true, "")
	log.Printf("[remote] registered with hub as %s", c.identity.DeviceID)

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		t := time.NewTicker(time.Duration(tunnel.HeartbeatIntervalSec) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-t.C:
				payload, _ := tunnel.MarshalPayload(tunnel.HeartbeatPayload{TsUnixMs: time.Now().UnixMilli()})
				if err := c.writer.writeFrame(tunnel.TypeHeartbeat, 0, payload); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(time.Duration(tunnel.HeartbeatTimeoutSec) * time.Second))
			fr, rerr := readConnFrame(conn)
			if rerr != nil {
				errCh <- rerr
				return
			}
			c.dispatch(sessionCtx, fr)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Connector) dispatch(ctx context.Context, f tunnel.Frame) {
	switch f.Type {
	case tunnel.TypeHeartbeat:
		// ignore
	case tunnel.TypeHTTPOpen:
		var open tunnel.HTTPOpenPayload
		if err := tunnel.UnmarshalPayload(f.Payload, &open); err != nil {
			_ = c.sendError(f.StreamID, "bad_frame", err.Error())
			return
		}
		c.startHTTPOpen(ctx, f.StreamID, open)
	case tunnel.TypeHTTPBody:
		c.handleHTTPBody(f.StreamID, f.Payload)
	case tunnel.TypeStreamClose:
		c.handleStreamClose(f.StreamID)
	case tunnel.TypeError:
		var ep tunnel.ErrorPayload
		_ = tunnel.UnmarshalPayload(f.Payload, &ep)
		log.Printf("[remote] hub error: %s: %s", ep.Code, ep.Message)
	}
}

func (c *Connector) cancelAllStreams() {
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()
	for id, cancel := range c.streamCancels {
		cancel()
		delete(c.streamCancels, id)
	}
	for id, s := range c.inbound {
		s.finOnce.Do(func() { close(s.done) })
		delete(c.inbound, id)
	}
}

func (c *Connector) setConnected(ok bool, lastErr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = ok
	c.lastErr = lastErr
	if ok {
		now := time.Now().UTC()
		c.connectedAt = &now
	}
}

func readConnFrame(conn *websocket.Conn) (tunnel.Frame, error) {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return tunnel.Frame{}, err
	}
	return tunnel.DecodeFrame(bytes.NewReader(data))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

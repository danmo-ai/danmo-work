package service

import (
	"context"
	"sync"
	"testing"

	"danmo-work/core/port"
)

type mockEndpoint struct {
	typ      port.ChannelType
	caps     port.ChannelCapabilities
	mu       sync.Mutex
	delivered []port.OutboundMessage
	asks      []port.AskPrompt
	streams   []string
}

func (m *mockEndpoint) Type() port.ChannelType               { return m.typ }
func (m *mockEndpoint) Capabilities() port.ChannelCapabilities { return m.caps }

func (m *mockEndpoint) Deliver(ctx context.Context, in *port.InboundMessage, msg port.OutboundMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delivered = append(m.delivered, msg)
	return nil
}

func (m *mockEndpoint) StartStream(ctx context.Context, in *port.InboundMessage) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams = append(m.streams, "start")
	return "stream-1", nil
}

func (m *mockEndpoint) UpdateStream(ctx context.Context, in *port.InboundMessage, streamID, fullContent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams = append(m.streams, "update:"+fullContent)
	return nil
}

func (m *mockEndpoint) FinishStream(ctx context.Context, in *port.InboundMessage, streamID string, final port.OutboundMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams = append(m.streams, "finish:"+final.Text)
	m.delivered = append(m.delivered, final)
	return nil
}

func (m *mockEndpoint) PresentAsk(ctx context.Context, in *port.InboundMessage, ask port.AskPrompt) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asks = append(m.asks, ask)
	return true, nil
}

func TestDeliverOrReturnUsesEndpoint(t *testing.T) {
	ing := &ChannelIngressService{
		endpoints:   make(map[port.ChannelType]port.ChannelEndpoint),
		pendingAsks: make(map[string]pendingAsk),
	}
	ep := &mockEndpoint{
		typ:  port.ChannelFeishu,
		caps: port.ChannelCapabilities{RichCards: true},
	}
	ing.RegisterEndpoint(ep)
	msg := &port.InboundMessage{Type: port.ChannelFeishu, AccountID: "app", PeerID: "u1"}
	reply, err := ing.deliverOrReturn(context.Background(), msg, "hello", port.OutboundMessage{
		Kind: port.OutboundKindMarkdown,
		Text: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "" {
		t.Fatalf("expected empty reply when endpoint delivers, got %q", reply)
	}
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if len(ep.delivered) != 1 || ep.delivered[0].Text != "hello" {
		t.Fatalf("delivered=%v", ep.delivered)
	}
	if ep.delivered[0].Kind != port.OutboundKindMarkdown {
		t.Fatalf("kind=%s", ep.delivered[0].Kind)
	}
}

func TestDeliverOrReturnWithoutEndpoint(t *testing.T) {
	ing := &ChannelIngressService{
		endpoints:   make(map[port.ChannelType]port.ChannelEndpoint),
		pendingAsks: make(map[string]pendingAsk),
	}
	msg := &port.InboundMessage{Type: port.ChannelWeixin, AccountID: "a", PeerID: "p"}
	reply, err := ing.deliverOrReturn(context.Background(), msg, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "fallback" {
		t.Fatalf("got %q", reply)
	}
}

func TestPeerKeyAndAskAnswerHelpers(t *testing.T) {
	msg := port.InboundMessage{Type: port.ChannelFeishu, AccountID: "app", PeerID: "ou_1"}
	if peerKey(msg) != "feishu|app|ou_1" {
		t.Fatalf("peerKey=%s", peerKey(msg))
	}
	if got := resolveAskAnswer("1", []string{"yes", "no"}); got != "yes" {
		t.Fatalf("got %q", got)
	}
}

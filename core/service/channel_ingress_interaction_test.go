package service

import (
	"context"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type mockApproverEndpoint struct {
	mockEndpoint
	perms []port.PermissionPrompt
}

func (m *mockApproverEndpoint) PresentPermission(ctx context.Context, in *port.InboundMessage, ask port.PermissionPrompt) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.perms = append(m.perms, ask)
	return true, nil
}

func TestHandlePermissionAskSetsPendingAndStreamID(t *testing.T) {
	ep := &mockApproverEndpoint{
		mockEndpoint: mockEndpoint{
			typ:  port.ChannelFeishu,
			caps: port.ChannelCapabilities{InteractiveApprove: true, ProgressiveStream: true},
		},
	}
	ing := &ChannelIngressService{
		endpoints:    map[port.ChannelType]port.ChannelEndpoint{port.ChannelFeishu: ep},
		pendingAsks:  make(map[string]pendingAsk),
		pendingPerms: make(map[string]pendingPerm),
	}
	msg := &port.InboundMessage{Type: port.ChannelFeishu, AccountID: "app", PeerID: "u1"}
	ev := domain.StreamEvent{
		Type:    domain.EventPermissionAsk,
		Payload: []byte(`{"approvalId":"apr_1","tool":"exec_shell","description":"ls"}`),
	}
	ing.handlePermissionAsk(ev, collectParams{
		msg:         msg,
		ep:          ep,
		caps:        ep.Capabilities(),
		streamID:    "msg_progress",
		autoApprove: false,
	})
	key := peerKey(*msg)
	if !ing.hasPendingPerm(key) {
		t.Fatal("expected pending permission")
	}
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if len(ep.perms) != 1 || ep.perms[0].StreamID != "msg_progress" || ep.perms[0].ApprovalID != "apr_1" {
		t.Fatalf("perms=%+v", ep.perms)
	}
}

func TestHandleInteractionRequiresPeer(t *testing.T) {
	ing := &ChannelIngressService{
		endpoints:    make(map[port.ChannelType]port.ChannelEndpoint),
		pendingAsks:  make(map[string]pendingAsk),
		pendingPerms: make(map[string]pendingPerm),
	}
	err := ing.HandleInteraction(context.Background(), port.InteractionEvent{
		Type: port.ChannelFeishu,
		Kind: port.InteractionPermission,
	})
	if err == nil {
		t.Fatal("expected error for missing peer")
	}
}

func TestHandleInteractionUnknownKindNoStream(t *testing.T) {
	ep := &mockEndpoint{
		typ:  port.ChannelFeishu,
		caps: port.ChannelCapabilities{RichCards: true},
	}
	ing := &ChannelIngressService{
		endpoints:    map[port.ChannelType]port.ChannelEndpoint{port.ChannelFeishu: ep},
		pendingAsks:  make(map[string]pendingAsk),
		pendingPerms: make(map[string]pendingPerm),
	}
	if err := ing.HandleInteraction(context.Background(), port.InteractionEvent{
		Type:      port.ChannelFeishu,
		AccountID: "app",
		PeerID:    "u1",
		Kind:      port.InteractionKind("noop"),
	}); err != nil {
		t.Fatal(err)
	}
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if len(ep.streams) != 0 || len(ep.delivered) != 0 {
		t.Fatalf("unknown kind must not start turn/stream: streams=%v delivered=%v", ep.streams, ep.delivered)
	}
}

func TestResolvePeerProjectPrefersMeta(t *testing.T) {
	store := &stubPeerStore{
		projectID: "default-proj",
		meta:      map[string]string{"project_id": "peer-proj"},
	}
	ing := &ChannelIngressService{peers: store}
	pid, meta, err := ing.resolvePeerProject(context.Background(), port.InboundMessage{
		Type: port.ChannelQQ, AccountID: "app", PeerID: "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pid != "peer-proj" {
		t.Fatalf("got %q", pid)
	}
	if meta["project_id"] != "peer-proj" {
		t.Fatalf("meta=%v", meta)
	}
}

type stubPeerStore struct {
	projectID string
	meta      map[string]string
	sessionID string
}

func (s *stubPeerStore) GetProjectID(ctx context.Context, channel port.ChannelType, accountID string) (string, error) {
	return s.projectID, nil
}
func (s *stubPeerStore) GetBinding(ctx context.Context, channel port.ChannelType, accountID, peerID string) (string, map[string]string, error) {
	return s.sessionID, s.meta, nil
}
func (s *stubPeerStore) UpsertBinding(ctx context.Context, channel port.ChannelType, accountID, peerID, sessionID string, meta map[string]string) error {
	s.sessionID = sessionID
	s.meta = meta
	return nil
}
func (s *stubPeerStore) UpdateBindingMeta(ctx context.Context, channel port.ChannelType, accountID, peerID string, meta map[string]string) error {
	s.meta = meta
	return nil
}

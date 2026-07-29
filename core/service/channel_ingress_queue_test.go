package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	sqlitestore "danmo-work/core/store/sqlite"
)

func TestQueueInboundWhileBusyEnqueuesPending(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	sess := domain.Session{
		ID: "sess-1", Title: "IM", AgentID: "agent", ProjectID: "proj",
		Status: domain.SessionStatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := st.Sessions().Create(ctx, sess); err != nil {
		t.Fatal(err)
	}

	sessions := NewSessionManager(st, nil, nil)
	ing := &ChannelIngressService{sessions: sessions}

	msg := &port.InboundMessage{
		Type: port.ChannelWeixin, AccountID: "a", PeerID: "p",
		Text: "follow-up while busy",
	}
	reply, err := ing.queueInboundWhileBusy(ctx, msg, sess.ID, "agent", "mock/model")
	if err != nil {
		t.Fatal(err)
	}
	if reply != channelQueuedReply {
		t.Fatalf("reply=%q want %q", reply, channelQueuedReply)
	}

	list, err := sessions.ListPending(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("pending=%+v", list)
	}
	if list[0].Content != "follow-up while busy" || list[0].Status != domain.PendingQueued {
		t.Fatalf("item=%+v", list[0])
	}
	if list[0].AgentID != "agent" || list[0].ModelID != "mock/model" {
		t.Fatalf("agent/model not preserved: %+v", list[0])
	}
}

func TestQueueInboundWhileBusyFallsBackWhenSessionMissing(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager(st, nil, nil)
	ing := &ChannelIngressService{sessions: sessions}
	msg := &port.InboundMessage{Type: port.ChannelFeishu, AccountID: "a", PeerID: "p", Text: "x"}
	reply, err := ing.queueInboundWhileBusy(context.Background(), msg, "missing", "agent", "mock/model")
	if err != nil {
		t.Fatal(err)
	}
	if reply != channelBusyReply {
		t.Fatalf("reply=%q want fallback %q", reply, channelBusyReply)
	}
}

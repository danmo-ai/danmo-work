package browser

import (
	"context"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func testPage() *pageSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &pageSession{id: "s1", tabCtx: ctx, cancel: cancel, lastUsed: time.Now()}
}

func receiveWatch(t *testing.T, ch <-chan port.BrowserWatchEvent) port.BrowserWatchEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("watch channel closed")
		}
		return ev
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for browser frame")
		return port.BrowserWatchEvent{}
	}
}

func TestWatchPage_MissingDoesNotAcquire(t *testing.T) {
	m := New(domain.ConfigBrowserSection{Enabled: false})
	defer m.Close(context.Background())
	if _, err := m.WatchPage(context.Background(), "missing", port.BrowserWatchOptions{}); err != port.ErrBrowserPageNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if len(m.pages) != 0 {
		t.Fatal("watching must not create a page")
	}
}

func TestPageStream_MultipleWatchersAndFinalFrame(t *testing.T) {
	p := testPage()
	defer p.Close(context.Background())
	m := New(domain.ConfigBrowserSection{Enabled: false})
	defer m.Close(context.Background())
	p.mgr = m
	s := p.newStream(port.BrowserWatchOptions{})
	started := make(chan struct{}, 1)
	s.newRun = func(ctx context.Context) {
		defer close(s.done)
		started <- struct{}{}
		s.publish(port.BrowserWatchEvent{Frame: &port.BrowserFrame{Data: []byte("jpeg"), MimeType: "image/jpeg"}})
		<-ctx.Done()
	}
	ctx1, cancel1 := context.WithCancel(context.Background())
	ch1, err := s.subscribe(ctx1)
	if err != nil {
		t.Fatal(err)
	}
	_ = receiveWatch(t, ch1) // initial URL/title status
	ch2, err := s.subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ev := receiveWatch(t, ch1); string(ev.Frame.Data) != "jpeg" {
		t.Fatal("missing first frame")
	}
	if ev := receiveWatch(t, ch2); ev.Frame != nil || ev.Status.State != "live" {
		t.Fatal("new watcher must receive current status")
	}
	if ev := receiveWatch(t, ch2); string(ev.Frame.Data) != "jpeg" {
		t.Fatal("new watcher must receive cached frame")
	}
	cancel1()
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		n := len(s.watchers)
		s.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first watcher was not removed")
		}
		time.Sleep(time.Millisecond)
	}
	s.publish(port.BrowserWatchEvent{Frame: &port.BrowserFrame{Data: []byte("latest"), MimeType: "image/jpeg"}})
	if ev := receiveWatch(t, ch2); string(ev.Frame.Data) != "latest" {
		t.Fatal("remaining watcher stopped unexpectedly")
	}
	s.close()
	if _, ok := <-ch2; ok {
		t.Fatal("close must close watcher channel")
	}
	if last, ok := m.LastPageFrame("s1"); !ok || string(last.Frame.Data) != "latest" || last.Status.State != "closed" {
		t.Fatalf("unexpected final frame: %+v", last)
	}
}

func TestPageStream_SlowWatcherDropsFrames(t *testing.T) {
	p := testPage()
	defer p.Close(context.Background())
	out := make(chan port.BrowserWatchEvent, 1)
	for i := 0; i < 10; i++ {
		sendWatchEvent(context.Background(), out, port.BrowserWatchEvent{Frame: &port.BrowserFrame{Data: []byte{byte(i)}}})
	}
	select {
	case ev := <-out:
		if ev.Frame.Data[0] != 9 {
			t.Fatalf("expected latest frame, got %d", ev.Frame.Data[0])
		}
	default:
		t.Fatal("slow watcher did not receive latest frame")
	}
}

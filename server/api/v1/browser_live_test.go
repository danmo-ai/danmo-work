package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
	v1 "danmo-work/server/api/v1"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type liveBrowser struct {
	events    chan port.BrowserWatchEvent
	watched   chan string
	cancelled chan struct{}
}

func (b *liveBrowser) Status() domain.BrowserStatus                                  { return domain.BrowserStatus{} }
func (b *liveBrowser) Configure(domain.ConfigBrowserSection)                         {}
func (b *liveBrowser) Close(context.Context) error                                   { return nil }
func (b *liveBrowser) CloseAll(context.Context) error                                { return nil }
func (b *liveBrowser) ClosePage(context.Context, string) error                       { return nil }
func (b *liveBrowser) AcquirePage(context.Context, string) (port.BrowserPage, error) { return nil, nil }
func (b *liveBrowser) RenderHTML(context.Context, port.BrowserRenderOptions) (string, string, error) {
	return "", "", nil
}
func (b *liveBrowser) WatchPage(ctx context.Context, sid string, _ port.BrowserWatchOptions) (<-chan port.BrowserWatchEvent, error) {
	b.watched <- sid
	go func() { <-ctx.Done(); close(b.cancelled) }()
	return b.events, nil
}
func (b *liveBrowser) LastPageFrame(string) (port.BrowserLastFrame, bool) {
	return port.BrowserLastFrame{}, false
}

func TestSessionBrowserLive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := sqlitestore.New(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Sessions().Create(context.Background(), domain.Session{ID: "s1", Content: "test"}); err != nil {
		t.Fatal(err)
	}
	br := &liveBrowser{events: make(chan port.BrowserWatchEvent, 2), watched: make(chan string, 1), cancelled: make(chan struct{})}
	h := &v1.Handler{Sessions: service.NewSessionManager(st, nil, nil), Browser: br}
	srv := httptest.NewServer(v1.NewRouter(h, v1.RouterConfig{}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/sessions/s1/browser/live"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if sid := <-br.watched; sid != "s1" {
		t.Fatalf("watched %q", sid)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	typ, data, err := conn.ReadMessage()
	if err != nil || typ != websocket.TextMessage {
		t.Fatalf("status: %v", err)
	}
	var status struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(data, &status); err != nil || status.State != "connecting" {
		t.Fatalf("status=%s", data)
	}
	br.events <- port.BrowserWatchEvent{Frame: &port.BrowserFrame{Data: []byte("jpeg-data"), MimeType: "image/jpeg"}}
	typ, data, err = conn.ReadMessage()
	if err != nil || typ != websocket.BinaryMessage || string(data) != "jpeg-data" {
		t.Fatalf("frame type=%d data=%q err=%v", typ, data, err)
	}
	close(br.events)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err = conn.ReadMessage()
		if err != nil {
			break
		}
		var msg struct {
			State string `json:"state"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.State == "closed" {
			break
		}
	}
	_ = conn.Close()
	select {
	case <-br.cancelled:
	case <-time.After(time.Second):
		t.Fatal("watch context was not cancelled")
	}
	resp, err := http.Get(srv.URL + "/api/v1/sessions/missing/browser/live")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing session status=%d", resp.StatusCode)
	}
}

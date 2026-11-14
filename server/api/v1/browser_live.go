package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"danmo-work/core/port"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	browserLiveMaxConnections = 4
	browserLiveWriteTimeout   = 5 * time.Second
)

type browserLiveStatus struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	State     string `json:"state"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
	Error     string `json:"error,omitempty"`
}

func writeBrowserLiveStatus(ctx context.Context, conn *websocket.Conn, st browserLiveStatus) error {
	st.Type = "status"
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(browserLiveWriteTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// sessionBrowserLive streams the Agent-owned sticky page. It deliberately
// does not AcquirePage: an observer can never launch a browser or a container.
var browserLiveSessions = struct {
	sync.Mutex
	bySession map[string]int
}{bySession: map[string]int{}}

func acquireBrowserLiveSession(sessionID string) (func(), bool) {
	browserLiveSessions.Lock()
	defer browserLiveSessions.Unlock()
	if browserLiveSessions.bySession[sessionID] >= browserLiveMaxConnections {
		return nil, false
	}
	browserLiveSessions.bySession[sessionID]++
	return func() {
		browserLiveSessions.Lock()
		defer browserLiveSessions.Unlock()
		browserLiveSessions.bySession[sessionID]--
		if browserLiveSessions.bySession[sessionID] <= 0 {
			delete(browserLiveSessions.bySession, sessionID)
		}
	}, true
}

func sessionBrowserLive(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		if _, err := h.Sessions.Get(c.Request.Context(), sessionID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		release, ok := acquireBrowserLiveSession(sessionID)
		if !ok {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many browser live viewers"})
			return
		}
		defer release()
		watcher, ok := h.Browser.(port.BrowserWatcher)
		if h.Browser == nil || !ok {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "browser live view unavailable"})
			return
		}
		conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		watchCtx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		// Detect a disconnected observer even while no frames are being produced.
		go func() {
			defer cancel()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		frames, err := watcher.WatchPage(watchCtx, sessionID, port.BrowserWatchOptions{})
		if err != nil {
			if last, ok := watcher.LastPageFrame(sessionID); ok {
				last.Status.State = "closed"
				_ = writeBrowserLiveStatus(c.Request.Context(), conn, browserLiveStatus{
					SessionID: sessionID, State: "closed", URL: last.Status.URL, Title: last.Status.Title,
				})
				_ = conn.WriteMessage(websocket.BinaryMessage, last.Frame.Data)
				_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "page closed"), time.Now().Add(time.Second))
				return
			}
			state := "closed"
			if !errors.Is(err, port.ErrBrowserPageNotFound) && !errors.Is(err, port.ErrBrowserPageClosed) {
				state = "error"
			}
			_ = writeBrowserLiveStatus(c.Request.Context(), conn, browserLiveStatus{
				SessionID: sessionID, State: state, Error: err.Error(),
			})
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, state), time.Now().Add(time.Second))
			return
		}
		_ = writeBrowserLiveStatus(c.Request.Context(), conn, browserLiveStatus{SessionID: sessionID, State: "connecting"})
		for {
			select {
			case <-watchCtx.Done():
				return
			case ev, ok := <-frames:
				if !ok {
					_ = writeBrowserLiveStatus(c.Request.Context(), conn, browserLiveStatus{SessionID: sessionID, State: "closed"})
					_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "page closed"), time.Now().Add(time.Second))
					return
				}
				if ev.Frame != nil {
					if len(ev.Frame.Data) == 0 {
						continue
					}
					_ = conn.SetWriteDeadline(time.Now().Add(browserLiveWriteTimeout))
					if err := conn.WriteMessage(websocket.BinaryMessage, ev.Frame.Data); err != nil {
						return
					}
					continue
				}
				st := browserLiveStatus{SessionID: sessionID, State: ev.Status.State, URL: ev.Status.URL, Title: ev.Status.Title}
				if st.State == "" {
					st.State = "live"
				}
				if err := writeBrowserLiveStatus(c.Request.Context(), conn, st); err != nil {
					return
				}
			}
		}
	}
}

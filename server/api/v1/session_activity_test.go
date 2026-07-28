package v1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"danmo-work/core/domain"
	sqlitestore "danmo-work/core/store/sqlite"
	v1 "danmo-work/server/api/v1"

	"github.com/gin-gonic/gin"
)

func TestListSessionActivity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	st, err := sqlitestore.New(filepath.Join(dir, "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()

	if err := st.Turns().Create(ctx, domain.TurnLog{
		ID:        "turn-1",
		SessionID: "sess-run",
		Status:    domain.TurnRunning,
		AgentID:   "default",
		Goal:      "go",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Approvals().Create(ctx, domain.Approval{
		ID:        "appr-1",
		SessionID: "sess-wait",
		TurnID:    "turn-2",
		ToolName:  "exec_shell",
		Status:    "pending",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	// Running + pending approval on same session → awaiting_approval wins.
	if err := st.Turns().Create(ctx, domain.TurnLog{
		ID:        "turn-3",
		SessionID: "sess-both",
		Status:    domain.TurnRunning,
		AgentID:   "default",
		Goal:      "go",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Approvals().Create(ctx, domain.Approval{
		ID:        "appr-2",
		SessionID: "sess-both",
		TurnID:    "turn-3",
		ToolName:  "write_file",
		Status:    "pending",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	h := &v1.Handler{Store: st}
	engine := v1.NewRouter(h, v1.RouterConfig{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/activity", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var items []struct {
		SessionID            string `json:"sessionId"`
		State                string `json:"state"`
		RunningTurnID        string `json:"runningTurnId"`
		PendingApprovalCount int    `json:"pendingApprovalCount"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	byID := map[string]struct {
		State string
		Count int
	}{}
	for _, it := range items {
		byID[it.SessionID] = struct {
			State string
			Count int
		}{it.State, it.PendingApprovalCount}
	}
	if byID["sess-run"].State != "running" {
		t.Fatalf("sess-run=%+v", byID["sess-run"])
	}
	if byID["sess-wait"].State != "awaiting_approval" || byID["sess-wait"].Count != 1 {
		t.Fatalf("sess-wait=%+v", byID["sess-wait"])
	}
	if byID["sess-both"].State != "awaiting_approval" || byID["sess-both"].Count != 1 {
		t.Fatalf("sess-both=%+v", byID["sess-both"])
	}
}

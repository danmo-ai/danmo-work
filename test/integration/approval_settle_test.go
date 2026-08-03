package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"danmo-work/core/adapter/llm"
	"danmo-work/core/domain"
)

// ---------- Test: cancelling a turn blocked on approval settles the approval ----------
//
// Regression test for the abandoned-approval leak: when a turn is cancelled
// while WaitApproval is blocked, the pending DB row must flip to "expired"
// and a permission.decided(approved=false) event must be published so UIs
// reloading the event stream do not resurrect stale approve/deny buttons.
//
// Mock-only: the scripted exec_shell call (dangerous command → always Ask)
// makes the approval deterministic, which a real LLM cannot guarantee.
func TestCancelDuringApprovalSettlesApproval(t *testing.T) {
	requireMockLLM(t)
	ctx := context.Background()

	core, _ := setupCoreWithAutoApprove(t, false, func(m *llm.MockProvider) {
		// LooksDangerous command → gate Asks even inside a strong sandbox.
		m.AddToolCall("exec_shell", map[string]any{"command": "sudo rm -rf /tmp/approval-settle-test"})
		m.Finish("不会到达这里")
	})
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "审批挂起后取消 turn",
		AgentID: agentDefault,
		ModelID: mockModelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	// Wait for the gate to ask for approval.
	var since int64
	askEv := waitForSpecificEvent(t, r, s.ID, &since, domain.EventPermissionAsk)
	var ask domain.PermissionAskPayload
	if err := json.Unmarshal(askEv.Payload, &ask); err != nil || ask.ApprovalID == "" {
		t.Fatalf("bad permission.ask payload: %v %s", err, string(askEv.Payload))
	}
	turnID := askEv.TurnID
	if turnID == "" {
		t.Fatal("permission.ask event missing turn id")
	}

	appr, err := core.Store.Approvals().Get(ctx, ask.ApprovalID)
	if err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if appr.Status != "pending" {
		t.Fatalf("approval before cancel: want pending, got %q", appr.Status)
	}

	// Cancel the turn while WaitApproval is blocked.
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/sessions/"+s.ID+"/turns/"+turnID, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != 200 {
		t.Fatalf("cancel: %d %s", w2.Code, w2.Body.String())
	}

	// The abandoned approval must settle: DB row expired…
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		appr, err = core.Store.Approvals().Get(ctx, ask.ApprovalID)
		if err == nil && appr.Status == "expired" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if appr.Status != "expired" {
		t.Fatalf("approval after cancel: want expired, got %q", appr.Status)
	}

	// …and a permission.decided(approved=false) event published.
	var sawDecided bool
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !sawDecided {
		events := pollEvents(t, r, s.ID, 0)
		for _, ev := range events {
			if ev.Type != domain.EventPermissionDecided {
				continue
			}
			var p domain.PermissionDecidedPayload
			_ = json.Unmarshal(ev.Payload, &p)
			if p.ApprovalID == ask.ApprovalID {
				if p.Approved {
					t.Fatal("abandoned approval must not be published as approved")
				}
				sawDecided = true
			}
		}
		if !sawDecided {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !sawDecided {
		t.Error("expected permission.decided(approved=false) for abandoned approval")
	}

	// A late decision must not resurrect the expired approval.
	approveB, _ := json.Marshal(map[string]any{"approved": true, "scope": "once"})
	lateReq := httptest.NewRequest(http.MethodPost, "/api/v1/approvals/"+ask.ApprovalID+"/decide", nil)
	lateReq.Header.Set("Content-Type", "application/json")
	lateReq.Body = io.NopCloser(bytes.NewReader(approveB))
	lateReq.ContentLength = int64(len(approveB))
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, lateReq)
	time.Sleep(200 * time.Millisecond)
	appr, err = core.Store.Approvals().Get(ctx, ask.ApprovalID)
	if err != nil {
		t.Fatalf("get approval after late decide: %v", err)
	}
	if appr.Status == "approved" {
		t.Errorf("late decide resurrected expired approval: %q", appr.Status)
	}

	// The cancelled turn must land as cancelled in the DB.
	deadline = time.Now().Add(10 * time.Second)
	var turn domain.TurnLog
	for time.Now().Before(deadline) {
		turn, err = core.Store.Turns().Get(ctx, turnID)
		if err == nil && turn.Status == domain.TurnCancelled {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if turn.Status != domain.TurnCancelled {
		t.Errorf("cancelled turn status: want %q, got %q", domain.TurnCancelled, turn.Status)
	}
	t.Logf("approval settle ok: approval=%s status=%s turn=%s", ask.ApprovalID, appr.Status, turn.Status)
}

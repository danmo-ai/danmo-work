package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"danmo-work/core/domain"
)
const agentTeam = "team"

func TestTeamCreateSession(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", map[string]any{
		"content": "请列出可用的 Agent, 然后回复'Team 测试完成'",
		"agentId":  agentTeam,
		"modelId":  "deepseek/deepseek-v4-pro",
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)
	t.Logf("team session created: %s", s.ID)

	var since int64
	rep := waitForReport(t, r, s.ID, &since)
	if rep.Summary == "" {
		t.Error("expected non-empty report summary from team agent")
	}
	t.Logf("team report summary: %s", rep.Summary)

	entries := core.TurnLogs.ListSessionEntries(s.ID)
	if len(entries) < 2 {
		t.Errorf("expected at least 2 log entries, got %d", len(entries))
	}
	t.Logf("team turn log entries: %d", len(entries))
}

func TestTeamDelegation(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", map[string]any{
		"content": "请委派 explorer 去搜索代码库中是否有名为 AGENTS.md 的文件, 然后报告其内容摘要",
		"agentId":  agentTeam,
		"modelId":  "deepseek/deepseek-v4-pro",
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	events := collectAllEvents(t, r, s.ID)

	hasDelegate := false
	for _, ev := range events {
		if ev.Type == domain.EventDelegateStarted {
			hasDelegate = true
			var dsp domain.DelegateStartedPayload
			json.Unmarshal(ev.Payload, &dsp)
			t.Logf("delegate.started: agent=%s childTurn=%s", dsp.AgentID, dsp.ChildTurnID)
		}
		if ev.Type == domain.EventDelegateCompleted {
			t.Logf("delegate.completed")
		}
	}
	if !hasDelegate {
		t.Error("expected at least one delegate.started event from team agent")
	}
}

func TestTeamMultiTurn(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", map[string]any{
		"content": "请列出可用的 Agent, 回复'第一轮完成'",
		"agentId":  agentTeam,
		"modelId":  "deepseek/deepseek-v4-pro",
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	rep1 := waitForReport(t, r, s.ID, &since)
	t.Logf("team turn 1: %s", rep1.Summary)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "继续, 再次列出 Agent 清单, 回复'第二轮完成'",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var sendResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &sendResp)
	t.Logf("team turn 2 ID: %s", sendResp.TurnID)

	rep2 := waitForReport(t, r, s.ID, &since)
	t.Logf("team turn 2: %s", rep2.Summary)
	if rep2.Summary == "" {
		t.Error("expected non-empty report for team turn 2")
	}
}

func TestTeamCancelTurn(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", map[string]any{
		"content": "简单回复: 团队中断测试第一轮, 回答'第一轮完成'",
		"agentId":  agentTeam,
		"modelId":  "deepseek/deepseek-v4-pro",
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	waitForReport(t, r, s.ID, &since)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "这一轮会被取消, 不要执行任何操作",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var sendResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &sendResp)
	t.Logf("team turn to cancel: %s", sendResp.TurnID)

	time.Sleep(500 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/sessions/"+s.ID+"/turns/"+sendResp.TurnID, nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	if w3.Code != 200 {
		t.Fatalf("cancel: %d %s", w3.Code, w3.Body.String())
	}
	t.Logf("cancelled team turn: %s", sendResp.TurnID)

	// Wait for cancel to take effect so background goroutine exits before TempDir cleanup.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, s.ID, since)
		for _, ev := range events {
			since = ev.Seq
			if ev.Type == domain.EventTurnFailed {
				t.Logf("cancel confirmed via turn.failed event")
				// Wait for the background goroutine to finish writing to DB / turn log.
				waitDeadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(waitDeadline) {
					turn, err := core.Store.Turns().Get(context.Background(), sendResp.TurnID)
					if err == nil && (turn.Status == domain.TurnCancelled || turn.Status == domain.TurnFailed) {
						t.Logf("cancel goroutine finished, turn status=%s", turn.Status)
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("cancel event not received within timeout (goroutine may still be running)")
}

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/store/turnlog"
)
const agentDefault = "default"

func TestDefaultCreateSession(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请简单回复: 这是一条测试消息, 收到请回答'已收到测试消息'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)
	t.Logf("session created: %s", s.ID)

	var since int64
	rep := waitForReport(t, r, s.ID, &since)
	if rep.Summary == "" {
		t.Error("expected non-empty report summary from real LLM")
	}
	t.Logf("report summary: %s", rep.Summary)

	entries, err := turnlog.LoadTurnLog(core.Projects.ProjectDir, "_default", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 log entries, got %d", len(entries))
	}
	t.Logf("turn log entries: %d", len(entries))

	saved, err := core.Sessions.Get(nil, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != s.ID {
		t.Errorf("session ID mismatch: %s vs %s", saved.ID, s.ID)
	}
}

func TestDefaultNewTurn(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "简单回复: 第一轮, 回答'第一轮完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	rep1 := waitForReport(t, r, s.ID, &since)
	t.Logf("turn 1 report: %s", rep1.Summary)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "第二轮, 回答'第二轮完成'",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var sendResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &sendResp)
	t.Logf("new turn ID: %s", sendResp.TurnID)

	rep2 := waitForReport(t, r, s.ID, &since)
	t.Logf("turn 2 report: %s", rep2.Summary)
	if rep2.Summary == "" {
		t.Error("expected non-empty report for turn 2")
	}
}

func TestDefaultReview(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "简单回复: 审核测试第一轮, 回答'第一轮完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	waitForReport(t, r, s.ID, &since)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "审核测试第二轮, 回答'第二轮完成'",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	waitForReport(t, r, s.ID, &since)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+s.ID+"/turns", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	if w3.Code != 200 {
		t.Fatalf("list turns: %d", w3.Code)
	}
	var turns []domain.TurnLog
	json.Unmarshal(w3.Body.Bytes(), &turns)
	t.Logf("session %s has %d turns", s.ID, len(turns))
	if len(turns) < 2 {
		t.Errorf("expected at least 2 turns, got %d", len(turns))
	}
	for _, turn := range turns {
		t.Logf("  turn: id=%s status=%s sessionId=%s", turn.ID, turn.Status, turn.SessionID)
	}
}

func TestDefaultRecover(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "简单回复: 恢复测试, 回答'初始回复完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	var turnID string
	deadline := time.Now().Add(llmTimeout)
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, s.ID, since)
		for _, ev := range events {
			since = ev.Seq
			if ev.Type == domain.EventTurnStarted && turnID == "" {
				turnID = ev.TurnID
				var tsp domain.TurnStartedPayload
				json.Unmarshal(ev.Payload, &tsp)
				t.Logf("found turn: %s (goal: %s)", turnID, tsp.Goal)
			}
			if ev.Type == domain.EventSessionCompleted {
				goto firstTurnDone
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timeout waiting for session.completed")
firstTurnDone:
	if turnID == "" {
		t.Fatal("no turnID found")
	}

	t.Logf("resuming turn: %s", turnID)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sessions/"+s.ID+"/turns/"+turnID+"/resume", nil)
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != 200 {
		t.Fatalf("resume: %d %s", w2.Code, w2.Body.String())
	}

	deadline = time.Now().Add(llmTimeout)
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, s.ID, since)
		for _, ev := range events {
			since = ev.Seq
			if ev.Type == domain.EventSessionCompleted {
				t.Log("resumed turn completed")
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timeout waiting for resumed turn to complete")
}

func TestDefaultInterrupt(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "简单回复: 中断测试第一轮, 回答'第一轮完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	waitForReport(t, r, s.ID, &since)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "这一轮会被取消, 回复不要太长",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var sendResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &sendResp)
	t.Logf("turn to cancel: %s", sendResp.TurnID)

	time.Sleep(500 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/sessions/"+s.ID+"/turns/"+sendResp.TurnID, nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	if w3.Code != 200 {
		t.Fatalf("cancel: %d %s", w3.Code, w3.Body.String())
	}
	t.Logf("cancelled turn: %s", sendResp.TurnID)

	// Wait for the background goroutine to finish (turn status → cancelled in DB)
	// before returning, so TempDir cleanup doesn't race with the goroutine.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		turn, err := core.Store.Turns().Get(context.Background(), sendResp.TurnID)
		if err == nil && (turn.Status == domain.TurnCancelled || turn.Status == domain.TurnFailed) {
			t.Logf("cancel goroutine finished, turn status=%s", turn.Status)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("cancel not yet reflected in DB (goroutine may still be running)")
}

func TestDefaultContinueAfterInterrupt(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "简单回复: 中断前第一轮, 回答'第一轮完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	waitForReport(t, r, s.ID, &since)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "这轮会被取消, 回复'取消测试'",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var sendResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &sendResp)

	time.Sleep(300 * time.Millisecond)
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/sessions/"+s.ID+"/turns/"+sendResp.TurnID, nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	t.Logf("cancelled turn: %s (status: %d)", sendResp.TurnID, w3.Code)

	time.Sleep(1 * time.Second)
	pollEvents(t, r, s.ID, since)

	w4 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "中断后继续, 新的一轮, 回答'继续成功'",
	})
	if w4.Code != 200 {
		t.Fatalf("continue send: %d %s", w4.Code, w4.Body.String())
	}
	var continueResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w4.Body.Bytes(), &continueResp)
	t.Logf("continued turn: %s", continueResp.TurnID)

	deadline := time.Now().Add(llmTimeout)
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, s.ID, since)
		for _, ev := range events {
			since = ev.Seq
			if ev.Type == domain.EventReport {
				var rep domain.Report
				json.Unmarshal(ev.Payload, &rep)
				t.Logf("continue report: %s", rep.Summary)
				if rep.Summary == "" {
					t.Error("expected non-empty report after continue")
				}
				return
			}
			if ev.Type == domain.EventError {
				var ep domain.ErrorPayload
				json.Unmarshal(ev.Payload, &ep)
				t.Logf("(expected from cancel) error: %s", ep.Message)
			}
			if ev.Type == domain.EventTurnFailed {
				var tep domain.TurnEndedPayload
				json.Unmarshal(ev.Payload, &tep)
				t.Logf("(expected from cancel) turn failed: %s", tep.Summary)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timeout waiting for continue report")
}

func TestDefaultTurnLogPersistence(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请回复: 日志测试消息",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	events := collectAllEvents(t, r, s.ID)

	eventTypes := map[string]bool{}
	for _, ev := range events {
		eventTypes[ev.Type] = true
	}
	required := []string{domain.EventTurnStarted, domain.EventUserMessage, domain.EventReport, domain.EventTurnEnded, domain.EventSessionCompleted}
	for _, et := range required {
		if !eventTypes[et] {
			t.Errorf("missing required event type: %s", et)
		}
	}

	entries, err := turnlog.LoadTurnLog(core.Projects.ProjectDir, "_default", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("turn log entries: %d", len(entries))

	hasStart := false
	hasEnd := false
	for _, e := range entries {
		if e.Type == "start" {
			hasStart = true
		}
		if e.Type == "end" {
			hasEnd = true
			status, _ := e.Data["status"].(string)
			t.Logf("turn end status: %s", status)
			if status != "completed" {
				t.Errorf("expected turn end status 'completed', got %q", status)
			}
		}
	}
	if !hasStart {
		t.Error("turn log missing 'start' entry")
	}
	if !hasEnd {
		t.Error("turn log missing 'end' entry")
	}
}

func TestDefaultDBPersistence(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	cfgs, err := core.LLMConfig.GetAll(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) == 0 {
		t.Fatal("expected at least 1 LLM config in database")
	}
	t.Logf("LLM configs in DB: %d", len(cfgs))
	for _, cfg := range cfgs {
		t.Logf("  provider: %s/%s (models: %d)", cfg.Provider, cfg.Name, len(cfg.Models))
	}

	agents, err := core.Agents.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("agents in DB: %d", len(agents))
	if len(agents) == 0 {
		t.Fatal("expected agents in database")
	}

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "DB持久化测试: 请简单回复'持久化成功'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	waitForReport(t, r, s.ID, &since)

	saved, err := core.Sessions.Get(nil, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != s.ID {
		t.Errorf("persisted session ID mismatch")
	}
	if saved.AgentID != agentDefault {
		t.Errorf("persisted agent ID mismatch: %s", saved.AgentID)
	}

	all, _ := core.Sessions.List(nil)
	t.Logf("total sessions in DB: %d", len(all))
	found := false
	for _, tk := range all {
		if tk.ID == s.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created session not found in list")
	}
}

func TestDefaultLLMUsage(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请简单回复: LLM用量测试, 回复'用量测试完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	events := collectAllEvents(t, r, s.ID)

	var usageEvents []domain.StreamEvent
	for _, ev := range events {
		if ev.Type == domain.EventLLMUsage {
			usageEvents = append(usageEvents, ev)
			var usage domain.LLMUsagePayload
			json.Unmarshal(ev.Payload, &usage)
			t.Logf("llm.usage: prompt=%d completion=%d total=%d", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
		}
	}
	if len(usageEvents) == 0 {
		t.Error("expected at least 1 llm.usage event from real LLM")
	}
}

func TestDefaultFullLifecycle(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请回复: 生命周期测试-第一轮, 回答'第一轮完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	rep1 := waitForReport(t, r, s.ID, &since)
	t.Logf("step 1: first turn done, summary=%s", rep1.Summary)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "生命周期测试-第二轮, 回答'第二轮完成'",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var turn2Resp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &turn2Resp)
	t.Logf("step 2: new turn ID=%s", turn2Resp.TurnID)

	rep2 := waitForReport(t, r, s.ID, &since)
	t.Logf("step 3: second turn done, summary=%s", rep2.Summary)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sessions/"+s.ID+"/turns", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	var turns []domain.TurnLog
	json.Unmarshal(w3.Body.Bytes(), &turns)
	t.Logf("step 4: review turns, count=%d", len(turns))
	for _, turn := range turns {
		t.Logf("  turn id=%s status=%s", turn.ID, turn.Status)
	}
	if len(turns) < 2 {
		t.Errorf("expected at least 2 turns, got %d", len(turns))
	}

	w4 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "生命周期测试-第三轮(继续), 回答'第三轮完成'",
	})
	if w4.Code != 200 {
		t.Fatalf("continue: %d %s", w4.Code, w4.Body.String())
	}
	rep3 := waitForReport(t, r, s.ID, &since)
	t.Logf("step 5: third turn done, summary=%s", rep3.Summary)

	entries, err := turnlog.LoadTurnLog(core.Projects.ProjectDir, "_default", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("step 6: total log entries=%d", len(entries))

	var endCount int
	for _, e := range entries {
		if e.Type == "end" {
			endCount++
		}
	}
	t.Logf("  end entries: %d", endCount)
	if endCount < 3 {
		t.Errorf("expected at least 3 end entries (3 turns), got %d", endCount)
	}

	events := pollEvents(t, r, s.ID, 0)
	eventMap := map[string]int{}
	for _, ev := range events {
		eventMap[ev.Type]++
	}
	t.Logf("step 7: event types for all turns (current poll window):")
	for typ, count := range eventMap {
		t.Logf("  %s: %d", typ, count)
	}
	if eventMap[domain.EventTurnStarted] < 1 {
		t.Errorf("expected at least 1 turn.started event, got %d", eventMap[domain.EventTurnStarted])
	}
}

func TestDefaultSameSessionMultiTurn(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "多轮对话测试-第一轮, 回答'第一轮完成'",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	for i := 0; i < 3; i++ {
		if i == 0 {
			waitForReport(t, r, s.ID, &since)
			t.Logf("round %d completed", i+1)
			continue
		}
		msg := fmt.Sprintf("第%d轮, 回答'第%d轮完成'", i+1, i+1)
		w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
			UserInput: msg,
		})
		if w2.Code != 200 {
			t.Fatalf("send round %d: %d %s", i+1, w2.Code, w2.Body.String())
		}
		waitForReport(t, r, s.ID, &since)
		t.Logf("round %d completed", i+1)
	}

	entries, err := turnlog.LoadTurnLog(core.Projects.ProjectDir, "_default", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	endCount := 0
	for _, e := range entries {
		if e.Type == "end" {
			endCount++
		}
	}
	t.Logf("total end entries (turns): %d", endCount)
	if endCount < 3 {
		t.Errorf("expected at least 3 turns, got %d", endCount)
	}

	saved, _ := core.Sessions.Get(nil, s.ID)
	// After all turns complete, afterTurn sets the session status to "completed".
	// (It's only "active" while a turn is running.)
	if saved.Status != domain.SessionStatusCompleted {
		t.Errorf("session should be completed after all turns, got %s", saved.Status)
	}
}

func TestDefaultApprovalFlow(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCoreWithAutoApprove(t, false)
	// Force weak isolation so exec_shell still asks (sandbox-on auto-allow would skip).
	if core.Sandbox != nil {
		core.Sandbox.Configure(domain.ConfigSandboxSection{
			Enabled: true,
			Mode:    domain.SandboxModeWorkspaceWrite,
			Network: domain.SandboxNetworkDeny,
			Backend: "host-weak",
		})
	}
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	agent, err := core.Agents.Get(nil, agentDefault)
	if err != nil {
		t.Fatal(err)
	}
	agent.Tools = append(agent.Tools, domain.ToolBinding{
		ToolID:    "exec_shell",
		RiskLevel: domain.RiskHigh,
	})
	if err := core.Agents.Upsert(nil, *agent); err != nil {
		t.Fatal(err)
	}

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请使用 exec_shell 执行 'echo HELLO_OK' 一次, 然后报告执行结果. 不要调用任何其他工具, 仅调用 exec_shell 一次并报告. 直接以文本回复结果.",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)
	t.Logf("session created: %s", s.ID)

	var since int64
	deadline := time.Now().Add(llmTimeout)
	approvalCount := 0
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, s.ID, since)
		for _, ev := range events {
			since = ev.Seq
			switch ev.Type {
			case domain.EventPermissionAsk:
				var pap domain.PermissionAskPayload
				json.Unmarshal(ev.Payload, &pap)
				t.Logf("permission.ask: approvalID=%s tool=%s", pap.ApprovalID, pap.Tool)
				approvePermission(t, r, pap.ApprovalID)
				approvalCount++
				t.Logf("approval %s approved (%d total)", pap.ApprovalID, approvalCount)

			case domain.EventReport:
				var rep domain.Report
				json.Unmarshal(ev.Payload, &rep)
				t.Logf("report: %s", rep.Summary)
				if rep.Summary == "" {
					t.Error("expected non-empty report after approval")
				}
				t.Logf("total approvals handled: %d", approvalCount)
				if approvalCount == 0 {
					t.Error("expected at least 1 permission.ask event")
				}
				return

			case domain.EventError:
				var ep domain.ErrorPayload
				json.Unmarshal(ev.Payload, &ep)
				t.Fatalf("error: %s (kind=%s)", ep.Message, ep.Kind)

			case domain.EventTurnFailed:
				var tep domain.TurnEndedPayload
				json.Unmarshal(ev.Payload, &tep)
				t.Logf("turn failed (possibly doom loop): %s", tep.Summary)
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timeout waiting for report after approval")
}

func TestDefaultCrossTurnMessages(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	secret := "SECRET-X9Y8Z7"

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请记住这个密码: " + secret + "。简单回复'已记住'。",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	rep1 := waitForReport(t, r, s.ID, &since)
	t.Logf("turn 1 report: %s", rep1.Summary)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "我之前让你记住的密码是什么？请只回答密码值，不要说别的。",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}

	rep2 := waitForReport(t, r, s.ID, &since)
	t.Logf("turn 2 report: %s", rep2.Summary)

	if !strings.Contains(rep2.Summary, secret) {
		t.Errorf("turn 2 response does not contain secret %q — cross-turn context may be lost. Got: %s", secret, rep2.Summary)
	}
}

// TestDefaultToolCallWithRealLLM verifies that tool call arguments are correctly
// parsed from the real LLM response (not mock). This caught the parseArgs bug
// where OpenAI-compatible APIs return arguments as a JSON string, not object.
func TestDefaultToolCallWithRealLLM(t *testing.T) {
	requireRealLLM(t)
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请使用 read_file 工具列出工作目录的内容，然后回复我目录中有哪些文件和目录。",
		AgentID: agentDefault,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)
	t.Logf("session: %s", s.ID)

	events := collectAllEvents(t, r, s.ID)

	// Verify tool.running event exists (means arguments were parsed)
	var toolRunning bool
	var toolCompleted bool
	for _, ev := range events {
		if ev.Type == domain.EventToolRunning {
			toolRunning = true
			var tp domain.ToolPart
			json.Unmarshal(ev.Payload, &tp)
			t.Logf("tool.running: name=%s input=%v", tp.Name, tp.Input)
			if tp.Input == nil {
				t.Error("tool.running input is nil — arguments were not parsed from LLM response")
			}
		}
		if ev.Type == domain.EventToolCompleted {
			toolCompleted = true
			var tp domain.ToolPart
			json.Unmarshal(ev.Payload, &tp)
			t.Logf("tool.completed: name=%s output_len=%d", tp.Name, len(tp.Output))
		}
	}
	if !toolRunning {
		t.Error("expected at least one tool.running event — LLM did not call any tool")
	}
	if !toolCompleted {
		t.Error("expected at least one tool.completed event — tool did not complete")
	}

	// Verify turn log has assistant entries with tool_calls (the current format;
	// legacy "tool_call" entries are only produced by older code paths).
	entries, err := turnlog.LoadTurnLog(core.Projects.ProjectDir, "_default", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	var toolCalls int
	for _, e := range entries {
		if e.Type == "assistant" {
			if calls, ok := e.Data["tool_calls"].([]any); ok && len(calls) > 0 {
				toolCalls++
				t.Logf("turn log assistant tool_calls: %d call(s)", len(calls))
			}
		}
	}
	if toolCalls == 0 {
		t.Error("expected at least one tool_call in turn log")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

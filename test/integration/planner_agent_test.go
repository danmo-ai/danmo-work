package integration

import (
	"encoding/json"
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/store/turnlog"
)
const agentPlanner = "planner"

func TestPlannerCreateSession(t *testing.T) {
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请制定一个计划: 如何重构 core/domain 包以支持新的实体类型. 阅读 core/domain/ 目录下的文件并给出计划. 不要做任何修改.",
		AgentID: agentPlanner,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)
	t.Logf("planner session created: %s", s.ID)

	events := collectAllEvents(t, r, s.ID)

	hasReport := false
	for _, ev := range events {
		if ev.Type == domain.EventReport {
			hasReport = true
			var rep domain.Report
			json.Unmarshal(ev.Payload, &rep)
			t.Logf("planner report: %s", rep.Summary)
		}
		if ev.Type == domain.EventToolRunning {
			var tp domain.ToolPart
			json.Unmarshal(ev.Payload, &tp)
			t.Logf("planner tool: %s", tp.Name)
		}
	}
	if !hasReport {
		t.Error("expected report from planner agent")
	}

	entries, err := turnlog.LoadTurnLog(core.Projects.ProjectDir, "_default", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 log entries, got %d", len(entries))
	}
	t.Logf("planner turn log entries: %d", len(entries))
}

func TestPlannerNoWriteTools(t *testing.T) {
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	agent, err := core.Agents.Get(nil, agentPlanner)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range agent.Tools {
		if tool.ToolID == "write" || tool.ToolID == "edit" || tool.ToolID == "exec_shell" || tool.ToolID == "apply_patch" {
			t.Errorf("planner agent should NOT have %s tool", tool.ToolID)
		}
	}

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请只读分析 core/runtime 目录结构, 阅读 engine.go 文件, 然后给出架构分析报告. 不要做任何修改.",
		AgentID: agentPlanner,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	rep := waitForReport(t, r, s.ID, &since)
	if rep.Summary == "" {
		t.Error("expected non-empty report from planner agent")
	}
	t.Logf("planner report: %s", rep.Summary)
}

func TestPlannerMultiTurn(t *testing.T) {
	core, _ := setupCore(t)
	modelID := pickTestModel(t, core)
	r := newRouter(t, core)

	w := postJSON(t, r, "/api/v1/sessions", domain.CreateSessionRequest{
		Content: "请制定计划: 阅读 core/domain/agent.go 文件, 分析 Agent 实体的设计, 给出改进建议. 不要修改任何文件.",
		AgentID: agentPlanner,
		ModelID: modelID,
	})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var s domain.Session
	json.Unmarshal(w.Body.Bytes(), &s)

	var since int64
	rep1 := waitForReport(t, r, s.ID, &since)
	t.Logf("planner turn 1: %s", rep1.Summary)

	w2 := postJSON(t, r, "/api/v1/sessions/"+s.ID+"/turns", domain.SendMessageRequest{
		UserInput: "继续: 再调查 core/runtime 目录, 充实现有计划.",
	})
	if w2.Code != 200 {
		t.Fatalf("send: %d %s", w2.Code, w2.Body.String())
	}
	var sendResp struct {
		TurnID string `json:"turnId"`
	}
	json.Unmarshal(w2.Body.Bytes(), &sendResp)
	t.Logf("planner turn 2 ID: %s", sendResp.TurnID)

	rep2 := waitForReport(t, r, s.ID, &since)
	t.Logf("planner turn 2: %s", rep2.Summary)
	if rep2.Summary == "" {
		t.Error("expected non-empty report for planner turn 2")
	}
}

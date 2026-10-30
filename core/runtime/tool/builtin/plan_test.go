package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreatePlanRequiresNameAndPlan(t *testing.T) {
	h := &CreatePlan{}
	if _, err := h.Execute(context.Background(), map[string]any{"name": "x"}); err == nil {
		t.Fatal("expected error when plan missing")
	}
	if _, err := h.Execute(context.Background(), map[string]any{"plan": "# x"}); err == nil {
		t.Fatal("expected error when name missing")
	}
}

func TestCreatePlanFormatsTodosAndMeta(t *testing.T) {
	h := &CreatePlan{}
	res, err := h.Execute(context.Background(), map[string]any{
		"name":     "Auth",
		"overview": "Add login",
		"plan":     "# Auth\n\n**Goal:** login",
		"todos": []any{
			map[string]any{"id": "t1", "content": "Wire route", "status": "pending"},
			map[string]any{"id": "t2", "content": "Test", "status": "in_progress"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Plan (created): Auth") {
		t.Fatalf("content=%s", res.Content)
	}
	if !strings.Contains(res.Content, "[>] Test") {
		t.Fatalf("expected in-progress todo:\n%s", res.Content)
	}
	if res.Meta["name"] != "Auth" || res.Meta["plan"] != "# Auth\n\n**Goal:** login" {
		t.Fatalf("meta=%v", res.Meta)
	}
	raw, _ := res.Meta["todos"].(string)
	var todos []planTodoItem
	if err := json.Unmarshal([]byte(raw), &todos); err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 || todos[0].ID != "t1" {
		t.Fatalf("todos=%v", todos)
	}
}

func TestUpdatePlanRequiresAtLeastOneField(t *testing.T) {
	h := &UpdatePlan{}
	if _, err := h.Execute(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when empty")
	}
}

func TestUpdatePlanPartial(t *testing.T) {
	h := &UpdatePlan{}
	res, err := h.Execute(context.Background(), map[string]any{
		"overview": "Revised overview",
		"todos": []any{
			map[string]any{"content": "Only content", "status": "completed"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Plan (updated)") {
		t.Fatalf("content=%s", res.Content)
	}
	if !strings.Contains(res.Content, "[x] Only content") {
		t.Fatalf("expected completed todo:\n%s", res.Content)
	}
	raw, _ := res.Meta["todos"].(string)
	var todos []planTodoItem
	if err := json.Unmarshal([]byte(raw), &todos); err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0].ID != "todo-1" {
		t.Fatalf("auto id expected, got %#v", todos)
	}
}

func TestCreatePlanSchemaName(t *testing.T) {
	if got := (&CreatePlan{}).Name(); got != "create_plan" {
		t.Fatalf("name=%s", got)
	}
	if got := (&UpdatePlan{}).Name(); got != "update_plan" {
		t.Fatalf("name=%s", got)
	}
	if (&CreatePlan{}).Schema().Name != "create_plan" {
		t.Fatal("schema name mismatch")
	}
}

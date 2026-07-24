package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"danmo-work/core/bootstrap"
	"danmo-work/core/domain"
	apiv1 "danmo-work/server/api/v1"
)

const llmTimeout = 120 * time.Second

func setupCore(t *testing.T) (*bootstrap.Core, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "work.db")
	copyDB(t, "data/work.db", dbPath)
	dataDir := filepath.Join(tmpDir, "data")
	t.Setenv("WORK_DB_PATH", dbPath)
	core := bootstrap.New(bootstrap.Config{AutoApprove: true, DataDir: dataDir})
	return core, dataDir
}

func setupCoreWithAutoApprove(t *testing.T, autoApprove bool) (*bootstrap.Core, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "work.db")
	copyDB(t, "data/work.db", dbPath)
	dataDir := filepath.Join(tmpDir, "data")
	t.Setenv("WORK_DB_PATH", dbPath)
	core := bootstrap.New(bootstrap.Config{AutoApprove: autoApprove, DataDir: dataDir})
	return core, dataDir
}

// pickTestModel returns the best enabled model for integration tests.
// Priority: DeepSeek pro > DeepSeek any > first enabled model.
// Format: "provider_name/model_name" (e.g. "deepseek/deepseek-v4-pro").
func pickTestModel(t *testing.T, core *bootstrap.Core) string {
	t.Helper()
	models := core.LLMConfig.ListModels(context.Background())
	if len(models) == 0 {
		t.Fatal("no enabled LLM models in test DB — add at least one LLM provider config")
	}
	// Prefer DeepSeek pro model
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.ProviderID), "deepseek") &&
			strings.Contains(strings.ToLower(m.Name), "pro") {
			t.Logf("test model: %s (provider=%s)", m.ID, m.Provider)
			return m.ID
		}
	}
	// Fallback: first DeepSeek model
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.ProviderID), "deepseek") {
			t.Logf("test model: %s (provider=%s)", m.ID, m.Provider)
			return m.ID
		}
	}
	// Fallback: first enabled model
	t.Logf("test model (fallback): %s (provider=%s)", models[0].ID, models[0].Provider)
	return models[0].ID
}

func copyDB(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	candidates := []string{src, "data/work.db", "../../data/work.db"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".danmo-work", "work.db"))
	}
	var chosen string
	for _, c := range candidates {
		st, err := os.Stat(c)
		if err != nil || st.IsDir() {
			continue
		}
		// Prefer a seed that has LLM provider rows (package seed may be empty).
		out, err := exec.Command("sqlite3", c, "SELECT COUNT(*) FROM llm_configs;").Output()
		if err == nil && strings.TrimSpace(string(out)) != "0" {
			chosen = c
			break
		}
		if chosen == "" {
			chosen = c
		}
	}
	if chosen == "" {
		t.Fatalf("copy db: no seed work.db found (tried %v)", candidates)
	}
	cmd := exec.Command("cp", chosen, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("copy db from %s: %v: %s", chosen, err, out)
	}
	t.Logf("test seed db: %s", chosen)
}

func newRouter(t *testing.T, core *bootstrap.Core) http.Handler {
	t.Helper()
	h := &apiv1.Handler{
		Sessions:     core.Sessions,
		Projects:     core.Projects,
		LLMConfig:    core.LLMConfig,
		SearchConfig: core.SearchConfig,
		Agents:       core.Agents,
		Skills:       core.Skills,
		Weixin:       core.Weixin,
		Store:        core.Store,
	}
	return apiv1.NewRouter(h, apiv1.RouterConfig{})
}

func postJSON(t *testing.T, r http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func pollEvents(t *testing.T, r http.Handler, sessionID string, since int64) []domain.StreamEvent {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sessionID+"/events/poll?since="+fmt.Sprintf("%d", since), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var events []domain.StreamEvent
	_ = json.Unmarshal(w.Body.Bytes(), &events)
	return events
}

func findEvent(t *testing.T, events []domain.StreamEvent, typ string) domain.StreamEvent {
	t.Helper()
	for _, ev := range events {
		if ev.Type == typ {
			return ev
		}
	}
	t.Fatalf("event %q not found", typ)
	return domain.StreamEvent{}
}

func waitForReport(t *testing.T, r http.Handler, sessionID string, since *int64) domain.Report {
	t.Helper()
	deadline := time.Now().Add(llmTimeout)
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, sessionID, *since)
		for _, ev := range events {
			*since = ev.Seq
			if ev.Type == domain.EventReport {
				var rep domain.Report
				_ = json.Unmarshal(ev.Payload, &rep)
				return rep
			}
			if ev.Type == domain.EventError {
				var ep domain.ErrorPayload
				_ = json.Unmarshal(ev.Payload, &ep)
				t.Fatalf("engine error: %s (kind=%s)", ep.Message, ep.Kind)
			}
			if ev.Type == domain.EventTurnFailed {
				var tep domain.TurnEndedPayload
				_ = json.Unmarshal(ev.Payload, &tep)
				t.Fatalf("turn failed: %s", tep.Summary)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timeout waiting for report")
	return domain.Report{}
}

func collectAllEvents(t *testing.T, r http.Handler, sessionID string) []domain.StreamEvent {
	t.Helper()
	var since int64
	var events []domain.StreamEvent
	deadline := time.Now().Add(llmTimeout)
	for time.Now().Before(deadline) {
		batch := pollEvents(t, r, sessionID, since)
		for _, ev := range batch {
			since = ev.Seq
			events = append(events, ev)
			if ev.Type == domain.EventSessionCompleted {
				return events
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timeout waiting for session.completed")
	return events
}

func waitForSpecificEvent(t *testing.T, r http.Handler, sessionID string, since *int64, eventType string) domain.StreamEvent {
	t.Helper()
	deadline := time.Now().Add(llmTimeout)
	for time.Now().Before(deadline) {
		events := pollEvents(t, r, sessionID, *since)
		for _, ev := range events {
			*since = ev.Seq
			if ev.Type == eventType {
				return ev
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for event %q", eventType)
	return domain.StreamEvent{}
}

func approvePermission(t *testing.T, r http.Handler, approvalID string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"approved": true, "scope": "once"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/approvals/"+approvalID+"/decide", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(b))
	req.ContentLength = int64(len(b))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("decide approval %s: %d %s", approvalID, w.Code, w.Body.String())
	}
}

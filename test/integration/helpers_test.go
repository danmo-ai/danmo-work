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
	"sync"
	"testing"
	"time"

	"danmo-work/core/adapter/llm"
	"danmo-work/core/bootstrap"
	"danmo-work/core/domain"
	apiv1 "danmo-work/server/api/v1"
)

const llmTimeout = 120 * time.Second

// mockModelID is the model id used when tests run against the in-process
// mock LLM. Session creation does not validate the model against llm_configs,
// and the injected provider ignores the model string entirely.
const mockModelID = "mock/mock-model"

// ---------- LLM mode ----------
//
// Integration tests run in one of two modes:
//
//   - real: a seed work.db with llm_configs rows was found (developer machine)
//     — the full suite runs against the configured real LLM.
//   - mock: no seed DB with credentials — engine-level tests (recovery,
//     cancellation, approvals, history) run against llm.MockProvider through
//     the full bootstrap/HTTP stack; tests asserting real-LLM semantics
//     (tool-choice quality, delegation behavior, answer content) are skipped.
//
// WORK_TEST_LLM=real forces real mode (skipping everything when no seed DB
// exists); WORK_TEST_LLM=mock forces mock mode.

var (
	seedDBOnce sync.Once
	seedDBPath string // seed work.db containing llm_configs rows, "" if none
)

// seedDBWithProviders returns the path of a seed work.db that has at least
// one LLM provider configured, or "" when no such seed exists.
func seedDBWithProviders() string {
	seedDBOnce.Do(func() {
		candidates := []string{"data/work.db", "../../data/work.db"}
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, ".danmo-work", "work.db"))
		}
		for _, c := range candidates {
			st, err := os.Stat(c)
			if err != nil || st.IsDir() {
				continue
			}
			out, err := exec.Command("sqlite3", c, "SELECT COUNT(*) FROM llm_configs;").Output()
			if err == nil && strings.TrimSpace(string(out)) != "0" {
				seedDBPath = c
				return
			}
		}
	})
	return seedDBPath
}

// useMockLLM reports whether tests should run against the in-process mock.
func useMockLLM(t *testing.T) bool {
	t.Helper()
	switch os.Getenv("WORK_TEST_LLM") {
	case "mock":
		return true
	case "real":
		if seedDBWithProviders() == "" {
			t.Skip("WORK_TEST_LLM=real but no seed work.db with llm_configs found")
		}
		return false
	default:
		return seedDBWithProviders() == ""
	}
}

// requireRealLLM skips the test in mock mode. Use for tests that assert
// real-LLM behavior (tool selection, delegation decisions, answer content)
// which a scripted mock cannot meaningfully verify.
func requireRealLLM(t *testing.T) {
	t.Helper()
	if useMockLLM(t) {
		t.Skip("requires a real LLM (no seed work.db with llm_configs; set WORK_TEST_LLM=real to force)")
	}
}

// requireMockLLM skips the test in real mode. Use for tests that need a
// deterministically scripted LLM (e.g. exact tool-call sequences).
func requireMockLLM(t *testing.T) {
	t.Helper()
	if !useMockLLM(t) {
		t.Skip("requires the mock LLM (set WORK_TEST_LLM=mock to force)")
	}
}

// setupCore boots a full core in a temp environment. In mock mode a fresh DB
// is created (bootstrap migrates the schema and seeds builtin agents) and
// llm.MockProvider is injected; optional script funcs queue mock steps —
// they are ignored in real mode. Unscripted mock Chat calls return "done".
func setupCore(t *testing.T, script ...func(m *llm.MockProvider)) (*bootstrap.Core, string) {
	t.Helper()
	return setupCoreWithAutoApprove(t, true, script...)
}

func setupCoreWithAutoApprove(t *testing.T, autoApprove bool, script ...func(m *llm.MockProvider)) (*bootstrap.Core, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "work.db")
	mock := useMockLLM(t)
	if !mock {
		copyDB(t, "data/work.db", dbPath)
	}
	dataDir := filepath.Join(tmpDir, "data")
	t.Setenv("WORK_DB_PATH", dbPath)
	t.Setenv("WORK_STORE_DB_PATH", filepath.Join(tmpDir, "store.db"))
	cfg := bootstrap.Config{AutoApprove: autoApprove, DataDir: dataDir}
	if mock {
		provider := llm.NewMock()
		for _, s := range script {
			s(provider)
		}
		cfg.LLM = provider
	}
	core := bootstrap.New(cfg)
	return core, dataDir
}

// pickTestModel returns the best enabled model for integration tests.
// Priority: DeepSeek pro > DeepSeek any > first enabled model.
// Format: "provider_name/model_name" (e.g. "deepseek/deepseek-v4-pro").
// In mock mode it returns a fixed placeholder id.
func pickTestModel(t *testing.T, core *bootstrap.Core) string {
	t.Helper()
	if useMockLLM(t) {
		return mockModelID
	}
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

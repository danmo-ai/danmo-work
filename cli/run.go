package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/bootstrap"
	"danmo-work/core/domain"
)

func runHeadless(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	workdir := fs.String("workdir", "", "project working directory (required)")
	goal := fs.String("goal", "", "task goal / instruction (required)")
	agentID := fs.String("agent", "default", "agent id")
	modelID := fs.String("model", "", "model id as provider_name/model_name (or WORK_MODEL)")
	timeoutStr := fs.String("timeout", "10m", "max wall time for the turn")
	autoApprove := fs.Bool("auto-approve", true, "auto-approve tool permissions")
	reportPath := fs.String("report", "", "write final Report JSON to this path")
	logsDir := fs.String("logs-dir", "", "export turn JSONL + events + failure analysis (e.g. /logs/agent/turnlogs)")
	apiKey := fs.String("api-key", "", "LLM API key (or WORK_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY)")
	baseURL := fs.String("base-url", "", "LLM base URL (or WORK_BASE_URL)")
	providerType := fs.String("provider-type", "", "openai|anthropic (default: inferred from model / WORK_PROVIDER_TYPE)")
	evalMode := fs.Bool("eval", true, "eval mode: disable ask_user and isolate data dirs")
	dataDir := fs.String("data-dir", "", "override WORK_DATA_DIR (default: temp under workdir/.dq-eval)")
	configPath := fs.String("config", "", "WORK_CONFIG path (optional)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*workdir) == "" || strings.TrimSpace(*goal) == "" {
		fmt.Fprintln(os.Stderr, "usage: danmo-work-cli run --workdir DIR --goal TEXT [--model provider/model] ...")
		fs.PrintDefaults()
		return 2
	}

	absWork, err := filepath.Abs(*workdir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workdir:", err)
		return 2
	}
	if st, err := os.Stat(absWork); err != nil || !st.IsDir() {
		fmt.Fprintln(os.Stderr, "workdir must be an existing directory:", absWork)
		return 2
	}

	timeout, err := time.ParseDuration(*timeoutStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid --timeout:", err)
		return 2
	}

	model := firstNonEmpty(*modelID, os.Getenv("WORK_MODEL"))
	if model == "" {
		fmt.Fprintln(os.Stderr, "--model or WORK_MODEL is required")
		return 2
	}

	key := firstNonEmpty(*apiKey, os.Getenv("WORK_API_KEY"), os.Getenv("OPENAI_API_KEY"), os.Getenv("ANTHROPIC_API_KEY"))
	base := firstNonEmpty(*baseURL, os.Getenv("WORK_BASE_URL"))
	ptype := firstNonEmpty(*providerType, os.Getenv("WORK_PROVIDER_TYPE"))

	if *evalMode {
		_ = os.Setenv("WORK_EVAL_MODE", "1")
	}
	if *autoApprove {
		_ = os.Setenv("WORK_AUTO_APPROVE", "true")
	}

	evalRoot := *dataDir
	if evalRoot == "" {
		evalRoot = filepath.Join(absWork, ".dq-eval")
	}
	if err := os.MkdirAll(evalRoot, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "data-dir:", err)
		return 1
	}
	dbPath := filepath.Join(evalRoot, "work.db")
	dataPath := filepath.Join(evalRoot, "data")
	_ = os.Setenv("WORK_DB_PATH", dbPath)
	_ = os.Setenv("WORK_DATA_DIR", dataPath)

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = os.Getenv("WORK_CONFIG")
	}
	if cfgPath == "" {
		if cand := filepath.Join(filepath.Dir(os.Args[0]), "config.yaml"); fileExists(cand) {
			cfgPath = cand
		}
	}

	core := bootstrap.New(bootstrap.Config{
		ConfigPath:  cfgPath,
		AutoApprove: *autoApprove,
		DataDir:     dataPath,
	})
	defer core.Close()

	ctx := context.Background()
	if err := ensureEvalLLM(ctx, core, model, key, base, ptype); err != nil {
		fmt.Fprintln(os.Stderr, "llm config:", err)
		return 1
	}

	proj, err := core.Projects.Create(ctx, domain.CreateProjectRequest{
		Name:      "eval",
		Directory: absWork,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "create project:", err)
		return 1
	}

	session, err := core.Sessions.Create(ctx, domain.CreateSessionRequest{
		Content:   *goal,
		AgentID:   *agentID,
		ProjectID: proj.ID,
		ModelID:   model,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "create session:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "session %s project %s model %s\n", session.ID, proj.ID, model)

	rep, waitErr := waitForReport(core, session.ID, timeout)
	events := core.Sessions.StreamEvents(session.ID, 0)

	outLogs := *logsDir
	if outLogs == "" && os.Getenv("WORK_EVAL_LOGS_DIR") != "" {
		outLogs = os.Getenv("WORK_EVAL_LOGS_DIR")
	}
	if outLogs != "" {
		if err := exportEvalArtifacts(core, session.ID, proj.ID, *goal, model, outLogs, rep, waitErr, events); err != nil {
			fmt.Fprintln(os.Stderr, "export logs:", err)
		} else {
			fmt.Fprintf(os.Stderr, "turn logs exported to %s\n", outLogs)
		}
	}

	if waitErr != nil {
		fmt.Fprintln(os.Stderr, waitErr)
		// Still write partial report if we have one.
		if *reportPath != "" && rep.Status != "" {
			raw, _ := json.MarshalIndent(rep, "", "  ")
			_ = os.WriteFile(*reportPath, raw, 0644)
		}
		return 1
	}

	raw, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(raw))
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, raw, 0644); err != nil {
			fmt.Fprintln(os.Stderr, "write report:", err)
			return 1
		}
	}
	if rep.Status != domain.ReportDone {
		return 1
	}
	return 0
}

func exportEvalArtifacts(core *bootstrap.Core, sessionID, projectID, goal, model, logsDir string, rep domain.Report, waitErr error, events []domain.StreamEvent) error {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return err
	}

	// Full event stream for postmortem.
	evPath := filepath.Join(logsDir, "events.jsonl")
	ef, err := os.Create(evPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(ef)
	for _, ev := range events {
		_ = enc.Encode(ev)
	}
	_ = ef.Close()

	// Turn JSONL + zip per root turn.
	turnIDs := uniqueTurnIDs(events)
	turnsDir := filepath.Join(logsDir, "turns")
	_ = os.MkdirAll(turnsDir, 0755)
	for _, tid := range turnIDs {
		if raw, err := core.TurnLogs.LoadRawLog(tid); err == nil && len(raw) > 0 {
			_ = os.WriteFile(filepath.Join(turnsDir, tid+".jsonl"), raw, 0644)
		}
		if zipBytes, err := core.TurnLogs.LoadTurnLogZip(tid, events); err == nil && len(zipBytes) > 0 {
			_ = os.WriteFile(filepath.Join(turnsDir, tid+".zip"), zipBytes, 0644)
		}
	}

	analysis := buildFailureAnalysis(sessionID, projectID, goal, model, rep, waitErr, events)
	aj, _ := json.MarshalIndent(analysis, "", "  ")
	if err := os.WriteFile(filepath.Join(logsDir, "analysis.json"), aj, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(logsDir, "analysis.md"), []byte(renderAnalysisMarkdown(analysis)), 0644); err != nil {
		return err
	}
	if rep.Status != "" && waitErr != nil {
		// placeholder report on hard failure
		stub := domain.Report{Status: domain.ReportFailed, Summary: waitErr.Error()}
		raw, _ := json.MarshalIndent(stub, "", "  ")
		_ = os.WriteFile(filepath.Join(logsDir, "report.json"), raw, 0644)
	} else if rep.Status != "" {
		// no-op
	} else {
		raw, _ := json.MarshalIndent(rep, "", "  ")
		_ = os.WriteFile(filepath.Join(logsDir, "report.json"), raw, 0644)
	}
	return nil
}

type failureAnalysis struct {
	SessionID      string   `json:"sessionId"`
	ProjectID      string   `json:"projectId"`
	Goal           string   `json:"goal"`
	Model          string   `json:"model"`
	ReportStatus   string   `json:"reportStatus,omitempty"`
	ReportSummary  string   `json:"reportSummary,omitempty"`
	WaitError      string   `json:"waitError,omitempty"`
	FailureReasons []string `json:"failureReasons"`
	ToolErrors     []string `json:"toolErrors"`
	TurnFailures   []string `json:"turnFailures"`
	EngineErrors   []string `json:"engineErrors"`
	LastAgentText  string   `json:"lastAgentText,omitempty"`
	TurnIDs        []string `json:"turnIds"`
	EventCounts    map[string]int `json:"eventCounts"`
}

func buildFailureAnalysis(sessionID, projectID, goal, model string, rep domain.Report, waitErr error, events []domain.StreamEvent) failureAnalysis {
	a := failureAnalysis{
		SessionID:   sessionID,
		ProjectID:   projectID,
		Goal:        goal,
		Model:       model,
		EventCounts: map[string]int{},
		TurnIDs:     uniqueTurnIDs(events),
	}
	if rep.Status != "" {
		a.ReportStatus = string(rep.Status)
		a.ReportSummary = rep.Summary
	}
	if waitErr != nil {
		a.WaitError = waitErr.Error()
	}

	for _, ev := range events {
		a.EventCounts[ev.Type]++
		switch ev.Type {
		case domain.EventToolError:
			var p domain.ToolPart
			_ = json.Unmarshal(ev.Payload, &p)
			msg := p.Error
			if msg == "" {
				msg = p.Output
			}
			if msg == "" {
				msg = fmt.Sprintf("%s (%s)", p.Name, p.CallID)
			} else {
				msg = fmt.Sprintf("%s: %s", p.Name, msg)
			}
			a.ToolErrors = append(a.ToolErrors, msg)
		case domain.EventTurnFailed:
			var p domain.TurnEndedPayload
			_ = json.Unmarshal(ev.Payload, &p)
			a.TurnFailures = append(a.TurnFailures, fmt.Sprintf("%s: %s", p.TurnID, p.Summary))
		case domain.EventError:
			var p domain.ErrorPayload
			_ = json.Unmarshal(ev.Payload, &p)
			a.EngineErrors = append(a.EngineErrors, p.Message)
		case domain.EventAgentMessage:
			var p domain.AgentMessagePayload
			_ = json.Unmarshal(ev.Payload, &p)
			if strings.TrimSpace(p.Text) != "" {
				a.LastAgentText = p.Text
			}
		}
	}

	if waitErr != nil {
		a.FailureReasons = append(a.FailureReasons, "wait_error: "+waitErr.Error())
	}
	if rep.Status != "" && rep.Status != domain.ReportDone {
		a.FailureReasons = append(a.FailureReasons, "report_status: "+string(rep.Status))
		if rep.Summary != "" {
			a.FailureReasons = append(a.FailureReasons, "report_summary: "+rep.Summary)
		}
	}
	for _, e := range a.EngineErrors {
		a.FailureReasons = append(a.FailureReasons, "engine: "+e)
	}
	for _, e := range a.TurnFailures {
		a.FailureReasons = append(a.FailureReasons, "turn_failed: "+e)
	}
	for _, e := range a.ToolErrors {
		a.FailureReasons = append(a.FailureReasons, "tool_error: "+e)
	}
	if len(a.FailureReasons) == 0 && rep.Status == domain.ReportDone {
		a.FailureReasons = append(a.FailureReasons, "none (agent reported done — if Harbor reward is 0, check verifier / output files)")
	}
	return a
}

func renderAnalysisMarkdown(a failureAnalysis) string {
	var b strings.Builder
	b.WriteString("# Eval failure analysis\n\n")
	fmt.Fprintf(&b, "- **session**: `%s`\n", a.SessionID)
	fmt.Fprintf(&b, "- **model**: `%s`\n", a.Model)
	fmt.Fprintf(&b, "- **report**: `%s` — %s\n", a.ReportStatus, a.ReportSummary)
	if a.WaitError != "" {
		fmt.Fprintf(&b, "- **wait error**: %s\n", a.WaitError)
	}
	b.WriteString("\n## Failure reasons\n\n")
	if len(a.FailureReasons) == 0 {
		b.WriteString("- (none recorded)\n")
	} else {
		for _, r := range a.FailureReasons {
			fmt.Fprintf(&b, "- %s\n", r)
		}
	}
	if len(a.ToolErrors) > 0 {
		b.WriteString("\n## Tool errors\n\n")
		for _, r := range a.ToolErrors {
			fmt.Fprintf(&b, "- %s\n", r)
		}
	}
	if a.LastAgentText != "" {
		b.WriteString("\n## Last agent message\n\n```\n")
		b.WriteString(a.LastAgentText)
		b.WriteString("\n```\n")
	}
	b.WriteString("\n## Artifacts\n\n")
	b.WriteString("- `events.jsonl` — full stream\n")
	b.WriteString("- `turns/*.jsonl` — turn tool call logs\n")
	b.WriteString("- `turns/*.zip` — packaged turn logs (incl. delegates)\n")
	b.WriteString("- `analysis.json` — machine-readable copy of this summary\n")
	return b.String()
}

func uniqueTurnIDs(events []domain.StreamEvent) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, ev := range events {
		tid := ev.TurnID
		if tid == "" && ev.Type == domain.EventTurnStarted {
			var p domain.TurnStartedPayload
			if json.Unmarshal(ev.Payload, &p) == nil {
				tid = p.TurnID
			}
		}
		if tid == "" {
			continue
		}
		if _, ok := seen[tid]; ok {
			continue
		}
		seen[tid] = struct{}{}
		out = append(out, tid)
	}
	return out
}

func ensureEvalLLM(ctx context.Context, core *bootstrap.Core, modelID, apiKey, baseURL, providerType string) error {
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid model %q: expected provider_name/model_name", modelID)
	}
	providerName := parts[0]
	modelName := parts[1]

	ptype := domain.LLMProviderType(providerType)
	if ptype == "" {
		switch strings.ToLower(providerName) {
		case "anthropic", "claude":
			ptype = domain.LLMProviderAnthropic
		default:
			ptype = domain.LLMProviderOpenAI
		}
	}

	if apiKey == "" && ptype == domain.LLMProviderAnthropic {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" && (ptype == domain.LLMProviderOpenAI || ptype == domain.LLMProviderOpenAIResponses || ptype == domain.LLMProviderLocal) {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		if _, _, err := core.LLMConfig.ResolveProvider(ctx, modelID); err == nil {
			return nil
		}
		return fmt.Errorf("API key required (--api-key / WORK_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY)")
	}

	_, err := core.LLMConfig.Upsert(ctx, domain.UpsertLLMProviderConfigRequest{
		Provider: ptype,
		Name:     providerName,
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Models: []domain.LLMModelRef{
			{Name: modelName, Enabled: true},
		},
	})
	return err
}

func waitForReport(core *bootstrap.Core, sessionID string, timeout time.Duration) (domain.Report, error) {
	deadline := time.Now().Add(timeout)
	var since int64
	for time.Now().Before(deadline) {
		events := core.Sessions.StreamEvents(sessionID, since)
		for _, ev := range events {
			since = ev.Seq
			switch ev.Type {
			case domain.EventReport:
				var rep domain.Report
				if err := json.Unmarshal(ev.Payload, &rep); err != nil {
					return domain.Report{}, fmt.Errorf("decode report: %w", err)
				}
				return rep, nil
			case domain.EventError:
				var ep domain.ErrorPayload
				_ = json.Unmarshal(ev.Payload, &ep)
				return domain.Report{Status: domain.ReportFailed, Summary: ep.Message}, fmt.Errorf("engine error: %s", ep.Message)
			case domain.EventTurnFailed:
				var tep domain.TurnEndedPayload
				_ = json.Unmarshal(ev.Payload, &tep)
				return domain.Report{Status: domain.ReportFailed, Summary: tep.Summary}, fmt.Errorf("turn failed: %s", tep.Summary)
			case domain.EventAskUserPending:
				return domain.Report{Status: domain.ReportBlocked, Summary: "ask_user"}, fmt.Errorf("ask_user pending in eval mode (should be disabled)")
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return domain.Report{Status: domain.ReportFailed, Summary: "timeout"}, fmt.Errorf("timeout waiting for report after %s", timeout)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

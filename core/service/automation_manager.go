package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// AutomationManager persists automations and runs a lightweight cron loop.
type AutomationManager struct {
	repo     port.AutomationRepo
	sessions *SessionManager
	projects *ProjectManager

	mu     sync.Mutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewAutomationManager(repo port.AutomationRepo, sessions *SessionManager, projects *ProjectManager) *AutomationManager {
	return &AutomationManager{
		repo:     repo,
		sessions: sessions,
		projects: projects,
		stopCh:   make(chan struct{}),
	}
}

func (m *AutomationManager) List(ctx context.Context) ([]domain.Automation, error) {
	return m.repo.List(ctx)
}

func (m *AutomationManager) Get(ctx context.Context, id string) (domain.Automation, error) {
	return m.repo.Get(ctx, id)
}

func (m *AutomationManager) Create(ctx context.Context, req domain.UpsertAutomationRequest) (domain.Automation, error) {
	if strings.TrimSpace(req.Name) == "" {
		return domain.Automation{}, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return domain.Automation{}, fmt.Errorf("prompt is required")
	}
	trigger := req.Trigger
	if trigger == "" {
		trigger = domain.AutomationTriggerManual
	}
	a := domain.Automation{
		ID:          fmt.Sprintf("auto-%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		Trigger:     trigger,
		Schedule:    req.Schedule,
		EventType:   req.EventType,
		WebhookPath: req.WebhookPath,
		AgentID:     req.AgentID,
		ProjectID:   req.ProjectID,
		ModelID:     req.ModelID,
		Prompt:      req.Prompt,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if a.Trigger == domain.AutomationTriggerSchedule && a.Schedule != "" {
		a.NextRunAt = nextCronApprox(a.Schedule, time.Now()).UTC().Format(time.RFC3339)
	}
	if a.Trigger == domain.AutomationTriggerWebhook && a.WebhookPath == "" {
		a.WebhookPath = "/hooks/" + a.ID
	}
	if err := m.repo.Upsert(ctx, a); err != nil {
		return domain.Automation{}, err
	}
	return a, nil
}

func (m *AutomationManager) Update(ctx context.Context, id string, req domain.UpsertAutomationRequest) (domain.Automation, error) {
	existing, err := m.repo.Get(ctx, id)
	if err != nil {
		return domain.Automation{}, err
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.Enabled = req.Enabled
	if req.Trigger != "" {
		existing.Trigger = req.Trigger
	}
	existing.Schedule = req.Schedule
	existing.EventType = req.EventType
	if req.WebhookPath != "" {
		existing.WebhookPath = req.WebhookPath
	}
	existing.AgentID = req.AgentID
	existing.ProjectID = req.ProjectID
	existing.ModelID = req.ModelID
	if req.Prompt != "" {
		existing.Prompt = req.Prompt
	}
	existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if existing.Trigger == domain.AutomationTriggerSchedule && existing.Schedule != "" {
		existing.NextRunAt = nextCronApprox(existing.Schedule, time.Now()).UTC().Format(time.RFC3339)
	}
	if err := m.repo.Upsert(ctx, existing); err != nil {
		return domain.Automation{}, err
	}
	return existing, nil
}

func (m *AutomationManager) Delete(ctx context.Context, id string) error {
	return m.repo.Delete(ctx, id)
}

func (m *AutomationManager) Toggle(ctx context.Context, id string) (domain.Automation, error) {
	a, err := m.repo.Get(ctx, id)
	if err != nil {
		return domain.Automation{}, err
	}
	a.Enabled = !a.Enabled
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.repo.Upsert(ctx, a); err != nil {
		return domain.Automation{}, err
	}
	return a, nil
}

// RunNow executes an automation immediately (manual or webhook).
func (m *AutomationManager) RunNow(ctx context.Context, id string) (domain.Automation, error) {
	a, err := m.repo.Get(ctx, id)
	if err != nil {
		return domain.Automation{}, err
	}
	return m.execute(ctx, a)
}

// FindByWebhookPath locates an enabled webhook automation.
func (m *AutomationManager) FindByWebhookPath(ctx context.Context, path string) (domain.Automation, error) {
	path = strings.TrimSpace(path)
	list, err := m.repo.ListEnabled(ctx)
	if err != nil {
		return domain.Automation{}, err
	}
	for _, a := range list {
		if a.Trigger == domain.AutomationTriggerWebhook && strings.TrimSuffix(a.WebhookPath, "/") == strings.TrimSuffix(path, "/") {
			return a, nil
		}
	}
	return domain.Automation{}, fmt.Errorf("webhook automation not found")
}

func (m *AutomationManager) execute(ctx context.Context, a domain.Automation) (domain.Automation, error) {
	if m.sessions == nil {
		return a, fmt.Errorf("automation sessions not configured")
	}
	projectID := a.ProjectID
	if projectID == "" && m.projects != nil {
		if projects, err := m.projects.List(ctx); err == nil && len(projects) > 0 {
			projectID = projects[0].ID
		}
	}
	agentID := a.AgentID
	if agentID == "" {
		agentID = "default"
	}
	title := "Automation: " + a.Name
	// Create starts the first turn via StartSession (content required).
	sess, err := m.sessions.Create(ctx, domain.CreateSessionRequest{
		Title:         title,
		ProjectID:     projectID,
		AgentID:       agentID,
		ModelID:       a.ModelID,
		Content:       a.Prompt,
		SkipAutoTitle: true,
	})
	a.LastRunAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		a.LastStatus = "error"
		_ = m.repo.Upsert(ctx, a)
		return a, err
	}
	a.LastStatus = "started"
	a.LastTurnID = sess.ID
	if a.Trigger == domain.AutomationTriggerSchedule && a.Schedule != "" {
		a.NextRunAt = nextCronApprox(a.Schedule, time.Now()).UTC().Format(time.RFC3339)
	}
	_ = m.repo.Upsert(ctx, a)
	return a, nil
}

// StartScheduler ticks every minute and runs due schedule automations.
func (m *AutomationManager) StartScheduler() {
	m.mu.Lock()
	m.stopCh = make(chan struct{})
	m.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.tick(context.Background())
			}
		}
	}()
}

func (m *AutomationManager) StopScheduler() {
	m.mu.Lock()
	if m.stopCh != nil {
		select {
		case <-m.stopCh:
		default:
			close(m.stopCh)
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *AutomationManager) tick(ctx context.Context) {
	list, err := m.repo.ListEnabled(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, a := range list {
		if a.Trigger != domain.AutomationTriggerSchedule || a.Schedule == "" {
			continue
		}
		if a.NextRunAt == "" {
			a.NextRunAt = nextCronApprox(a.Schedule, now).UTC().Format(time.RFC3339)
			_ = m.repo.Upsert(ctx, a)
			continue
		}
		due, err := time.Parse(time.RFC3339, a.NextRunAt)
		if err != nil || now.Before(due) {
			continue
		}
		if _, err := m.execute(ctx, a); err != nil {
			log.Printf("automation %s run failed: %v", a.ID, err)
		}
	}
}

// nextCronApprox supports standard 5-field cron with minute/hour wildcards or numbers.
// It is intentionally simple (no @daily sugar): "M H * * *" → next matching local time.
func nextCronApprox(expr string, from time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) < 5 {
		return from.Add(time.Hour)
	}
	wantMin := parseCronField(fields[0])
	wantHour := parseCronField(fields[1])
	t := from.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 60*24*8; i++ {
		minOK := wantMin < 0 || t.Minute() == wantMin
		hourOK := wantHour < 0 || t.Hour() == wantHour
		if minOK && hourOK {
			return t
		}
		t = t.Add(time.Minute)
	}
	return from.Add(24 * time.Hour)
}

func parseCronField(f string) int {
	if f == "*" {
		return -1
	}
	var n int
	for _, r := range f {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

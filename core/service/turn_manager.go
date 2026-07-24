package service

import (
	"context"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type TurnManager struct {
	repo port.TurnRepo
}

func NewTurnManager(repo port.TurnRepo) *TurnManager {
	return &TurnManager{repo: repo}
}

func (m *TurnManager) Create(ctx context.Context, t domain.TurnLog) error {
	return m.repo.Create(ctx, t)
}

func (m *TurnManager) UpdateStatus(ctx context.Context, id string, status domain.TurnStatus) error {
	return m.repo.UpdateStatus(ctx, id, status)
}

func (m *TurnManager) Get(ctx context.Context, id string) (domain.TurnLog, error) {
	return m.repo.Get(ctx, id)
}

func (m *TurnManager) ListBySession(ctx context.Context, sessionID string) ([]domain.TurnLog, error) {
	return m.repo.ListBySession(ctx, sessionID)
}

func (m *TurnManager) ListByStatus(ctx context.Context, status domain.TurnStatus) ([]domain.TurnLog, error) {
	return m.repo.ListByStatus(ctx, status)
}

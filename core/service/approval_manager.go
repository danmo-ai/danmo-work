package service

import (
	"context"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

type ApprovalManager struct {
	store port.ApprovalRepo
}

func NewApprovalManager(store port.ApprovalRepo) *ApprovalManager {
	return &ApprovalManager{store: store}
}

func (m *ApprovalManager) Create(ctx context.Context, a domain.Approval) error {
	return m.store.Create(ctx, a)
}

func (m *ApprovalManager) Update(ctx context.Context, a domain.Approval) error {
	return m.store.Update(ctx, a)
}

func (m *ApprovalManager) ListByStatus(ctx context.Context, status string) ([]domain.Approval, error) {
	return m.store.ListByStatus(ctx, status)
}

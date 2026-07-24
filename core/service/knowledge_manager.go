package service

import (
	"context"
	"danmo-work/core/domain"
)

type KnowledgeManager struct{}

func NewKnowledgeManager() *KnowledgeManager { return &KnowledgeManager{} }

func (m *KnowledgeManager) Search(ctx context.Context, kbIDs []string, query string, k int) ([]domain.KnowledgeDoc, error) {
	return nil, nil
}
func (m *KnowledgeManager) List(ctx context.Context) ([]domain.KnowledgeDoc, error) { return nil, nil }

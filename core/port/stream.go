package port

import (
	"context"
	"danmo-work/core/domain"
)

type EventStream interface {
	Publish(ctx context.Context, sessionID, turnID, typ string, payload any) domain.StreamEvent
	Subscribe(sessionID string) chan domain.StreamEvent
	Unsubscribe(sessionID string, ch chan domain.StreamEvent)
	ListSince(sessionID string, since int64) []domain.StreamEvent
}

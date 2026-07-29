package port

import (
	"context"
	"errors"

	"danmo-work/core/domain"
)

var ErrSessionTurnRunning = errors.New("session already has a running turn")

type Engine interface {
	StartSession(ctx context.Context, s domain.Session, attachments []domain.UserAttachment)
	StartTurn(ctx context.Context, sessionID, userInput, agentID, modelID string, attachments []domain.UserAttachment) (string, error)
	ResumeTurn(ctx context.Context, sessionID, turnID string) error
	CancelTurn(ctx context.Context, turnID string)
	// ActiveTurnID returns the in-flight turn for a session, if any.
	ActiveTurnID(sessionID string) string
	// SoftSteer injects a user message into the active turn at the next safe
	// boundary (after the current tool batch / before the next model call).
	SoftSteer(sessionID, content string, attachments []domain.UserAttachment) error
	ListTurns(sessionID string) []domain.TurnLog

	StreamEvents(sessionID string, since int64) []domain.StreamEvent
	Subscribe(sessionID string) chan domain.StreamEvent
	Unsubscribe(sessionID string, ch chan domain.StreamEvent)
	ResolveApproval(id string, approved bool, scope string)
	PublishPermissionDecided(sessionID, turnID, approvalID string, approved bool, scope string)
	ResolveAskUser(askID, answer string) error
	// RevokeSessionNetworkGrants clears Soft + Hard session network grants.
	RevokeSessionNetworkGrants(sessionID string)
}

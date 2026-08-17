package study

import (
	"context"
	"errors"
	"time"
)

// ErrSessionNotFound is returned when no session matches the given ID.
var ErrSessionNotFound = errors.New("study session not found")

// SessionRepository persists study Sessions. Today the only implementation
// is SQLite-backed (internal/infrastructure/sqlite).
type SessionRepository interface {
	Create(ctx context.Context, session Session) error
	End(ctx context.Context, id string, endedAt time.Time) error
}

// MessageRepository persists the Messages exchanged within a study Session.
type MessageRepository interface {
	Append(ctx context.Context, message Message) error
	ListBySession(ctx context.Context, sessionID string) ([]Message, error)
}

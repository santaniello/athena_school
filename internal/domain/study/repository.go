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
	// GetByID returns the session with the given id, or
	// ErrSessionNotFound if it does not exist.
	GetByID(ctx context.Context, id string) (Session, error)
	// ListByFolder returns every session in the given folder.
	ListByFolder(ctx context.Context, folderID string) ([]Session, error)
	// Reopen clears EndedAt on the session with the given id, so the user
	// can keep chatting in it, or returns ErrSessionNotFound if it does
	// not exist.
	Reopen(ctx context.Context, id string) error
	// MoveToFolder reassigns the session with the given id to folderID,
	// or returns ErrSessionNotFound if it does not exist.
	MoveToFolder(ctx context.Context, id, folderID string) error
	// ReassignFolder moves every session in fromFolderID to toFolderID.
	ReassignFolder(ctx context.Context, fromFolderID, toFolderID string) error
	// Delete permanently removes the session with the given id, or returns
	// ErrSessionNotFound if it does not exist.
	Delete(ctx context.Context, id string) error
}

// MessageRepository persists the Messages exchanged within a study Session.
type MessageRepository interface {
	Append(ctx context.Context, message Message) error
	ListBySession(ctx context.Context, sessionID string) ([]Message, error)
	// DeleteBySession permanently removes every message for sessionID.
	DeleteBySession(ctx context.Context, sessionID string) error
}

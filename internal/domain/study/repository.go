package study

import (
	"context"
	"errors"
)

// ErrSessionNotFound is returned when no session matches the given ID.
var ErrSessionNotFound = errors.New("study session not found")

// SessionRepository persists study Sessions. Today the only implementation
// is SQLite-backed (internal/infrastructure/sqlite).
type SessionRepository interface {
	Create(ctx context.Context, session Session) error
	// GetByID returns the session with the given id, or
	// ErrSessionNotFound if it does not exist.
	GetByID(ctx context.Context, id string) (Session, error)
	// ListByFolder returns every session in the given folder.
	ListByFolder(ctx context.Context, folderID string) ([]Session, error)
	// MoveToFolder reassigns the session with the given id to folderID,
	// or returns ErrSessionNotFound if it does not exist.
	MoveToFolder(ctx context.Context, id, folderID string) error
	// ReassignFolder moves every session in fromFolderID to toFolderID.
	ReassignFolder(ctx context.Context, fromFolderID, toFolderID string) error
	// Delete permanently removes the session with the given id, or returns
	// ErrSessionNotFound if it does not exist.
	Delete(ctx context.Context, id string) error
	// UpdateContext persists sessionID's new ContextUsage, or returns
	// ErrSessionNotFound if it does not exist. Called from within a
	// Transactor.WithinTx block whenever it must land atomically with a
	// message write (see application/study).
	UpdateContext(ctx context.Context, sessionID string, usage ContextUsage) error
}

// MessageRepository persists the Messages exchanged within a study Session.
type MessageRepository interface {
	Append(ctx context.Context, message Message) error
	ListBySession(ctx context.Context, sessionID string) ([]Message, error)
	// DeleteBySession permanently removes every message for sessionID.
	DeleteBySession(ctx context.Context, sessionID string) error
}

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
	// DeleteByFolder permanently removes every session in folderID, along
	// with their owned dependents, atomically.
	DeleteByFolder(ctx context.Context, folderID string) error
	// Delete permanently removes the session with the given id and its owned
	// dependents atomically, or returns ErrSessionNotFound if it does not exist.
	Delete(ctx context.Context, id string) error
	// UpdateContext persists sessionID's new ContextUsage, or returns
	// ErrSessionNotFound if it does not exist. Called from within a
	// Transactor.WithinTx block whenever it must land atomically with a
	// message write (see application/study).
	UpdateContext(ctx context.Context, sessionID string, usage ContextUsage) error
}

var (
	// ErrConversationSummaryNotFound is returned when a session has no summary yet.
	ErrConversationSummaryNotFound = errors.New("conversation summary not found")
	// ErrSummaryMessageNotInSession is returned by Save when the summary's
	// CoversUntilMessageID does not belong to its session.
	ErrSummaryMessageNotInSession = errors.New("summary message does not belong to the session")
)

// ConversationSummaryRepository persists the single current ConversationSummary
// of each session.
type ConversationSummaryRepository interface {
	// Get returns the session's summary, or ErrConversationSummaryNotFound.
	Get(ctx context.Context, sessionID string) (ConversationSummary, error)
	// Save inserts or replaces the session's summary. It returns
	// ErrSummaryMessageNotInSession, persisting nothing, when
	// CoversUntilMessageID belongs to another session or does not exist.
	Save(ctx context.Context, summary ConversationSummary) error
}

// ErrMessageNotInSession is returned by ListAfter when the cursor message does
// not exist or belongs to another session.
var ErrMessageNotInSession = errors.New("message does not belong to the session")

// MessageRepository persists the Messages exchanged within a study Session.
type MessageRepository interface {
	Append(ctx context.Context, message Message) error
	ListBySession(ctx context.Context, sessionID string) ([]Message, error)
	// ListAfter returns the session's messages that come after afterMessageID,
	// in conversation order. An empty afterMessageID returns every message. It
	// returns ErrMessageNotInSession when the cursor is not a message of the
	// session.
	ListAfter(ctx context.Context, sessionID, afterMessageID string) ([]Message, error)
}

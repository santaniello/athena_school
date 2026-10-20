package study

import (
	"context"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Page sizes for reading a session's history: Resume and ListMessagesBefore
// return DefaultMessagePageSize messages unless asked for another size, and
// never more than MaxMessagePageSize.
const (
	DefaultMessagePageSize = 50
	MaxMessagePageSize     = 200
)

// MessageHistoryPage is one chronological slice of a session's history, each
// message paired with its persisted Sources. NextCursor is the opaque cursor
// to pass to ListMessagesBefore for the next older page, and is empty when
// HasMore is false.
type MessageHistoryPage struct {
	Messages   []MessageWithSources
	HasMore    bool
	NextCursor string
}

// ListMessagesBefore returns the page of sessionID's history that precedes
// the message cursor. limit is clamped to [1, MaxMessagePageSize]; a limit
// below 1 means DefaultMessagePageSize. It returns
// domainstudy.ErrInvalidMessageCursor for an empty cursor and
// domainstudy.ErrMessageNotInSession for a cursor that is not a message of
// the session, in both cases without returning any data. Sources are loaded
// only for the returned messages.
func (s *Service) ListMessagesBefore(ctx context.Context, sessionID, cursor string, limit int) (MessageHistoryPage, error) {
	before, err := domainstudy.NewMessageCursor(cursor)
	if err != nil {
		return MessageHistoryPage{}, fmt.Errorf("study: reading messages before %q: %w", cursor, err)
	}
	return s.readMessagePage(ctx, sessionID, &before, limit)
}

// readMessagePage reads one page before the given cursor (nil for the
// latest page) and attaches the persisted sources of its messages.
func (s *Service) readMessagePage(ctx context.Context, sessionID string, before *domainstudy.MessageCursor, limit int) (MessageHistoryPage, error) {
	limit = clampMessagePageSize(limit)

	// One extra message tells NewMessagePage whether older history exists.
	newestFirst, err := s.messages.ListPage(ctx, sessionID, before, limit+1)
	if err != nil {
		return MessageHistoryPage{}, fmt.Errorf("study: loading history: %w", err)
	}
	page := domainstudy.NewMessagePage(newestFirst, limit)

	messageIDs := make([]string, len(page.Messages))
	for i, message := range page.Messages {
		messageIDs[i] = message.ID
	}
	sourcesByMessage, err := s.messageSources.ListByMessageIDs(ctx, messageIDs)
	if err != nil {
		return MessageHistoryPage{}, fmt.Errorf("study: loading message sources: %w", err)
	}

	result := MessageHistoryPage{
		Messages: make([]MessageWithSources, len(page.Messages)),
		HasMore:  page.HasMore,
	}
	for i, message := range page.Messages {
		result.Messages[i] = MessageWithSources{Message: message, Sources: sourcesByMessage[message.ID]}
	}
	if page.NextCursor != nil {
		result.NextCursor = page.NextCursor.MessageID()
	}
	return result, nil
}

func clampMessagePageSize(limit int) int {
	if limit < 1 {
		return DefaultMessagePageSize
	}
	return min(limit, MaxMessagePageSize)
}

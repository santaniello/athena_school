package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// MessageSourceRepository is the SQLite-backed implementation of
// domainknowledge.MessageSourceRepository.
type MessageSourceRepository struct {
	db *sql.DB
}

// NewMessageSourceRepository creates a MessageSourceRepository backed by
// db. db must already have its migrations applied (see Open).
func NewMessageSourceRepository(db *sql.DB) *MessageSourceRepository {
	return &MessageSourceRepository{db: db}
}

// Save replaces messageID's persisted sources with the given ones, in
// order. It routes through execer so it transparently joins the caller's
// enclosing WithinTx transaction (streamAndPersist always calls it
// alongside messages.Append) instead of opening its own — db.go caps the
// pool at one connection, so a second, independent BeginTx from inside an
// already-open transaction would block forever waiting for a connection
// that transaction itself is holding.
func (r *MessageSourceRepository) Save(ctx context.Context, messageID string, sources []domainknowledge.Source) error {
	exec := execer(ctx, r.db)
	if _, err := exec.ExecContext(ctx, `DELETE FROM message_sources WHERE message_id = ?`, messageID); err != nil {
		return fmt.Errorf("sqlite: clearing message sources: %w", err)
	}
	for position, source := range sources {
		_, err := exec.ExecContext(ctx, `
			INSERT INTO message_sources (
				message_id, position, chunk_id, item_id, source_type, file_path, heading, concept, score, excerpt
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			messageID, position, source.ChunkID, source.ItemID, source.SourceType,
			source.FilePath, source.Heading, source.Concept, source.Score, source.Excerpt,
		)
		if err != nil {
			return fmt.Errorf("sqlite: saving message source: %w", err)
		}
	}
	return nil
}

// ListBySession returns every persisted source for every message in
// sessionID, keyed by message ID, ordered by position within each message.
func (r *MessageSourceRepository) ListBySession(ctx context.Context, sessionID string) (map[string][]domainknowledge.Source, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT message_sources.message_id, chunk_id, item_id, source_type, file_path, heading, concept, score, excerpt
		FROM message_sources
		JOIN messages ON messages.id = message_sources.message_id
		WHERE messages.session_id = ?
		ORDER BY message_sources.message_id, message_sources.position`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing message sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]domainknowledge.Source)
	for rows.Next() {
		var messageID string
		var source domainknowledge.Source
		if err := rows.Scan(
			&messageID, &source.ChunkID, &source.ItemID, &source.SourceType,
			&source.FilePath, &source.Heading, &source.Concept, &source.Score, &source.Excerpt,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scanning message source: %w", err)
		}
		result[messageID] = append(result[messageID], source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating message sources: %w", err)
	}
	return result, nil
}

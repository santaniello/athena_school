package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// DocumentRepository is the SQLite-backed implementation of
// knowledge.DocumentRepository.
type DocumentRepository struct {
	db *sql.DB
}

// NewDocumentRepository creates a DocumentRepository backed by db. db must
// already have its migrations applied (see Open).
func NewDocumentRepository(db *sql.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// Save inserts content for itemID, or replaces it if a document is already
// stored for that item (a re-import).
func (r *DocumentRepository) Save(ctx context.Context, itemID, sessionID, content string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO knowledge_documents (item_id, session_id, content) VALUES (?, ?, ?)
		 ON CONFLICT(item_id) DO UPDATE SET session_id = excluded.session_id, content = excluded.content`,
		itemID, sessionID, content,
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving knowledge document %s: %w", itemID, err)
	}
	return nil
}

// Get returns itemID's stored text within sessionID, or
// knowledge.ErrDocumentNotFound if none is stored — including when itemID
// exists but belongs to a different session, indistinguishable from not
// existing at all (same rule as ingest.Service.RemoveSource's ownership
// check).
func (r *DocumentRepository) Get(ctx context.Context, sessionID, itemID string) (string, error) {
	var content string
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT content FROM knowledge_documents WHERE item_id = ? AND session_id = ?`, itemID, sessionID,
	).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", knowledge.ErrDocumentNotFound
	}
	if err != nil {
		return "", fmt.Errorf("sqlite: getting knowledge document %s: %w", itemID, err)
	}
	return content, nil
}

// DeleteByItemID removes itemID's stored text within sessionID. It is a
// no-op, not an error, when no document is stored.
func (r *DocumentRepository) DeleteByItemID(ctx context.Context, sessionID, itemID string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`DELETE FROM knowledge_documents WHERE session_id = ? AND item_id = ?`, sessionID, itemID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: deleting knowledge document %s: %w", itemID, err)
	}
	return nil
}

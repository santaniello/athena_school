package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

// Resetter is the SQLite-backed implementation of reset.Resetter.
type Resetter struct {
	db *sql.DB
}

// NewResetter creates a Resetter backed by db. db must already have its
// migrations applied (see Open).
func NewResetter(db *sql.DB) *Resetter {
	return &Resetter{db: db}
}

// Reset permanently deletes every study session (and its messages),
// every non-default folder, and every knowledge-domain row, atomically.
// usage survives detached (session_id set NULL by its own foreign key),
// matching how deleting one session already behaves — see
// TestOpen_migratesLegacyForeignKeysAndDetachesUsageWithoutRemovingIt.
// The deletion order matters: knowledge_items and
// knowledge_reconciliation_proposals are deleted before knowledge_evidence
// because their junction tables (knowledge_item_evidence,
// knowledge_reconciliation_evidence) reference it and only cascade from
// the item/proposal side.
func (r *Resetter) Reset(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: beginning reset transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM sessions`, nil},
		{`DELETE FROM folders WHERE id <> ?`, []any{domainfolder.DefaultFolderID}},
		{`DELETE FROM knowledge_items`, nil},
		{`DELETE FROM knowledge_reconciliation_proposals`, nil},
		{`DELETE FROM knowledge_evidence`, nil},
		{`DELETE FROM knowledge_chunks`, nil},
		{`DELETE FROM ingested_files`, nil},
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			return fmt.Errorf("sqlite: resetting local data: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing reset transaction: %w", err)
	}
	committed = true
	return nil
}

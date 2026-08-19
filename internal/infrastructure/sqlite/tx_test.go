package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func countFolders(t *testing.T, db *sql.DB, id string) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM folders WHERE id = ?`, id).Scan(&count))
	return count
}

func TestSQLTransactor_WithinTx_commitsWritesMadeThroughExecer(t *testing.T) {
	// Given a transactor over a real database
	db := newTestDB(t)
	transactor := NewSQLTransactor(db)

	// When fn writes through execer(ctx, db) and returns no error
	err := transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		_, execErr := execer(ctx, db).ExecContext(ctx,
			`INSERT INTO folders (id, name, is_default, created_at) VALUES (?, ?, 0, CURRENT_TIMESTAMP)`,
			"tx-commit", "Committed",
		)
		return execErr
	})

	// Then the write is committed and visible outside the transaction
	require.NoError(t, err)
	assert.Equal(t, 1, countFolders(t, db, "tx-commit"))
}

func TestSQLTransactor_WithinTx_rollsBackWritesWhenFnReturnsError(t *testing.T) {
	// Given a transactor over a real database
	db := newTestDB(t)
	transactor := NewSQLTransactor(db)
	fnErr := errors.New("boom")

	// When fn writes through execer(ctx, db) but then fails
	err := transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		_, execErr := execer(ctx, db).ExecContext(ctx,
			`INSERT INTO folders (id, name, is_default, created_at) VALUES (?, ?, 0, CURRENT_TIMESTAMP)`,
			"tx-rollback", "Rolled back",
		)
		require.NoError(t, execErr)
		return fnErr
	})

	// Then fn's error is returned and the write never became visible
	assert.ErrorIs(t, err, fnErr)
	assert.Equal(t, 0, countFolders(t, db, "tx-rollback"))
}

func TestExecer_usesPooledDB_whenContextHasNoActiveTransaction(t *testing.T) {
	// Given a database and a plain context with no transaction on it
	db := newTestDB(t)
	ctx := context.Background()

	// When writing through execer(ctx, db) outside of WithinTx
	_, err := execer(ctx, db).ExecContext(ctx,
		`INSERT INTO folders (id, name, is_default, created_at) VALUES (?, ?, 0, CURRENT_TIMESTAMP)`,
		"no-tx", "Auto-commit",
	)

	// Then the write is immediately visible (auto-commit), proving execer
	// fell back to db rather than requiring an active transaction
	require.NoError(t, err)
	assert.Equal(t, 1, countFolders(t, db, "no-tx"))
}

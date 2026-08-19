package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// sqlExecer is satisfied by both *sql.DB and *sql.Tx, letting every
// repository route its calls through execer(ctx, r.db) instead of r.db
// directly, so they transparently participate in a transaction started by
// SQLTransactor.WithinTx without changing any repository's constructor or
// the domain Repository interfaces they implement.
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// execer returns the *sql.Tx stashed on ctx by an enclosing WithinTx call,
// or db itself when there is none — a plain, auto-committing call.
func execer(ctx context.Context, db *sql.DB) sqlExecer {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// SQLTransactor runs a sequence of repository calls inside one SQLite
// transaction. It is the infrastructure implementation of any application
// package's own Transactor port (see internal/application/ingest).
type SQLTransactor struct {
	db *sql.DB
}

// NewSQLTransactor creates a SQLTransactor backed by db.
func NewSQLTransactor(db *sql.DB) *SQLTransactor {
	return &SQLTransactor{db: db}
}

// WithinTx begins a transaction, stashes it on the context passed to fn so
// every repository call made through it shares the same transaction (see
// execer above), and commits on success. An error from fn rolls the
// transaction back; the original error is returned, joined with a
// rollback failure if one also occurs. A commit failure is returned as-is.
// A deferred best-effort rollback guards against fn panicking: db.go caps
// the pool at one connection, so a transaction left open by an unwound
// panic would otherwise wedge every later call behind busy_timeout.
func (t *SQLTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: beginning transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("sqlite: rolling back transaction: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing transaction: %w", err)
	}
	committed = true
	return nil
}

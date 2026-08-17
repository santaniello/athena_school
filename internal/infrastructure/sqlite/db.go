// Package sqlite provides the local SQLite-backed infrastructure adapters
// (internal/domain/auth.AccountRepository today; more ports as later specs
// add tables — see specs/phases/phase-01-desktop-mvp/07-sqlite.md).
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// Open opens the SQLite database at path and applies all migrations. It is
// safe to call on every launch: migrations are idempotent and a no-op once
// already applied.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: opening database: %w", err)
	}
	for _, migration := range migrations {
		if err := migration(db); err != nil {
			return nil, errors.Join(
				fmt.Errorf("sqlite: applying migration: %w", err),
				db.Close(),
			)
		}
	}
	return db, nil
}

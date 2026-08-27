// Package sqlite provides the local SQLite-backed infrastructure adapters
// (internal/domain/auth.AccountRepository today; more ports as later specs
// add tables — see specs/phases/phase-01-desktop-mvp/07-sqlite.md).
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// Open opens the SQLite database at path and applies all migrations. It is
// safe to call on every launch: migrations are idempotent and a no-op once
// already applied.
//
// database/sql pools connections for drivers built around concurrent
// access (Postgres, MySQL, ...), but SQLite allows only one writer at a
// time and modernc.org/sqlite has no busy_timeout by default — two pooled
// connections both trying to write fail immediately with "database is
// locked (5) (SQLITE_BUSY)" instead of one just waiting its turn. Capping
// the pool at a single connection routes every query through it, so
// database/sql itself serializes access instead of opening a second,
// colliding connection; the busy_timeout PRAGMA is an extra safety net for
// any lock briefly held by something outside this pool (e.g. a read while
// a migration is still applying elsewhere).
func Open(path string) (*sql.DB, error) {
	// foreign_keys is set via the DSN, not a post-open Exec, because PRAGMAs
	// are per-connection in SQLite: an Exec only configures whichever
	// physical connection happens to service that one call, and
	// database/sql can silently open a fresh, differently-configured
	// connection later (e.g. after driver.ErrBadConn). A DSN parameter is
	// applied by the driver to every connection this *sql.DB ever opens,
	// including a replacement, and is scoped to this *sql.DB alone — unlike
	// a driver-level connection hook, it has no effect on any other
	// sql.Open("sqlite", ...) call in the process, such as the raw
	// connections test fixtures open to build pre-migration legacy data.
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("sqlite: opening database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return nil, errors.Join(fmt.Errorf("sqlite: setting busy_timeout: %w", err), db.Close())
	}
	for _, migration := range migrations {
		if err := migration(db); err != nil {
			return nil, errors.Join(
				fmt.Errorf("sqlite: applying migration: %w", err),
				db.Close(),
			)
		}
	}
	if err := checkForeignKeys(db); err != nil {
		return nil, errors.Join(fmt.Errorf("sqlite: foreign key check: %w", err), db.Close())
	}
	return db, nil
}

func checkForeignKeys(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return rows.Err()
	}
	var table, parent string
	var rowID sql.NullInt64
	var foreignKeyID int
	if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
		return err
	}
	rowDescription := "NULL" // a WITHOUT ROWID table reports no row id
	if rowID.Valid {
		rowDescription = strconv.FormatInt(rowID.Int64, 10)
	}
	return fmt.Errorf("table %q row %s references missing parent %q through foreign key %d", table, rowDescription, parent, foreignKeyID)
}

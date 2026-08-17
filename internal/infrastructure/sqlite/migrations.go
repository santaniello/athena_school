package sqlite

import "database/sql"

// migrations are idempotent DDL/DML steps applied in order on every Open
// call. Additive by design: a new table is a new entry in this slice, no
// version-tracking table needed. Steps beyond a plain CREATE TABLE (e.g.
// adding a column to an existing table) use PRAGMA/conditional logic
// instead of a bare statement, since SQLite has no "ADD COLUMN IF NOT
// EXISTS" and re-running one unconditionally would error on the second
// Open.
var migrations = []func(*sql.DB) error{
	execSQL(`CREATE TABLE IF NOT EXISTS accounts (
		id            TEXT PRIMARY KEY,
		email         TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at    DATETIME
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS usage (
		id            TEXT PRIMARY KEY,
		session_id    TEXT REFERENCES sessions(id),
		model         TEXT,
		input_tokens  INTEGER,
		output_tokens INTEGER,
		cost          REAL,
		created_at    DATETIME
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		topic      TEXT,
		mode       TEXT,
		started_at DATETIME,
		ended_at   DATETIME
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		session_id TEXT REFERENCES sessions(id),
		role       TEXT,
		content    TEXT,
		created_at DATETIME
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS folders (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME
	)`),
	execSQL(`INSERT OR IGNORE INTO folders (id, name, is_default, created_at)
		VALUES ('default', 'General', 1, CURRENT_TIMESTAMP)`),
}

// execSQL adapts a plain DDL/DML statement, unconditionally safe to
// re-run (CREATE TABLE IF NOT EXISTS, INSERT OR IGNORE, ...), to the
// migration step signature.
func execSQL(stmt string) func(*sql.DB) error {
	return func(db *sql.DB) error {
		_, err := db.Exec(stmt)
		return err
	}
}

package sqlite

// migrations are idempotent DDL statements applied in order on every Open
// call. Additive by design: a new table is a new entry in this slice, no
// version-tracking table needed.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS accounts (
		id            TEXT PRIMARY KEY,
		email         TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at    DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS usage (
		id            TEXT PRIMARY KEY,
		session_id    TEXT,
		model         TEXT,
		input_tokens  INTEGER,
		output_tokens INTEGER,
		cost          REAL,
		created_at    DATETIME
	)`,
}

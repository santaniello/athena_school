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
		started_at DATETIME
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
	addSessionsFolderIDColumn,
}

// addSessionsFolderIDColumn adds sessions.folder_id if it does not already
// exist (SQLite has no "ADD COLUMN IF NOT EXISTS") and backfills any
// existing rows to the default folder, so folder_id is always populated
// even though it cannot be declared NOT NULL on an ALTER TABLE.
func addSessionsFolderIDColumn(db *sql.DB) error {
	hasFolderID, err := sessionsHasFolderIDColumn(db)
	if err != nil {
		return err
	}

	if !hasFolderID {
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN folder_id TEXT REFERENCES folders(id)`); err != nil {
			return err
		}
	}

	_, err = db.Exec(`UPDATE sessions SET folder_id = 'default' WHERE folder_id IS NULL`)
	return err
}

// sessionsHasFolderIDColumn reports whether the sessions table already has
// a folder_id column.
func sessionsHasFolderIDColumn(db *sql.DB) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(sessions)`)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == "folder_id" {
			return true, nil
		}
	}
	return false, rows.Err()
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

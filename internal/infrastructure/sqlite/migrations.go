package sqlite

import (
	"database/sql"
	"fmt"
)

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
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_items (
		id               TEXT PRIMARY KEY,
		topic            TEXT,
		concept          TEXT,
		definition       TEXT,
		properties       TEXT, -- JSON array
		trade_offs       TEXT, -- JSON array
		related_concepts TEXT, -- JSON array
		source           TEXT,
		status           TEXT DEFAULT 'draft',
		created_at       DATETIME,
		updated_at       DATETIME
	)`),
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_items_status_created_at
		ON knowledge_items(status, created_at)`),
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_items_topic
		ON knowledge_items(topic)`),
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_chunks (
		id          TEXT PRIMARY KEY,
		source      TEXT,
		topic       TEXT,
		status      TEXT,
		item_id     TEXT,
		source_path TEXT, -- canonical absolute identity; set for imported_doc
		file_path   TEXT, -- stable first-import relative/display path
		heading     TEXT,
		content     TEXT,
		embedding   BLOB, -- tightly-packed little-endian float32
		embedding_model TEXT NOT NULL,
		item_updated_at DATETIME, -- NULL for imported_doc
		created_at DATETIME
	)`),
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_file_path ON knowledge_chunks(file_path)`),
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_item_id ON knowledge_chunks(item_id)`),
	addKnowledgeChunksSourcePathColumn,
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_source_path ON knowledge_chunks(source_path)`),
	execSQL(`CREATE TABLE IF NOT EXISTS ingested_files (
		source_path     TEXT PRIMARY KEY,
		file_path       TEXT NOT NULL,
		mtime_unix_nano INTEGER NOT NULL,
		embedding_model TEXT NOT NULL,
		chunk_count     INTEGER NOT NULL,
		item_id         TEXT NOT NULL,
		ingested_at     DATETIME
	)`),
	migrateIngestedFilesToSourcePathSchema,
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
	return hasColumn(db, "sessions", "folder_id")
}

// hasColumn reports whether table already has a column named column.
func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
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
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// tableIsEmpty reports whether table currently holds zero rows.
func tableIsEmpty(db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// addKnowledgeChunksSourcePathColumn adds knowledge_chunks.source_path when
// an older schema (predating this column) is detected. This pre-release
// schema correction assumes no deployed or local knowledge records must
// survive it: it verifies the table holds no rows and fails rather than
// silently proceeding if that premise is violated, instead of attempting a
// heuristic backfill.
func addKnowledgeChunksSourcePathColumn(db *sql.DB) error {
	hasSourcePath, err := hasColumn(db, "knowledge_chunks", "source_path")
	if err != nil {
		return err
	}
	if hasSourcePath {
		return nil
	}

	empty, err := tableIsEmpty(db, "knowledge_chunks")
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("sqlite: knowledge_chunks predates source_path and is not empty; refusing to alter it")
	}

	_, err = db.Exec(`ALTER TABLE knowledge_chunks ADD COLUMN source_path TEXT`)
	return err
}

// migrateIngestedFilesToSourcePathSchema rebuilds the pre-release
// ingested_files table (file_path PRIMARY KEY, mtime) into its
// source_path-keyed, nanosecond-precision replacement when an older schema
// is detected. There are no deployed or local knowledge records that must
// survive this change, but as a safety net it still verifies the table
// holds no rows and fails rather than dropping data if that premise is
// violated. Idempotent on the next Open, since a rebuilt table already has
// a source_path column.
func migrateIngestedFilesToSourcePathSchema(db *sql.DB) error {
	hasSourcePath, err := hasColumn(db, "ingested_files", "source_path")
	if err != nil {
		return err
	}
	if hasSourcePath {
		return nil
	}

	empty, err := tableIsEmpty(db, "ingested_files")
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("sqlite: ingested_files predates source_path and is not empty; refusing to drop it")
	}

	_, err = db.Exec(`DROP TABLE ingested_files`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE ingested_files (
		source_path     TEXT PRIMARY KEY,
		file_path       TEXT NOT NULL,
		mtime_unix_nano INTEGER NOT NULL,
		embedding_model TEXT NOT NULL,
		chunk_count     INTEGER NOT NULL,
		item_id         TEXT NOT NULL,
		ingested_at     DATETIME
	)`)
	return err
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

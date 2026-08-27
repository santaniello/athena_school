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
		session_id    TEXT REFERENCES sessions(id) ON DELETE SET NULL,
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
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
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
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_evidence (
		id           TEXT PRIMARY KEY,
		origin_type  TEXT NOT NULL,
		origin_id    TEXT NOT NULL,
		source_label TEXT NOT NULL,
		excerpt      TEXT NOT NULL,
		created_at   DATETIME NOT NULL,
		UNIQUE (origin_type, origin_id, excerpt)
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_item_evidence (
		item_id     TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
		evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
		PRIMARY KEY (item_id, evidence_id)
	)`),
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_item_evidence_evidence
		ON knowledge_item_evidence(evidence_id)`),
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
	addSessionsContextColumns,
	migrateSessionForeignKeyActions,
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

// addSessionsContextColumns adds the sessions.context_* columns (see
// specs/phases/phase-02-knowledge-engine/06-study-context-limits.md) if
// they do not already exist. Unlike addSessionsFolderIDColumn, every
// column here declares a NOT NULL DEFAULT, which SQLite backfills onto
// existing rows as part of ADD COLUMN itself — no separate UPDATE pass is
// needed.
func addSessionsContextColumns(db *sql.DB) error {
	columns := []struct{ name, ddl string }{
		{"context_state", `ALTER TABLE sessions ADD COLUMN context_state TEXT NOT NULL DEFAULT 'normal'`},
		{"context_model", `ALTER TABLE sessions ADD COLUMN context_model TEXT NOT NULL DEFAULT ''`},
		{"context_used_tokens", `ALTER TABLE sessions ADD COLUMN context_used_tokens INTEGER NOT NULL DEFAULT 0`},
		{"context_length", `ALTER TABLE sessions ADD COLUMN context_length INTEGER NOT NULL DEFAULT 0`},
		{"context_estimated", `ALTER TABLE sessions ADD COLUMN context_estimated INTEGER NOT NULL DEFAULT 0`},
	}
	for _, column := range columns {
		has, err := hasColumn(db, "sessions", column.name)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := db.Exec(column.ddl); err != nil {
			return err
		}
	}
	return nil
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

// migrateSessionForeignKeyActions upgrades the two pre-enforcement session
// relationships to their ownership semantics: messages are owned by their
// session, while usage remains as an unattributed financial record after a
// session is deleted. A session whose folder_id predates folders existing,
// or points at one that no longer does, is reassigned to the default folder
// (mirroring addSessionsFolderIDColumn's own backfill) rather than deleted,
// since a session is never lost over a stale folder reference. Only a
// message or usage row with no owning session at all is removed, before the
// tables are rebuilt.
func migrateSessionForeignKeyActions(db *sql.DB) error {
	messagesReady, err := hasForeignKeyDeleteAction(db, "messages", "session_id", "sessions", "CASCADE")
	if err != nil {
		return err
	}
	usageReady, err := hasForeignKeyDeleteAction(db, "usage", "session_id", "sessions", "SET NULL")
	if err != nil {
		return err
	}
	if messagesReady && usageReady {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite: beginning session foreign-key migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := []string{
		`UPDATE sessions SET folder_id = 'default'
		 WHERE folder_id IS NULL
		    OR folder_id = ''
		    OR NOT EXISTS (SELECT 1 FROM folders WHERE folders.id = sessions.folder_id)`,
		`DELETE FROM messages
		 WHERE session_id IS NULL
		    OR session_id = ''
		    OR NOT EXISTS (SELECT 1 FROM sessions WHERE sessions.id = messages.session_id)`,
		`UPDATE usage SET session_id = NULL
		 WHERE session_id IS NOT NULL
		   AND (session_id = ''
		        OR NOT EXISTS (SELECT 1 FROM sessions WHERE sessions.id = usage.session_id))`,
		`ALTER TABLE messages RENAME TO messages_before_foreign_keys`,
		`CREATE TABLE messages (
			id         TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role       TEXT,
			content    TEXT,
			created_at DATETIME
		)`,
		`INSERT INTO messages (id, session_id, role, content, created_at)
		 SELECT id, session_id, role, content, created_at FROM messages_before_foreign_keys`,
		`DROP TABLE messages_before_foreign_keys`,
		`ALTER TABLE usage RENAME TO usage_before_foreign_keys`,
		`CREATE TABLE usage (
			id            TEXT PRIMARY KEY,
			session_id    TEXT REFERENCES sessions(id) ON DELETE SET NULL,
			model         TEXT,
			input_tokens  INTEGER,
			output_tokens INTEGER,
			cost          REAL,
			created_at    DATETIME
		)`,
		`INSERT INTO usage (id, session_id, model, input_tokens, output_tokens, cost, created_at)
		 SELECT id, session_id, model, input_tokens, output_tokens, cost, created_at
		 FROM usage_before_foreign_keys`,
		`DROP TABLE usage_before_foreign_keys`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("sqlite: migrating session foreign keys: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing session foreign-key migration: %w", err)
	}
	committed = true
	return nil
}

func hasForeignKeyDeleteAction(db *sql.DB, table, fromColumn, referencedTable, action string) (bool, error) {
	rows, err := db.Query(`PRAGMA foreign_key_list(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, sequence int
		var target, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &sequence, &target, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return false, err
		}
		if target == referencedTable && from == fromColumn && onDelete == action {
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

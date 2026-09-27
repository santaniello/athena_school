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
		created_at DATETIME
	)`),
	addSessionsFolderIDColumn,
	addSessionsContextColumns,
	migrateSessionForeignKeyActions,
	repairSessionsWithInvalidFolder,
	addSessionsGoalColumn,
	// dropAccountsTable removes the local login/account table: no other
	// table ever referenced it by foreign key, and the app is now a
	// single-user local install identified by ~/.athena/profile.json, not
	// by an account row. IF EXISTS keeps this safe to re-run on every Open
	// (a fresh install never created the table in the first place). See
	// specs/phases/phase-01-desktop-mvp/12-remove-local-login.md.
	execSQL(`DROP TABLE IF EXISTS accounts`),
	// message_sources persists the local-knowledge Sources that backed one
	// completed assistant message, so they survive a resume instead of
	// only existing as the transient "study:sources" event. chunk_id/
	// item_id are NOT NULL — retrieval.go never produces a Source with
	// either blank. See
	// specs/phases/phase-02-knowledge-engine/09-persistent-provenance.md.
	execSQL(`CREATE TABLE IF NOT EXISTS message_sources (
		message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		chunk_id    TEXT NOT NULL,
		item_id     TEXT NOT NULL,
		source_type TEXT NOT NULL,
		file_path   TEXT NOT NULL DEFAULT '',
		heading     TEXT NOT NULL DEFAULT '',
		concept     TEXT NOT NULL DEFAULT '',
		score       REAL NOT NULL,
		excerpt     TEXT NOT NULL,
		PRIMARY KEY (message_id, position)
	)`),
	dropFoldersIsDefaultColumn,
	addSessionsFolderIDCascade,
	migrateKnowledgeToDocumentsOnly,
}

// addSessionsFolderIDColumn adds sessions.folder_id if it does not already
// exist (SQLite has no "ADD COLUMN IF NOT EXISTS"). The repair for rows
// left with a missing/dangling folder_id lives separately in
// repairSessionsWithInvalidFolder, positioned after
// migrateSessionForeignKeyActions in the migrations slice — see that
// function's comment for why.
func addSessionsFolderIDColumn(db *sql.DB) error {
	hasFolderID, err := sessionsHasFolderIDColumn(db)
	if err != nil {
		return err
	}
	if hasFolderID {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE sessions ADD COLUMN folder_id TEXT REFERENCES folders(id)`)
	return err
}

// repairSessionsWithInvalidFolder deletes any session whose folder_id is
// missing, empty, or points at a folder that no longer exists — there is no
// fallback folder left to reassign it to, so folder_id is always either
// populated and valid or the row is gone. Its messages go with it via the
// ON DELETE CASCADE already declared on messages.session_id by this point
// (see below for why it's positioned here). This repair runs unconditionally
// on every Open, not gated behind migrateSessionForeignKeyActions's own
// readiness check: that migration only rebuilds messages/usage once, so a
// session that goes stale afterward (e.g. a partial restore of just the
// sessions table from an older backup) would never be repaired if this
// lived there instead.
//
// It runs after migrateSessionForeignKeyActions, not alongside
// addSessionsFolderIDColumn near the top of the slice: deleting a stale
// session here can be the parent side of a usage row that predates
// migrateSessionForeignKeyActions's SET NULL upgrade, and a plain
// REFERENCES sessions(id) with no ON DELETE action blocks the delete
// entirely under foreign_keys=ON. Running after that upgrade guarantees
// usage.session_id already detaches instead of blocking.
func repairSessionsWithInvalidFolder(db *sql.DB) error {
	// Only sessions are deleted explicitly — messages.session_id already
	// declares ON DELETE CASCADE by this point (this runs right after
	// migrateSessionForeignKeyActions, which guarantees it), so the
	// database removes their messages in the same operation.
	_, err := db.Exec(`DELETE FROM sessions WHERE
		folder_id IS NULL
		OR folder_id = ''
		OR NOT EXISTS (SELECT 1 FROM folders WHERE folders.id = sessions.folder_id)`)
	return err
}

// dropFoldersIsDefaultColumn removes folders.is_default: there is no
// concept of a default/fallback folder any more, so the column is dead
// weight. Guarded by hasColumn so a fresh install — whose folders table
// never had the column — is a no-op.
//
// Rebuilt via create-under-a-temporary-name/copy/drop-old/rename-into-place,
// deliberately NOT the more obvious rename-old-away/create/copy/drop-old
// order: sessions.folder_id references folders(id), and renaming folders
// away would make SQLite silently rewrite that reference to the temporary
// name, leaving it dangling forever once the temporary table is dropped
// and breaking every later query against sessions. Renaming a fresh
// temporary table (that nothing references) into the now-free "folders"
// name at the end avoids that rewrite entirely. foreign_keys is also
// disabled for the rebuild: with it on, dropping "folders" while sessions
// still references it (NO ACTION at this point in the migration order) can
// itself fail the whole migration with a constraint error, even though
// nothing here actually touches sessions' rows or schema. See
// specs/phases/phase-01-desktop-mvp/15-remove-default-folder.md.
func dropFoldersIsDefaultColumn(db *sql.DB) error {
	hasIsDefault, err := hasColumn(db, "folders", "is_default")
	if err != nil {
		return err
	}
	if !hasIsDefault {
		return nil
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("sqlite: disabling foreign keys for folders.is_default removal: %w", err)
	}
	defer func() { _, _ = db.Exec(`PRAGMA foreign_keys = ON`) }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite: beginning folders.is_default removal: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := []string{
		`CREATE TABLE folders_without_is_default (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			created_at DATETIME
		)`,
		`INSERT INTO folders_without_is_default (id, name, created_at)
		 SELECT id, name, created_at FROM folders`,
		`DROP TABLE folders`,
		`ALTER TABLE folders_without_is_default RENAME TO folders`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("sqlite: removing folders.is_default: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing folders.is_default removal: %w", err)
	}
	committed = true
	return nil
}

// addSessionsFolderIDCascade upgrades sessions.folder_id to
// NOT NULL ... ON DELETE CASCADE against folders(id): deleting a folder now
// deletes its sessions (see application/folder.DeleteFolder), so the schema
// must agree instead of leaving that invariant to application code alone.
// NOT NULL was never declarable when the column was first added via a plain
// ALTER TABLE ADD COLUMN (see addSessionsFolderIDColumn) — SQLite only
// allows that with a table rebuild, which this migration already is. Safe
// because repairSessionsWithInvalidFolder, positioned earlier in the
// migrations slice, has already deleted every session with a missing
// folder_id by the time this runs. Guarded by hasForeignKeyDeleteAction so
// an already-migrated database is a no-op.
//
// Rebuilt via create-under-a-temporary-name/copy/drop-old/rename-into-place
// — see dropFoldersIsDefaultColumn's comment for why, applied here to
// sessions instead of folders: renaming sessions away would leave
// messages.session_id and usage.session_id permanently dangling once the
// temporary table is dropped, breaking every later query against them.
// foreign_keys is disabled for the same reason as there: dropping
// "sessions" while messages/usage still reference it would otherwise
// cascade-delete or block on their rows, even though nothing here touches
// their data. See specs/phases/phase-01-desktop-mvp/15-remove-default-folder.md.
func addSessionsFolderIDCascade(db *sql.DB) error {
	ready, err := hasForeignKeyDeleteAction(db, "sessions", "folder_id", "folders", "CASCADE")
	if err != nil {
		return err
	}
	if ready {
		return nil
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("sqlite: disabling foreign keys for sessions.folder_id cascade migration: %w", err)
	}
	defer func() { _, _ = db.Exec(`PRAGMA foreign_keys = ON`) }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite: beginning sessions.folder_id cascade migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := []string{
		`CREATE TABLE sessions_with_folder_cascade (
			id                   TEXT PRIMARY KEY,
			topic                TEXT,
			mode                 TEXT,
			started_at           DATETIME,
			folder_id            TEXT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
			context_state        TEXT NOT NULL DEFAULT 'normal',
			context_model        TEXT NOT NULL DEFAULT '',
			context_used_tokens  INTEGER NOT NULL DEFAULT 0,
			context_length       INTEGER NOT NULL DEFAULT 0,
			context_estimated    INTEGER NOT NULL DEFAULT 0,
			goal                 TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT INTO sessions_with_folder_cascade (
			id, topic, mode, started_at, folder_id,
			context_state, context_model, context_used_tokens, context_length, context_estimated, goal
		 )
		 SELECT id, topic, mode, started_at, folder_id,
			context_state, context_model, context_used_tokens, context_length, context_estimated, goal
		 FROM sessions`,
		`DROP TABLE sessions`,
		`ALTER TABLE sessions_with_folder_cascade RENAME TO sessions`,
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("sqlite: migrating sessions.folder_id cascade: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing sessions.folder_id cascade migration: %w", err)
	}
	committed = true
	return nil
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

// addSessionsGoalColumn adds sessions.goal if it does not already exist. It
// declares NOT NULL DEFAULT ”, which SQLite backfills onto existing rows as
// part of ADD COLUMN itself, so a session created before this column existed
// simply has goal = ” — see
// specs/phases/phase-01-desktop-mvp/14-session-goal.md for why that renders
// no "Goal: ..." fragment in the system prompt instead of falling back to
// anything else.
func addSessionsGoalColumn(db *sql.DB) error {
	has, err := hasColumn(db, "sessions", "goal")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE sessions ADD COLUMN goal TEXT NOT NULL DEFAULT ''`)
	return err
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

// migrateSessionForeignKeyActions upgrades the two pre-enforcement session
// relationships to their ownership semantics: messages are owned by their
// session, while usage remains as an unattributed financial record after a
// session is deleted. Only a message or usage row with no owning session at
// all is removed, before the tables are rebuilt; sessions.folder_id itself
// is repaired separately, unconditionally, by addSessionsFolderIDColumn.
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

// documentsOnlyKnowledgeSchema is the final shape of the knowledge tables:
// a session owns its imported documents (knowledge_items is the record that
// owns each document's chunks), their chunks and their dedup state.
var documentsOnlyKnowledgeSchema = []string{
	`CREATE TABLE knowledge_items (
		id               TEXT PRIMARY KEY,
		session_id       TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		topic            TEXT,
		concept          TEXT,
		definition       TEXT,
		properties       TEXT, -- JSON array
		trade_offs       TEXT, -- JSON array
		related_concepts TEXT, -- JSON array
		source           TEXT,
		created_at       DATETIME,
		updated_at       DATETIME
	)`,
	`CREATE INDEX idx_knowledge_items_session_id ON knowledge_items(session_id)`,
	`CREATE TABLE knowledge_chunks (
		id              TEXT PRIMARY KEY,
		session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		source          TEXT,
		topic           TEXT,
		item_id         TEXT,
		source_path     TEXT, -- canonical absolute identity of the imported document
		file_path       TEXT, -- stable first-import relative/display path
		heading         TEXT,
		content         TEXT,
		embedding       BLOB, -- tightly-packed little-endian float32
		embedding_model TEXT NOT NULL,
		created_at      DATETIME
	)`,
	`CREATE INDEX idx_knowledge_chunks_session_id ON knowledge_chunks(session_id)`,
	`CREATE INDEX idx_knowledge_chunks_file_path ON knowledge_chunks(file_path)`,
	`CREATE INDEX idx_knowledge_chunks_item_id ON knowledge_chunks(item_id)`,
	`CREATE INDEX idx_knowledge_chunks_source_path ON knowledge_chunks(session_id, source_path)`,
	`CREATE TABLE ingested_files (
		session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		source_path     TEXT NOT NULL,
		file_path       TEXT NOT NULL,
		mtime_unix_nano INTEGER NOT NULL,
		embedding_model TEXT NOT NULL,
		chunk_count     INTEGER NOT NULL,
		item_id         TEXT NOT NULL,
		ingested_at     DATETIME,
		PRIMARY KEY (session_id, source_path)
	)`,
}

// knowledgeTablesToRebuild lists every table migrateKnowledgeToDocumentsOnly
// drops before it creates the final schema, children before the tables they
// reference. The first five only conversation extraction used.
var knowledgeTablesToRebuild = []string{
	"knowledge_reconciliation_evidence",
	"knowledge_reconciliation_proposals",
	"knowledge_item_relations",
	"knowledge_item_evidence",
	"knowledge_evidence",
	"knowledge_chunks",
	"knowledge_items",
	"ingested_files",
}

// migrateKnowledgeToDocumentsOnly leaves the knowledge tables in their final,
// documents-only shape: no status, no normalized concept, no item-staleness
// column, and none of the tables conversation extraction used (evidence,
// relations, reconciliation proposals). It runs only while knowledge_items is
// not already in that shape — a fresh database, or one from before this
// change — so the knowledge tables are never rebuilt twice.
//
// Rows are discarded on purpose: nothing is deployed anywhere it must
// survive and the only database in use held no imported documents, so there
// is no copy path (see decision 6 of
// specs/phases/phase-02-knowledge-engine/16-remove-conversation-extraction.md,
// which follows the same call spec 15 made). Sessions, messages and
// message_sources are untouched; a message_sources row that names a dropped
// item keeps rendering from its own stored columns.
//
// The knowledge tables' CREATE statements live only here — earlier steps must
// never create them, or every Open would bring the dropped tables back before
// this step could run again. It sits after every sessions rebuild
// (addSessionsFolderIDCascade) because dropping and recreating sessions
// cascades into whatever references it.
func migrateKnowledgeToDocumentsOnly(db *sql.DB) error {
	owned, err := hasColumn(db, "knowledge_items", "session_id")
	if err != nil {
		return err
	}
	hasStatus, err := hasColumn(db, "knowledge_items", "status")
	if err != nil {
		return err
	}
	if owned && !hasStatus {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite: beginning documents-only knowledge migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := make([]string, 0, len(knowledgeTablesToRebuild)+len(documentsOnlyKnowledgeSchema))
	for _, table := range knowledgeTablesToRebuild {
		statements = append(statements, `DROP TABLE IF EXISTS `+table)
	}
	statements = append(statements, documentsOnlyKnowledgeSchema...)
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("sqlite: migrating knowledge to documents only: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing documents-only knowledge migration: %w", err)
	}
	committed = true
	return nil
}

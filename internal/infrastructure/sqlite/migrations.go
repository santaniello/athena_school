package sqlite

import (
	"database/sql"
	"fmt"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
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
	addKnowledgeItemsNormalizedConceptColumn,
	execSQL(`CREATE INDEX IF NOT EXISTS idx_knowledge_items_topic_normalized_concept
		ON knowledge_items(topic, normalized_concept)`),
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
	repairSessionsWithInvalidFolder,
	addSessionsGoalColumn,
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_reconciliation_proposals (
		id                 TEXT PRIMARY KEY,
		action             TEXT NOT NULL,
		status             TEXT NOT NULL,
		candidate_snapshot TEXT NOT NULL, -- validated JSON Item snapshot
		target_item_id     TEXT,
		target_updated_at  DATETIME,
		reason             TEXT NOT NULL,
		changes            TEXT NOT NULL, -- validated JSON ItemChanges
		created_at         DATETIME NOT NULL,
		resolved_at        DATETIME
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_reconciliation_evidence (
		proposal_id TEXT NOT NULL REFERENCES knowledge_reconciliation_proposals(id) ON DELETE CASCADE,
		evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
		PRIMARY KEY (proposal_id, evidence_id)
	)`),
	execSQL(`CREATE TABLE IF NOT EXISTS knowledge_item_relations (
		from_item_id  TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
		to_item_id    TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
		relation_type TEXT NOT NULL,
		created_at    DATETIME NOT NULL,
		PRIMARY KEY (from_item_id, to_item_id, relation_type),
		CHECK (from_item_id <> to_item_id)
	)`),
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

// tableIsEmpty reports whether table currently holds zero rows.
func tableIsEmpty(db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// addKnowledgeItemsNormalizedConceptColumn adds
// knowledge_items.normalized_concept if it does not already exist, then
// backfills every row whose value is still NULL — every pre-existing item,
// or one inserted through a path that predates this column — computing it
// from Concept via domainknowledge.NormalizeConcept, the exact function
// FindByNormalizedConcept's application-layer caller uses to normalize a
// candidate before comparing. This repair runs unconditionally on every
// Open, mirroring addSessionsFolderIDColumn, rather than gating it behind a
// one-time "table was empty" check: normalized_concept must always match
// Concept for exact-match duplicate detection to be trustworthy, and a plain
// SQL UPDATE cannot compute NormalizeConcept's Unicode-aware result itself.
// See specs/phases/phase-02-knowledge-engine/10-01-duplicate-detection-decisions.md
// Decision 1.
func addKnowledgeItemsNormalizedConceptColumn(db *sql.DB) error {
	hasNormalizedConcept, err := hasColumn(db, "knowledge_items", "normalized_concept")
	if err != nil {
		return err
	}
	if !hasNormalizedConcept {
		if _, err := db.Exec(`ALTER TABLE knowledge_items ADD COLUMN normalized_concept TEXT`); err != nil {
			return err
		}
	}
	return backfillKnowledgeItemsNormalizedConcept(db)
}

// knowledgeItemConceptRow is one row pendingNormalizedConceptBackfills reads.
type knowledgeItemConceptRow struct{ id, concept string }

// backfillKnowledgeItemsNormalizedConcept computes normalized_concept in Go
// for every row where it is still NULL, since the normalization rule cannot
// be expressed as a single SQL statement. The SELECT runs, and its Rows are
// fully closed, inside pendingNormalizedConceptBackfills before any UPDATE
// below runs — db.SetMaxOpenConns(1) means an UPDATE issued while those Rows
// were still open here would block forever waiting for the very connection
// they're holding.
func backfillKnowledgeItemsNormalizedConcept(db *sql.DB) error {
	pending, err := pendingNormalizedConceptBackfills(db)
	if err != nil {
		return err
	}
	for _, item := range pending {
		normalized := domainknowledge.NormalizeConcept(item.concept)
		if _, err := db.Exec(
			`UPDATE knowledge_items SET normalized_concept = ? WHERE id = ?`, normalized, item.id,
		); err != nil {
			return err
		}
	}
	return nil
}

func pendingNormalizedConceptBackfills(db *sql.DB) ([]knowledgeItemConceptRow, error) {
	rows, err := db.Query(`SELECT id, concept FROM knowledge_items WHERE normalized_concept IS NULL`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pending []knowledgeItemConceptRow
	for rows.Next() {
		var item knowledgeItemConceptRow
		if scanErr := rows.Scan(&item.id, &item.concept); scanErr != nil {
			return nil, scanErr
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pending, nil
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

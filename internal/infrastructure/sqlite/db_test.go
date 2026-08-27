package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_createsAccountsTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the accounts table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'accounts'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "accounts", tableName)
}

func TestOpen_createsUsageTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the usage table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'usage'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "usage", tableName)
}

func TestOpen_createsSessionsTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the sessions table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sessions'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "sessions", tableName)
}

func TestOpen_createsMessagesTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the messages table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'messages'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "messages", tableName)
}

func TestOpen_createsKnowledgeEvidenceTablesWithSharingAndOwnershipConstraints(t *testing.T) {
	// Given a freshly opened database with one Knowledge Item and one Evidence snapshot
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	_, err = db.Exec(`INSERT INTO knowledge_items
		(id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at)
		VALUES ('item-1', 'Go', 'Channels', 'Typed conduits.', '[]', '[]', '[]', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_evidence
		(id, origin_type, origin_id, source_label, excerpt, created_at)
		VALUES ('evidence-1', 'session_message', 'message-1', 'Go', 'literal quote', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES ('item-1', 'evidence-1')`)
	require.NoError(t, err)

	// When attempting duplicate Evidence and links with missing owners
	_, duplicateErr := db.Exec(`INSERT INTO knowledge_evidence
		(id, origin_type, origin_id, source_label, excerpt, created_at)
		VALUES ('evidence-duplicate', 'session_message', 'message-1', 'Go', 'literal quote', CURRENT_TIMESTAMP)`)
	_, missingItemErr := db.Exec(`INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES ('missing-item', 'evidence-1')`)
	_, missingEvidenceErr := db.Exec(`INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES ('item-1', 'missing-evidence')`)

	// Then one origin plus excerpt is globally shared and both link owners are enforced
	require.Error(t, duplicateErr)
	require.Error(t, missingItemErr)
	require.Error(t, missingEvidenceErr)

	// And deleting the Item cascades only its link, preserving the immutable snapshot
	_, err = db.Exec(`DELETE FROM knowledge_items WHERE id = 'item-1'`)
	require.NoError(t, err)
	var linkCount, evidenceCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM knowledge_item_evidence`).Scan(&linkCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM knowledge_evidence`).Scan(&evidenceCount))
	assert.Zero(t, linkCount)
	assert.Equal(t, 1, evidenceCount)
}

func TestOpen_enablesForeignKeysAndRejectsMessageForMissingSession(t *testing.T) {
	// Given a freshly opened database
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// When inspecting foreign-key enforcement and inserting an orphan message
	var foreignKeysEnabled int
	queryErr := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled)
	require.NoError(t, queryErr)
	_, insertErr := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at)
		VALUES ('message-1', 'missing-session', 'user', 'hello', CURRENT_TIMESTAMP)`)

	// Then enforcement is active and the orphan write is rejected
	assert.Equal(t, 1, foreignKeysEnabled)
	require.Error(t, insertErr)
}

func TestOpen_enforcesForeignKeysOnAFreshConnectionAfterTheFirstIsDiscarded(t *testing.T) {
	// Given a freshly opened database with idle connections disabled, so
	// database/sql cannot keep reusing the one connection Open happened to
	// use for the checks above — every query below has to open (or
	// re-open) a physical connection of its own
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	db.SetMaxIdleConns(0)

	// When querying foreign-key enforcement across several connections in
	// a row
	for i := 0; i < 5; i++ {
		var foreignKeysEnabled int
		require.NoError(t, db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled))

		// Then every one of them enforces foreign keys, not just the
		// connection Open itself used — proving enforcement comes from the
		// DSN applied to every connection this *sql.DB opens, not a
		// one-time Exec against whichever connection served it
		assert.Equal(t, 1, foreignKeysEnabled, "connection %d", i)
	}
}

func TestOpen_configuresSessionDeletionToCascadeMessagesAndDetachUsage(t *testing.T) {
	// Given a session with one message and one usage entry
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	_, err = db.Exec(`INSERT INTO sessions (id, topic, mode, folder_id, started_at)
		VALUES ('session-1', 'Go', 'socratic', 'default', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at)
		VALUES ('message-1', 'session-1', 'user', 'hello', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage (id, session_id, model, input_tokens, output_tokens, cost, created_at)
		VALUES ('usage-1', 'session-1', 'model', 10, 5, 0.01, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	// When deleting the session
	_, deleteErr := db.Exec(`DELETE FROM sessions WHERE id = 'session-1'`)

	// Then its messages are deleted and its usage remains detached
	require.NoError(t, deleteErr)
	var messageCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM messages WHERE id = 'message-1'`).Scan(&messageCount))
	assert.Zero(t, messageCount)
	var usageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'usage-1'`).Scan(&usageSessionID))
	assert.False(t, usageSessionID.Valid)
}

func TestOpen_migratesLegacyForeignKeysAndDetachesUsageWithoutRemovingIt(t *testing.T) {
	// Given a legacy database with valid rows, orphan rows, and a session
	// whose folder no longer exists
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE folders (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0, created_at DATETIME
		);
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY, topic TEXT, mode TEXT, folder_id TEXT REFERENCES folders(id), started_at DATETIME
		);
		CREATE TABLE messages (
			id TEXT PRIMARY KEY, session_id TEXT REFERENCES sessions(id), role TEXT, content TEXT, created_at DATETIME
		);
		CREATE TABLE usage (
			id TEXT PRIMARY KEY, session_id TEXT REFERENCES sessions(id), model TEXT,
			input_tokens INTEGER, output_tokens INTEGER, cost REAL, created_at DATETIME
		);
		INSERT INTO folders (id, name, is_default) VALUES ('default', 'General', 1);
		INSERT INTO sessions (id, topic, mode, folder_id) VALUES
			('valid-session', 'Go', 'socratic', 'default'),
			('invalid-session', 'Rust', 'socratic', 'missing-folder');
		INSERT INTO messages (id, session_id, role, content) VALUES
			('valid-message', 'valid-session', 'user', 'valid'),
			('invalid-session-message', 'invalid-session', 'user', 'invalid parent'),
			('orphan-message', 'missing-session', 'user', 'orphan');
		INSERT INTO usage (id, session_id, model) VALUES
			('valid-usage', 'valid-session', 'model'),
			('invalid-session-usage', 'invalid-session', 'model'),
			('orphan-usage', 'missing-session', 'model');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the session with a stale folder reference is reassigned to the
	// default folder, not deleted — its message survives with it. Only the
	// message with no owning session at all is removed. Usage is a
	// financial record, never deleted for merely losing its session: every
	// row survives, detached instead
	for _, check := range []struct {
		name  string
		query string
	}{
		{name: "sessions", query: `SELECT COUNT(*) FROM sessions`},
		{name: "messages", query: `SELECT COUNT(*) FROM messages`},
	} {
		var count int
		require.NoError(t, db.QueryRow(check.query).Scan(&count))
		assert.Equal(t, 2, count, check.name)
	}
	var invalidSessionFolderID string
	require.NoError(t, db.QueryRow(`SELECT folder_id FROM sessions WHERE id = 'invalid-session'`).Scan(&invalidSessionFolderID))
	assert.Equal(t, "default", invalidSessionFolderID, "session with a stale folder reference is reassigned to default, not deleted")
	var validMessageContent string
	require.NoError(t, db.QueryRow(`SELECT content FROM messages WHERE id = 'valid-message'`).Scan(&validMessageContent))
	assert.Equal(t, "valid", validMessageContent)
	var invalidSessionMessageContent string
	require.NoError(t, db.QueryRow(`SELECT content FROM messages WHERE id = 'invalid-session-message'`).Scan(&invalidSessionMessageContent))
	assert.Equal(t, "invalid parent", invalidSessionMessageContent, "message survives because its session was reassigned, not deleted")
	var usageCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage`).Scan(&usageCount))
	assert.Equal(t, 3, usageCount, "usage rows are never deleted")
	var invalidSessionUsageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'invalid-session-usage'`).Scan(&invalidSessionUsageSessionID))
	require.True(t, invalidSessionUsageSessionID.Valid, "usage stays attached to invalid-session, since that session was reassigned, not deleted")
	assert.Equal(t, "invalid-session", invalidSessionUsageSessionID.String)
	var orphanUsageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'orphan-usage'`).Scan(&orphanUsageSessionID))
	assert.Falsef(t, orphanUsageSessionID.Valid, "orphan-usage should be detached (NULL session_id), since missing-session never existed")
	var validUsageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'valid-usage'`).Scan(&validUsageSessionID))
	require.True(t, validUsageSessionID.Valid)
	assert.Equal(t, "valid-session", validUsageSessionID.String)

	// And the rebuilt constraints cascade messages, detach usage, and have
	// no remaining violations
	_, err = db.Exec(`DELETE FROM sessions WHERE id = 'valid-session'`)
	require.NoError(t, err)
	var messageCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount))
	assert.Equal(t, 1, messageCount, "only valid-session's message cascades away; invalid-session's message survives with its reassigned session")
	var usageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'valid-usage'`).Scan(&usageSessionID))
	assert.False(t, usageSessionID.Valid)
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	assert.False(t, rows.Next())
}

func TestOpen_refusesDatabaseWithUnexpectedForeignKeyViolation(t *testing.T) {
	// Given a database containing an orphan relationship unknown to Athena's
	// targeted legacy cleanup
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE unexpected_parent (id TEXT PRIMARY KEY);
		CREATE TABLE unexpected_child (
			id TEXT PRIMARY KEY,
			parent_id TEXT REFERENCES unexpected_parent(id)
		);
		INSERT INTO unexpected_child (id, parent_id) VALUES ('child-1', 'missing-parent');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it through Athena
	db, openErr := Open(path)
	if db != nil {
		_ = db.Close()
	}

	// Then startup is rejected rather than accepting the unknown violation
	require.Error(t, openErr)
	assert.ErrorContains(t, openErr, "foreign key check")
}

func TestOpen_createsFoldersTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the folders table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'folders'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "folders", tableName)
}

func TestOpen_seedsDefaultFolder(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then a default folder named "General" is seeded
	var name string
	var isDefault bool
	queryErr := db.QueryRow(
		`SELECT name, is_default FROM folders WHERE id = 'default'`,
	).Scan(&name, &isDefault)
	require.NoError(t, queryErr)
	assert.Equal(t, "General", name)
	assert.True(t, isDefault)
}

func TestOpen_doesNotDuplicateDefaultFolderOnSecondOpen(t *testing.T) {
	// Given a database that was already opened once
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening the same database file again
	second, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = second.Close() }()

	// Then the default folder still exists exactly once
	var count int
	queryErr := second.QueryRow(`SELECT COUNT(*) FROM folders WHERE id = 'default'`).Scan(&count)
	require.NoError(t, queryErr)
	assert.Equal(t, 1, count)
}

func TestOpen_createsKnowledgeItemsTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the knowledge_items table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_items'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "knowledge_items", tableName)
}

func TestOpen_isIdempotentOnSecondOpen_forKnowledgeItems(t *testing.T) {
	// Given a database that was already opened once
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening the same database file again
	second, err := Open(path)

	// Then it succeeds without error on the repeated CREATE TABLE/INDEX
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
}

func TestOpen_createsKnowledgeChunksTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the knowledge_chunks table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_chunks'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "knowledge_chunks", tableName)
}

func TestOpen_createsKnowledgeChunksIndexes(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then both knowledge_chunks indexes exist
	for _, indexName := range []string{"idx_knowledge_chunks_file_path", "idx_knowledge_chunks_item_id"} {
		var name string
		queryErr := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName,
		).Scan(&name)
		require.NoError(t, queryErr)
		assert.Equal(t, indexName, name)
	}
}

func TestOpen_isIdempotentOnSecondOpen_forKnowledgeChunks(t *testing.T) {
	// Given a database that was already opened once
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening the same database file again
	second, err := Open(path)

	// Then it succeeds without error on the repeated CREATE TABLE/INDEX
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
}

func TestOpen_createsIngestedFilesTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the ingested_files table exists
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'ingested_files'`,
	).Scan(&tableName)
	require.NoError(t, queryErr)
	assert.Equal(t, "ingested_files", tableName)
}

func TestOpen_createsKnowledgeChunksSourcePathIndex(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the source_path index exists
	var name string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_knowledge_chunks_source_path'`,
	).Scan(&name)
	require.NoError(t, queryErr)
	assert.Equal(t, "idx_knowledge_chunks_source_path", name)
}

func TestOpen_addsSourcePathColumn_toAnOlderKnowledgeChunksTable_whenEmpty(t *testing.T) {
	// Given a database whose knowledge_chunks table predates source_path
	// (dropped down to the old column set, still empty)
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, execErr := db.Exec(`DROP TABLE knowledge_chunks`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(`CREATE TABLE knowledge_chunks (
		id TEXT PRIMARY KEY, source TEXT, topic TEXT, status TEXT, item_id TEXT,
		file_path TEXT, heading TEXT, content TEXT, embedding BLOB,
		embedding_model TEXT NOT NULL, item_updated_at DATETIME, created_at DATETIME
	)`)
	require.NoError(t, execErr)
	require.NoError(t, db.Close())

	// When reopening the database (re-running migrations)
	second, err := Open(path)

	// Then it succeeds and the column now exists
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
	hasIt, colErr := hasColumn(second, "knowledge_chunks", "source_path")
	require.NoError(t, colErr)
	assert.True(t, hasIt)
}

func TestOpen_refusesToAlterKnowledgeChunks_whenOlderTableIsNotEmpty(t *testing.T) {
	// Given a database whose old-schema knowledge_chunks table already has a row
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, execErr := db.Exec(`DROP TABLE knowledge_chunks`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(`CREATE TABLE knowledge_chunks (
		id TEXT PRIMARY KEY, source TEXT, topic TEXT, status TEXT, item_id TEXT,
		file_path TEXT, heading TEXT, content TEXT, embedding BLOB,
		embedding_model TEXT NOT NULL, item_updated_at DATETIME, created_at DATETIME
	)`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(
		`INSERT INTO knowledge_chunks (id, source, embedding_model) VALUES ('c1', 'imported_doc', 'model')`,
	)
	require.NoError(t, execErr)
	require.NoError(t, db.Close())

	// When reopening the database
	_, err = Open(path)

	// Then it fails loudly instead of silently dropping/altering the data
	require.Error(t, err)
}

func TestOpen_migratesIngestedFilesToSourcePathSchema_whenEmpty(t *testing.T) {
	// Given a database whose ingested_files table still uses the pre-release
	// file_path-keyed, second-precision schema, and is empty
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, execErr := db.Exec(`DROP TABLE ingested_files`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(`CREATE TABLE ingested_files (
		file_path TEXT PRIMARY KEY, mtime INTEGER NOT NULL, embedding_model TEXT NOT NULL,
		chunk_count INTEGER NOT NULL, item_id TEXT NOT NULL, ingested_at DATETIME
	)`)
	require.NoError(t, execErr)
	require.NoError(t, db.Close())

	// When reopening the database (re-running migrations)
	second, err := Open(path)

	// Then it succeeds and the table now has the new schema
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
	hasIt, colErr := hasColumn(second, "ingested_files", "source_path")
	require.NoError(t, colErr)
	assert.True(t, hasIt)
	_, execErr = second.Exec(
		`INSERT INTO ingested_files (source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id)
		 VALUES ('/abs/go.md', 'go.md', 123, 'model', 1, 'item-1')`,
	)
	assert.NoError(t, execErr)
}

func TestOpen_refusesToDropIngestedFiles_whenOlderTableIsNotEmpty(t *testing.T) {
	// Given a database whose old-schema ingested_files table already has a row
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, execErr := db.Exec(`DROP TABLE ingested_files`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(`CREATE TABLE ingested_files (
		file_path TEXT PRIMARY KEY, mtime INTEGER NOT NULL, embedding_model TEXT NOT NULL,
		chunk_count INTEGER NOT NULL, item_id TEXT NOT NULL, ingested_at DATETIME
	)`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(
		`INSERT INTO ingested_files (file_path, mtime, embedding_model, chunk_count, item_id)
		 VALUES ('go.md', 123, 'model', 1, 'item-1')`,
	)
	require.NoError(t, execErr)
	require.NoError(t, db.Close())

	// When reopening the database
	_, err = Open(path)

	// Then it fails loudly instead of silently dropping the data
	require.Error(t, err)
}

func TestOpen_migrationIsIdempotentOnNextOpen(t *testing.T) {
	// Given a database that actually went through the pre-release-schema
	// migration (not one created fresh with the target schema already in
	// place), by starting from the old ingested_files shape
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	_, execErr := first.Exec(`DROP TABLE ingested_files`)
	require.NoError(t, execErr)
	_, execErr = first.Exec(`CREATE TABLE ingested_files (
		file_path TEXT PRIMARY KEY, mtime INTEGER NOT NULL, embedding_model TEXT NOT NULL,
		chunk_count INTEGER NOT NULL, item_id TEXT NOT NULL, ingested_at DATETIME
	)`)
	require.NoError(t, execErr)
	require.NoError(t, first.Close())

	// When reopening it (running the migration for real) and then
	// reopening a third time against the now-already-migrated schema
	second, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, second.Close())
	third, err := Open(path)

	// Then the third open still succeeds, proving the migration step is a
	// no-op once source_path already exists
	require.NoError(t, err)
	defer func() { _ = third.Close() }()
	hasIt, colErr := hasColumn(third, "ingested_files", "source_path")
	require.NoError(t, colErr)
	assert.True(t, hasIt)
}

func TestOpen_addsFolderIDColumnToSessions(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the sessions table has a folder_id column
	rows, queryErr := db.Query(`PRAGMA table_info(sessions)`)
	require.NoError(t, queryErr)
	defer func() { _ = rows.Close() }()

	hasFolderID := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dfltValue sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk))
		if name == "folder_id" {
			hasFolderID = true
		}
	}
	require.NoError(t, rows.Err())
	assert.True(t, hasFolderID)
}

func TestOpen_backfillsExistingSessionsToDefaultFolder(t *testing.T) {
	// Given a session row inserted with no folder_id, as if it predated
	// this migration
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, execErr := db.Exec(
		`INSERT INTO sessions (id, topic, mode, started_at) VALUES (?, ?, ?, ?)`,
		"session-1", "Topic", "study", "2024-01-01",
	)
	require.NoError(t, execErr)
	require.NoError(t, db.Close())

	// When reopening the database (re-running migrations)
	second, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = second.Close() }()

	// Then the pre-existing session is backfilled to the default folder
	var folderID string
	queryErr := second.QueryRow(`SELECT folder_id FROM sessions WHERE id = ?`, "session-1").Scan(&folderID)
	require.NoError(t, queryErr)
	assert.Equal(t, "default", folderID)
}

func TestOpen_isNoOpOnSecondOpenAndKeepsExistingData(t *testing.T) {
	// Given a database that was already opened once and has a row in it
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	_, execErr := first.Exec(
		`INSERT INTO accounts (id, email, password_hash) VALUES (?, ?, ?)`,
		"acc-1", "user@example.com", "hash",
	)
	require.NoError(t, execErr)
	require.NoError(t, first.Close())

	// When opening the same database file again
	second, err := Open(path)

	// Then it succeeds without re-running migrations destructively and the
	// existing row is still there
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
	var email string
	queryErr := second.QueryRow(`SELECT email FROM accounts WHERE id = ?`, "acc-1").Scan(&email)
	require.NoError(t, queryErr)
	assert.Equal(t, "user@example.com", email)
}

func TestOpen_serializesConcurrentWrites_withoutDatabaseLockedErrors(t *testing.T) {
	// Given an open database and many goroutines about to write to it at
	// once — database/sql pools connections by default, and modernc.org/
	// sqlite has no busy_timeout unless configured, so competing writers on
	// separate pooled connections previously failed immediately with
	// "database is locked (5) (SQLITE_BUSY)" instead of queuing.
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	const workers = 20
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, execErr := db.Exec(
				`INSERT INTO accounts (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("acc-%d", i), fmt.Sprintf("user%d@example.com", i), "hash", time.Now().UTC(),
			)
			errs <- execErr
		}(i)
	}

	// When they all write concurrently
	wg.Wait()
	close(errs)

	// Then every write succeeds — none see a locked database
	for execErr := range errs {
		assert.NoError(t, execErr)
	}
}

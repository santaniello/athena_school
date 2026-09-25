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

func TestOpen_doesNotCreateAccountsTable(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then no accounts table is created — local login was removed, see
	// specs/phases/phase-01-desktop-mvp/12-remove-local-login.md
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'accounts'`,
	).Scan(&tableName)
	assert.ErrorIs(t, queryErr, sql.ErrNoRows)
}

func TestOpen_dropsAccountsTable_fromAPriorInstall(t *testing.T) {
	// Given a legacy database that still has the accounts table from before
	// local login was removed
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE accounts (
			id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL, created_at DATETIME
		);
		INSERT INTO accounts (id, email, password_hash) VALUES ('acc-1', 'user@example.com', 'hash');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the accounts table, and the row in it, are gone
	var tableName string
	queryErr := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'accounts'`,
	).Scan(&tableName)
	assert.ErrorIs(t, queryErr, sql.ErrNoRows)
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

func TestOpen_appliesForeignKeysPragma_whenPathAlreadyHasQueryParameters(t *testing.T) {
	// Given a path that already carries its own URI query parameters
	path := "file:" + filepath.Join(t.TempDir(), "athena.db") + "?mode=rwc"

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then foreign-key enforcement is still applied — the existing query
	// string is extended with '&', not overwritten by a second '?'
	var foreignKeysEnabled int
	require.NoError(t, db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled))
	assert.Equal(t, 1, foreignKeysEnabled)
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
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, topic, mode, folder_id, started_at)
		VALUES ('session-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP)`)
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

	// Then the session with a stale folder reference is deleted — there is
	// no fallback folder left to reassign it to — along with its message.
	// Only the message with no owning session at all is separately removed.
	// Usage is a financial record, never deleted for merely losing its
	// session: every row survives, detached instead
	var sessionCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount))
	assert.Equal(t, 1, sessionCount, "only valid-session survives; invalid-session had a stale folder reference")
	var messageCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount))
	assert.Equal(t, 1, messageCount, "only valid-message survives; invalid-session-message and orphan-message are both gone")
	var validMessageContent string
	require.NoError(t, db.QueryRow(`SELECT content FROM messages WHERE id = 'valid-message'`).Scan(&validMessageContent))
	assert.Equal(t, "valid", validMessageContent)
	var usageCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage`).Scan(&usageCount))
	assert.Equal(t, 3, usageCount, "usage rows are never deleted")
	var invalidSessionUsageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'invalid-session-usage'`).Scan(&invalidSessionUsageSessionID))
	assert.Falsef(t, invalidSessionUsageSessionID.Valid, "usage is detached, since invalid-session was deleted, not reassigned")
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
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount))
	assert.Equal(t, 0, messageCount, "valid-session's message cascades away; no sessions remain")
	var usageSessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'valid-usage'`).Scan(&usageSessionID))
	assert.False(t, usageSessionID.Valid)
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	assert.False(t, rows.Next())
}

func TestOpen_deletesSessionsWithInvalidFolders_evenWhenMessagesAndUsageAreAlreadyMigrated(t *testing.T) {
	// Given a database whose messages/usage tables already declare the
	// current foreign-key actions — as any schema created fresh, or
	// already migrated in an earlier Open, would — but whose sessions
	// carry NULL, empty, and dangling folder_id values, as if only the
	// sessions table had been restored from an older backup into an
	// otherwise up-to-date database
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
			id         TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role       TEXT, content TEXT, created_at DATETIME
		);
		CREATE TABLE usage (
			id            TEXT PRIMARY KEY,
			session_id    TEXT REFERENCES sessions(id) ON DELETE SET NULL,
			model TEXT, input_tokens INTEGER, output_tokens INTEGER, cost REAL, created_at DATETIME
		);
		INSERT INTO folders (id, name, is_default) VALUES ('default', 'General', 1);
		INSERT INTO sessions (id, topic, mode, folder_id) VALUES
			('null-folder-session', 'Go', 'socratic', NULL),
			('empty-folder-session', 'Rust', 'socratic', ''),
			('missing-folder-session', 'Python', 'socratic', 'missing-folder');
		INSERT INTO messages (id, session_id, role, content) VALUES
			('null-folder-message', 'null-folder-session', 'user', 'a'),
			('empty-folder-message', 'empty-folder-session', 'user', 'b'),
			('missing-folder-message', 'missing-folder-session', 'user', 'c');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then every session with an invalid folder is deleted — the repair is
	// not skipped just because messages/usage already had the current
	// foreign-key actions — and every message goes with it
	var sessionCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount))
	assert.Equal(t, 0, sessionCount)
	var messageCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount))
	assert.Equal(t, 0, messageCount)
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

func TestOpen_startsWithNoFoldersSeeded(t *testing.T) {
	// Given a path to a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening the database
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then no folder is auto-created — the user must create their own
	var count int
	queryErr := db.QueryRow(`SELECT COUNT(*) FROM folders`).Scan(&count)
	require.NoError(t, queryErr)
	assert.Zero(t, count)
}

func TestOpen_dropsFoldersIsDefaultColumn(t *testing.T) {
	// Given a legacy database whose folders table still has is_default,
	// with a pre-existing row in it
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE folders (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0, created_at DATETIME
		);
		INSERT INTO folders (id, name, is_default, created_at) VALUES ('default', 'General', 1, CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then is_default is gone, but the row survives as an ordinary folder
	has, err := hasColumn(db, "folders", "is_default")
	require.NoError(t, err)
	assert.False(t, has)
	var name string
	queryErr := db.QueryRow(`SELECT name FROM folders WHERE id = 'default'`).Scan(&name)
	require.NoError(t, queryErr)
	assert.Equal(t, "General", name)
}

func TestOpen_configuresFolderDeletionToCascadeSessions(t *testing.T) {
	// Given a folder with a session in it
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, topic, mode, folder_id, started_at)
		VALUES ('session-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	// When deleting the folder
	_, deleteErr := db.Exec(`DELETE FROM folders WHERE id = 'folder-1'`)

	// Then its session is deleted along with it
	require.NoError(t, deleteErr)
	var sessionCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = 'session-1'`).Scan(&sessionCount))
	assert.Zero(t, sessionCount)
}

func TestOpen_rejectsSessionsWithNoFolder(t *testing.T) {
	// Given a freshly migrated database
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// When inserting a session with no folder_id at all
	_, execErr := db.Exec(
		`INSERT INTO sessions (id, topic, mode, started_at) VALUES (?, ?, ?, ?)`,
		"session-1", "Topic", "study", "2024-01-01",
	)

	// Then it is rejected at the schema level, not just by application code
	require.Error(t, execErr)
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

func TestOpen_deletesExistingSessionsWithNoFolder(t *testing.T) {
	// Given a legacy database predating the folder_id column entirely, with
	// a session row already in it — sessions.folder_id is NOT NULL once
	// migrated, so this can only be simulated on a database that never went
	// through Open() yet, not by inserting through an already-migrated one
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE sessions (id TEXT PRIMARY KEY, topic TEXT, mode TEXT, started_at DATETIME);
		INSERT INTO sessions (id, topic, mode, started_at) VALUES ('session-1', 'Topic', 'study', '2024-01-01');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it through the current migration path — there is no
	// fallback folder to backfill it to
	second, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = second.Close() }()

	// Then the pre-existing session is deleted
	var count int
	queryErr := second.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, "session-1").Scan(&count)
	require.NoError(t, queryErr)
	assert.Zero(t, count)
}

func TestOpen_backfillsExistingSessionsWithEmptyGoal(t *testing.T) {
	// Given a session row inserted with a valid folder but no goal, as if
	// it predated this migration
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, execErr := db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP)`)
	require.NoError(t, execErr)
	_, execErr = db.Exec(
		`INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES (?, ?, ?, ?, ?)`,
		"session-1", "Topic", "study", "folder-1", "2024-01-01",
	)
	require.NoError(t, execErr)
	require.NoError(t, db.Close())

	// When reopening the database (re-running migrations)
	second, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = second.Close() }()

	// Then the pre-existing session is backfilled to an empty goal, not
	// left NULL or some other placeholder
	var goal string
	queryErr := second.QueryRow(`SELECT goal FROM sessions WHERE id = ?`, "session-1").Scan(&goal)
	require.NoError(t, queryErr)
	assert.Empty(t, goal)
}

func TestOpen_isNoOpOnSecondOpenAndKeepsExistingData(t *testing.T) {
	// Given a database that was already opened once and has a row in it
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	_, execErr := first.Exec(
		`INSERT INTO folders (id, name) VALUES (?, ?)`,
		"folder-1", "Custom",
	)
	require.NoError(t, execErr)
	require.NoError(t, first.Close())

	// When opening the same database file again
	second, err := Open(path)

	// Then it succeeds without re-running migrations destructively and the
	// existing row is still there
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
	var name string
	queryErr := second.QueryRow(`SELECT name FROM folders WHERE id = ?`, "folder-1").Scan(&name)
	require.NoError(t, queryErr)
	assert.Equal(t, "Custom", name)
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
				`INSERT INTO folders (id, name, created_at) VALUES (?, ?, ?)`,
				fmt.Sprintf("folder-%d", i), fmt.Sprintf("Folder %d", i), time.Now().UTC(),
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

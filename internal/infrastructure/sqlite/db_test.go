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
	// Given a database already migrated to the source_path schema
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening it again
	second, err := Open(path)

	// Then it succeeds without error on the repeated migration steps
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
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

package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

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

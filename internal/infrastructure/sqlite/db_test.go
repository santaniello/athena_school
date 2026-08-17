package sqlite

import (
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

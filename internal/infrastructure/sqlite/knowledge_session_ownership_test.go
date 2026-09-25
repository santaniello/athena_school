package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSessionID is the study session every repository test's knowledge
// rows belong to; seedSession creates it (and its folder) first because
// knowledge rows reference sessions(id).
const testSessionID = "session-1"

// seedSession inserts a folder and a study session with id, so knowledge
// rows owned by it satisfy their foreign key. Safe to call twice for the
// same id.
func seedSession(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT OR IGNORE INTO folders (id, name, created_at) VALUES ('seed-folder', 'Seed', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT OR IGNORE INTO sessions (id, topic, mode, folder_id, started_at)
		VALUES (?, 'Topic', 'study', 'seed-folder', CURRENT_TIMESTAMP)`, id)
	require.NoError(t, err)
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow(query, args...).Scan(&count))
	return count
}

// insertKnowledgeRecordsFor writes one row into every table that hangs off a
// session, all owned by sessionID, keyed by suffix so two sessions never
// collide.
func insertKnowledgeRecordsFor(t *testing.T, db *sql.DB, sessionID, suffix string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO knowledge_items
		(id, session_id, topic, concept, definition, properties, trade_offs, related_concepts, source, created_at, updated_at)
		VALUES (?, ?, 'Go', 'go.md', 'A document.', '[]', '[]', '[]', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		       (?, ?, 'Go', 'sync.md', 'Another document.', '[]', '[]', '[]', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"item-"+suffix, sessionID, "other-item-"+suffix, sessionID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_chunks
		(id, session_id, source, topic, item_id, source_path, file_path, heading, content, embedding, embedding_model, created_at)
		VALUES (?, ?, 'imported_doc', 'Go', ?, '/abs/go.md', 'go.md', 'Intro', 'text', x'00', 'model', CURRENT_TIMESTAMP)`,
		"chunk-"+suffix, sessionID, "item-"+suffix)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO ingested_files
		(session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
		VALUES (?, '/abs/go.md', 'go.md', 1, 'model', 1, ?, CURRENT_TIMESTAMP)`, sessionID, "item-"+suffix)
	require.NoError(t, err)
}

func TestOpen_deletingASession_cascadesEveryKnowledgeRecordItOwns_andOnlyThose(t *testing.T) {
	// Given two sessions that each own a full set of knowledge records
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	insertKnowledgeRecordsFor(t, db, "session-a", "a")
	insertKnowledgeRecordsFor(t, db, "session-b", "b")

	// When session A is deleted
	_, err = db.Exec(`DELETE FROM sessions WHERE id = 'session-a'`)
	require.NoError(t, err)

	// Then A's items, chunks and ingested files are gone while B keeps every
	// one of its own
	for _, table := range []string{"knowledge_chunks", "ingested_files"} {
		assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE session_id = 'session-a'`), table)
		assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE session_id = 'session-b'`), table)
	}
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items WHERE session_id = 'session-a'`))
	assert.Equal(t, 2, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items WHERE session_id = 'session-b'`))
}

func TestOpen_deletingAFolder_cascadesKnowledgeOfItsSessions(t *testing.T) {
	// Given a session inside a folder that owns knowledge
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	seedSession(t, db, "session-a")
	insertKnowledgeRecordsFor(t, db, "session-a", "a")

	// When its folder is deleted
	_, err = db.Exec(`DELETE FROM folders WHERE id = 'seed-folder'`)
	require.NoError(t, err)

	// Then the knowledge goes with it
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_chunks`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM ingested_files`))
}

func TestOpen_rejectsKnowledgeRecordsWhoseSessionDoesNotExist(t *testing.T) {
	// Given a database with no sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// When inserting knowledge owned by a missing session
	_, itemErr := db.Exec(`INSERT INTO knowledge_items
		(id, session_id, topic, concept, definition, source, created_at, updated_at)
		VALUES ('item-1', 'missing', 'Go', 'C', 'D', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	_, blankErr := db.Exec(`INSERT INTO knowledge_items
		(id, session_id, topic, concept, definition, source, created_at, updated_at)
		VALUES ('item-2', NULL, 'Go', 'C', 'D', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)

	// Then both are refused
	require.Error(t, itemErr)
	require.Error(t, blankErr)
}

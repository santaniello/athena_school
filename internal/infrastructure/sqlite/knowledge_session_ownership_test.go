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
		(id, session_id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at)
		VALUES (?, ?, 'Go', 'Channels', 'Typed conduits.', '[]', '[]', '[]', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		       (?, ?, 'Go', 'Goroutines', 'Light threads.', '[]', '[]', '[]', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"item-"+suffix, sessionID, "other-item-"+suffix, sessionID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_chunks
		(id, session_id, source, topic, status, item_id, source_path, file_path, heading, content, embedding, embedding_model, created_at)
		VALUES (?, ?, 'imported_doc', 'Go', 'approved', ?, '/abs/go.md', 'go.md', 'Intro', 'text', x'00', 'model', CURRENT_TIMESTAMP)`,
		"chunk-"+suffix, sessionID, "item-"+suffix)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO ingested_files
		(session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
		VALUES (?, '/abs/go.md', 'go.md', 1, 'model', 1, ?, CURRENT_TIMESTAMP)`, sessionID, "item-"+suffix)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_reconciliation_proposals
		(id, session_id, action, status, candidate_snapshot, reason, changes, created_at)
		VALUES (?, ?, 'create', 'pending', '{}', 'why', '{}', CURRENT_TIMESTAMP)`, "proposal-"+suffix, sessionID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_evidence (id, origin_type, origin_id, source_label, excerpt, created_at)
		VALUES (?, 'session_message', 'message-1', 'Go', ?, CURRENT_TIMESTAMP)`, "evidence-"+suffix, "quote "+suffix)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES (?, ?)`,
		"item-"+suffix, "evidence-"+suffix)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_item_relations (from_item_id, to_item_id, relation_type, created_at)
		VALUES (?, ?, 'related', CURRENT_TIMESTAMP)`, "item-"+suffix, "other-item-"+suffix)
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

	// Then A's items, chunks, ingested files, proposals, evidence links and
	// relations are gone while B keeps every one of its own
	for _, table := range []string{
		"knowledge_chunks", "ingested_files", "knowledge_reconciliation_proposals",
	} {
		assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE session_id = 'session-a'`), table)
		assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE session_id = 'session-b'`), table)
	}
	assert.Equal(t, 0, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items WHERE session_id = 'session-a'`))
	assert.Equal(t, 2, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items WHERE session_id = 'session-b'`))
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM knowledge_item_evidence`))
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM knowledge_item_relations`))
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
		(id, session_id, topic, concept, definition, source, status, created_at, updated_at)
		VALUES ('item-1', 'missing', 'Go', 'C', 'D', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	_, blankErr := db.Exec(`INSERT INTO knowledge_items
		(id, session_id, topic, concept, definition, source, status, created_at, updated_at)
		VALUES ('item-2', NULL, 'Go', 'C', 'D', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)

	// Then both are refused
	require.Error(t, itemErr)
	require.Error(t, blankErr)
}

func TestOpen_rebuildsKnowledgeTablesWithoutSessionOwnership_discardingTheirRows(t *testing.T) {
	// Given a database from before session ownership, holding knowledge rows
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE knowledge_items (
			id TEXT PRIMARY KEY, topic TEXT, concept TEXT, definition TEXT, properties TEXT, trade_offs TEXT,
			related_concepts TEXT, source TEXT, status TEXT DEFAULT 'draft', created_at DATETIME, updated_at DATETIME
		);
		CREATE TABLE knowledge_chunks (
			id TEXT PRIMARY KEY, source TEXT, topic TEXT, status TEXT, item_id TEXT, source_path TEXT, file_path TEXT,
			heading TEXT, content TEXT, embedding BLOB, embedding_model TEXT NOT NULL, item_updated_at DATETIME,
			created_at DATETIME
		);
		CREATE TABLE knowledge_evidence (
			id TEXT PRIMARY KEY, origin_type TEXT NOT NULL, origin_id TEXT NOT NULL, source_label TEXT NOT NULL,
			excerpt TEXT NOT NULL, created_at DATETIME NOT NULL, UNIQUE (origin_type, origin_id, excerpt)
		);
		INSERT INTO knowledge_items (id, topic, concept, definition, source, status)
			VALUES ('legacy-item', 'Go', 'C', 'D', 'athena', 'approved');
		INSERT INTO knowledge_chunks (id, item_id, embedding_model) VALUES ('legacy-chunk', 'legacy-item', 'model');
		INSERT INTO knowledge_evidence (id, origin_type, origin_id, source_label, excerpt, created_at)
			VALUES ('legacy-evidence', 'session_message', 'm', 'Go', 'q', CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it
	db, err := Open(path)
	require.NoError(t, err)

	// Then the ownerless rows are gone and the tables now require a session
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_chunks`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_evidence`))
	seedSession(t, db, "session-a")
	insertKnowledgeRecordsFor(t, db, "session-a", "a")
	require.NoError(t, db.Close())

	// And reopening never rebuilds them again, so new rows survive
	db, err = Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	assert.Equal(t, 2, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items`))
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM knowledge_chunks`))
}

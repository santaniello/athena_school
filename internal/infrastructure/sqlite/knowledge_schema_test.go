package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// removedKnowledgeTables are the tables only conversation extraction used;
// see specs/phases/phase-02-knowledge-engine/16-remove-conversation-extraction.md.
var removedKnowledgeTables = []string{
	"knowledge_evidence",
	"knowledge_item_evidence",
	"knowledge_item_relations",
	"knowledge_reconciliation_proposals",
	"knowledge_reconciliation_evidence",
}

func columnNames(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	return countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table) == 1
}

func indexNames(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL ORDER BY name`, table)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func TestOpen_createsTheDocumentsOnlyKnowledgeSchema(t *testing.T) {
	// Given a database file that does not exist yet
	path := filepath.Join(t.TempDir(), "athena.db")

	// When opening it
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the knowledge tables carry no status, normalized concept or
	// item-staleness column
	assert.Equal(t, []string{
		"id", "session_id", "topic", "concept", "definition", "properties", "trade_offs",
		"related_concepts", "source", "created_at", "updated_at",
	}, columnNames(t, db, "knowledge_items"))
	assert.Equal(t, []string{
		"id", "session_id", "source", "topic", "item_id", "source_path", "file_path", "heading",
		"content", "embedding", "embedding_model", "created_at",
	}, columnNames(t, db, "knowledge_chunks"))
	assert.Equal(t, []string{
		"session_id", "source_path", "file_path", "mtime_unix_nano", "embedding_model",
		"chunk_count", "item_id", "ingested_at",
	}, columnNames(t, db, "ingested_files"))
	// And keeps only the indexes something still queries by
	assert.Equal(t, []string{"idx_knowledge_items_session_id"}, indexNames(t, db, "knowledge_items"))
	assert.Equal(t, []string{
		"idx_knowledge_chunks_file_path", "idx_knowledge_chunks_item_id",
		"idx_knowledge_chunks_session_id", "idx_knowledge_chunks_source_path",
	}, indexNames(t, db, "knowledge_chunks"))
	// And none of the extraction-only tables exist
	for _, table := range removedKnowledgeTables {
		assert.False(t, tableExists(t, db, table), table)
	}
}

// seedExtractionEraKnowledge rewrites db's knowledge tables into the shape
// they had before conversation extraction was removed, and fills every one
// of them (plus a conversation) with a row.
func seedExtractionEraKnowledge(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`DROP TABLE knowledge_chunks`,
		`DROP TABLE knowledge_items`,
		`DROP TABLE ingested_files`,
		`CREATE TABLE knowledge_items (
			id                 TEXT PRIMARY KEY,
			session_id         TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			topic              TEXT,
			concept            TEXT,
			definition         TEXT,
			properties         TEXT,
			trade_offs         TEXT,
			related_concepts   TEXT,
			source             TEXT,
			status             TEXT DEFAULT 'draft',
			created_at         DATETIME,
			updated_at         DATETIME,
			normalized_concept TEXT
		)`,
		`CREATE INDEX idx_knowledge_items_session_id ON knowledge_items(session_id)`,
		`CREATE INDEX idx_knowledge_items_status_created_at ON knowledge_items(status, created_at)`,
		`CREATE INDEX idx_knowledge_items_topic ON knowledge_items(topic)`,
		`CREATE INDEX idx_knowledge_items_topic_normalized_concept ON knowledge_items(topic, normalized_concept)`,
		`CREATE TABLE knowledge_chunks (
			id              TEXT PRIMARY KEY,
			session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			source          TEXT,
			topic           TEXT,
			status          TEXT,
			item_id         TEXT,
			source_path     TEXT,
			file_path       TEXT,
			heading         TEXT,
			content         TEXT,
			embedding       BLOB,
			embedding_model TEXT NOT NULL,
			item_updated_at DATETIME,
			created_at      DATETIME
		)`,
		`CREATE INDEX idx_knowledge_chunks_session_id ON knowledge_chunks(session_id)`,
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
		`CREATE TABLE knowledge_evidence (
			id TEXT PRIMARY KEY, origin_type TEXT NOT NULL, origin_id TEXT NOT NULL,
			source_label TEXT NOT NULL, excerpt TEXT NOT NULL, created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE knowledge_item_evidence (
			item_id     TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
			evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
			PRIMARY KEY (item_id, evidence_id)
		)`,
		`CREATE TABLE knowledge_item_relations (
			from_item_id  TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
			to_item_id    TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
			relation_type TEXT NOT NULL,
			created_at    DATETIME NOT NULL,
			PRIMARY KEY (from_item_id, to_item_id, relation_type)
		)`,
		`CREATE TABLE knowledge_reconciliation_proposals (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			action TEXT NOT NULL, status TEXT NOT NULL, candidate_snapshot TEXT NOT NULL,
			target_item_id TEXT, target_updated_at DATETIME, reason TEXT NOT NULL,
			changes TEXT NOT NULL, created_at DATETIME NOT NULL, resolved_at DATETIME
		)`,
		`CREATE TABLE knowledge_reconciliation_evidence (
			proposal_id TEXT NOT NULL REFERENCES knowledge_reconciliation_proposals(id) ON DELETE CASCADE,
			evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
			PRIMARY KEY (proposal_id, evidence_id)
		)`,
	}
	for _, statement := range statements {
		_, err := db.Exec(statement)
		require.NoError(t, err, statement)
	}

	seedSession(t, db, "session-1")
	inserts := []string{
		`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-1', 'session-1', 'assistant', 'hello', CURRENT_TIMESTAMP)`,
		`INSERT INTO message_sources (message_id, position, chunk_id, item_id, source_type, score, excerpt)
			VALUES ('message-1', 0, 'chunk-1', 'item-1', 'athena', 0.9, 'quote')`,
		`INSERT INTO knowledge_items (id, session_id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at, normalized_concept)
			VALUES ('item-1', 'session-1', 'Go', 'Channels', 'Typed conduits.', '[]', '[]', '[]', 'athena', 'approved', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'channels')`,
		`INSERT INTO knowledge_chunks (id, session_id, source, topic, status, item_id, source_path, file_path, heading, content, embedding, embedding_model, created_at)
			VALUES ('chunk-1', 'session-1', 'imported_doc', 'Go', 'approved', 'item-1', '/abs/go.md', 'go.md', 'Intro', 'text', x'00', 'model', CURRENT_TIMESTAMP)`,
		`INSERT INTO ingested_files (session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
			VALUES ('session-1', '/abs/go.md', 'go.md', 1, 'model', 1, 'item-1', CURRENT_TIMESTAMP)`,
		`INSERT INTO knowledge_evidence (id, origin_type, origin_id, source_label, excerpt, created_at)
			VALUES ('evidence-1', 'session_message', 'message-1', 'Chat', 'quote', CURRENT_TIMESTAMP)`,
		`INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES ('item-1', 'evidence-1')`,
		`INSERT INTO knowledge_reconciliation_proposals (id, session_id, action, status, candidate_snapshot, reason, changes, created_at)
			VALUES ('proposal-1', 'session-1', 'create', 'pending', '{}', 'why', '{}', CURRENT_TIMESTAMP)`,
	}
	for _, statement := range inserts {
		_, err := db.Exec(statement)
		require.NoError(t, err, statement)
	}
}

func TestOpen_migratesAnExtractionEraDatabase_discardingKnowledgeButKeepingConversations(t *testing.T) {
	// Given a database from before extraction was removed, holding knowledge
	// of every kind plus a conversation
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	seedExtractionEraKnowledge(t, first)
	require.NoError(t, first.Close())

	// When opening it
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the knowledge tables are rebuilt empty, in their final shape
	assert.NotContains(t, columnNames(t, db, "knowledge_items"), "status")
	assert.NotContains(t, columnNames(t, db, "knowledge_items"), "normalized_concept")
	assert.NotContains(t, columnNames(t, db, "knowledge_chunks"), "status")
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_items`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM knowledge_chunks`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM ingested_files`))
	// And the extraction-only tables are gone
	for _, table := range removedKnowledgeTables {
		assert.False(t, tableExists(t, db, table), table)
	}
	// And the conversation, including what a reply cited, is untouched
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM sessions WHERE id = 'session-1'`))
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM messages WHERE id = 'message-1'`))
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM message_sources WHERE message_id = 'message-1'`))
	// And no foreign key is left dangling
	violations, err := db.Query(`PRAGMA foreign_key_check`)
	require.NoError(t, err)
	defer func() { _ = violations.Close() }()
	assert.False(t, violations.Next(), "foreign_key_check must report nothing")
}

func TestOpen_runsTheKnowledgeMigrationOnce_keepingDocumentsAndNeverRecreatingRemovedTables(t *testing.T) {
	// Given a database that already went through the migration and then
	// received an imported document
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	seedExtractionEraKnowledge(t, first)
	require.NoError(t, first.Close())
	second, err := Open(path)
	require.NoError(t, err)
	seedSession(t, second, "session-2")
	_, err = second.Exec(`INSERT INTO knowledge_items (id, session_id, topic, concept, definition, properties, trade_offs, related_concepts, source, created_at, updated_at)
		VALUES ('item-2', 'session-2', 'Go', 'go.md', 'A doc.', '[]', '[]', '[]', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	require.NoError(t, second.Close())

	// When opening it again
	third, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = third.Close() }()

	// Then the document survives and nothing extraction-related came back
	assert.Equal(t, 1, countRows(t, third, `SELECT COUNT(*) FROM knowledge_items WHERE id = 'item-2'`))
	for _, table := range removedKnowledgeTables {
		assert.False(t, tableExists(t, third, table), table)
	}
}

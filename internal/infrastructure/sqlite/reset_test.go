package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

func TestReset_clearsStudyAndKnowledgeDataButKeepsTheDefaultFolderAndUsage(t *testing.T) {
	// Given a database with a non-default folder, a study session with a
	// message, its usage record and its persisted local sources, and a full
	// set of knowledge-domain rows (item, chunk, evidence, reconciliation
	// proposal, ingested file, and a relation between two items)
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		INSERT INTO folders (id, name, is_default, created_at) VALUES ('custom', 'Custom', 0, CURRENT_TIMESTAMP);
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES ('session-1', 'Go', 'socratic', 'custom', CURRENT_TIMESTAMP);
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-1', 'session-1', 'user', 'hi', CURRENT_TIMESTAMP);
		INSERT INTO usage (id, session_id, model, input_tokens, output_tokens, cost, created_at) VALUES ('usage-1', 'session-1', 'gpt', 10, 20, 0.01, CURRENT_TIMESTAMP);
		INSERT INTO knowledge_items (id, topic, concept, definition, source, status, created_at, updated_at, normalized_concept)
			VALUES ('item-1', 'Go', 'Goroutines', 'lightweight threads', 'athena', 'approved', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'goroutines');
		INSERT INTO knowledge_items (id, topic, concept, definition, source, status, created_at, updated_at, normalized_concept)
			VALUES ('item-2', 'Go', 'Channels', 'goroutine communication', 'athena', 'approved', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'channels');
		INSERT INTO knowledge_item_relations (from_item_id, to_item_id, relation_type, created_at) VALUES ('item-1', 'item-2', 'related', CURRENT_TIMESTAMP);
		INSERT INTO knowledge_chunks (id, source, topic, status, item_id, content, embedding, embedding_model, item_updated_at, created_at)
			VALUES ('chunk-1', 'athena', 'Go', 'approved', 'item-1', 'goroutines are cheap', x'0000', 'test-model', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO knowledge_evidence (id, origin_type, origin_id, source_label, excerpt, created_at)
			VALUES ('evidence-1', 'study_session', 'session-1', 'Go session', 'goroutines are cheap', CURRENT_TIMESTAMP);
		INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES ('item-1', 'evidence-1');
		INSERT INTO knowledge_reconciliation_proposals (id, action, status, candidate_snapshot, target_item_id, reason, changes, created_at)
			VALUES ('proposal-1', 'update', 'pending', '{}', 'item-1', 'similar concept', '{}', CURRENT_TIMESTAMP);
		INSERT INTO knowledge_reconciliation_evidence (proposal_id, evidence_id) VALUES ('proposal-1', 'evidence-1');
		INSERT INTO ingested_files (source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
			VALUES ('/notes/go.md', 'go.md', 1, 'test-model', 1, 'item-1', CURRENT_TIMESTAMP);
		INSERT INTO message_sources (message_id, position, chunk_id, item_id, source_type, score, excerpt)
			VALUES ('message-1', 0, 'chunk-1', 'item-1', 'athena', 0.9, 'goroutines are cheap');
	`)
	require.NoError(t, err)

	resetter := NewResetter(db)

	// When resetting local data
	resetErr := resetter.Reset(context.Background())

	// Then it succeeds
	require.NoError(t, resetErr)

	// And every study/knowledge table is empty
	for _, table := range []string{
		"sessions", "messages", "knowledge_items", "knowledge_chunks",
		"knowledge_evidence", "knowledge_item_evidence", "knowledge_item_relations",
		"knowledge_reconciliation_proposals", "knowledge_reconciliation_evidence",
		"ingested_files", "message_sources",
	} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count))
		assert.Equalf(t, 0, count, "expected table %s to be empty", table)
	}

	// And the default folder survives, but the custom one does not
	var folderCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM folders WHERE id = 'custom'`).Scan(&folderCount))
	assert.Equal(t, 0, folderCount)
	var defaultFolderName string
	require.NoError(t, db.QueryRow(`SELECT name FROM folders WHERE id = ?`, domainfolder.DefaultFolderID).Scan(&defaultFolderName))
	assert.Equal(t, "General", defaultFolderName)

	// And the usage record survives, detached from its now-deleted session
	var usageSessionID *string
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'usage-1'`).Scan(&usageSessionID))
	assert.Nil(t, usageSessionID)
}

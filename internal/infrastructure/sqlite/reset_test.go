package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReset_clearsStudyAndKnowledgeDataAndEveryFolder(t *testing.T) {
	// Given a database with a folder, a study session with a
	// message, its usage record and its persisted local sources, and a full
	// set of knowledge-domain rows (item, chunk and ingested file)
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		INSERT INTO folders (id, name, created_at) VALUES ('custom', 'Custom', CURRENT_TIMESTAMP);
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES ('session-1', 'Go', 'socratic', 'custom', CURRENT_TIMESTAMP);
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-1', 'session-1', 'user', 'hi', CURRENT_TIMESTAMP);
		INSERT INTO usage (id, session_id, model, input_tokens, output_tokens, cost, created_at) VALUES ('usage-1', 'session-1', 'gpt', 10, 20, 0.01, CURRENT_TIMESTAMP);
		INSERT INTO knowledge_items (id, session_id, topic, concept, definition, source, created_at, updated_at)
			VALUES ('item-1', 'session-1', 'Go', 'go.md', 'lightweight threads', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO knowledge_chunks (id, session_id, source, topic, item_id, content, embedding, embedding_model, created_at)
			VALUES ('chunk-1', 'session-1', 'imported_doc', 'Go', 'item-1', 'goroutines are cheap', x'0000', 'test-model', CURRENT_TIMESTAMP);
		INSERT INTO ingested_files (session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
			VALUES ('session-1', '/notes/go.md', 'go.md', 1, 'test-model', 1, 'item-1', CURRENT_TIMESTAMP);
		INSERT INTO message_sources (message_id, position, chunk_id, item_id, source_type, score, excerpt)
			VALUES ('message-1', 0, 'chunk-1', 'item-1', 'imported_doc', 0.9, 'goroutines are cheap');
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
		"ingested_files", "message_sources",
	} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count))
		assert.Equalf(t, 0, count, "expected table %s to be empty", table)
	}

	// And every folder is gone
	var folderCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM folders`).Scan(&folderCount))
	assert.Equal(t, 0, folderCount)

	// And the usage record survives, detached from its now-deleted session
	var usageSessionID *string
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = 'usage-1'`).Scan(&usageSessionID))
	assert.Nil(t, usageSessionID)
}

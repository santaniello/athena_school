package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReset_clearsEveryArchitectureLabTable(t *testing.T) {
	// Given a lab with one row in every Architecture Lab table, linked to a
	// session and a message, plus a pending offer in another session
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		INSERT INTO folders (id, name, created_at) VALUES ('custom', 'Custom', CURRENT_TIMESTAMP);
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES ('session-1', 'Go', 'socratic', 'custom', CURRENT_TIMESTAMP);
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES ('session-2', 'Go', 'socratic', 'custom', CURRENT_TIMESTAMP);
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-1', 'session-1', 'assistant', 'hi', CURRENT_TIMESTAMP);
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-2', 'session-2', 'assistant', 'hi', CURRENT_TIMESTAMP);
		INSERT INTO challenges (id, type, title, scenario, functional_requirements, non_functional_requirements,
			constraints, objectives, difficulty, topic, source_session_id, origin, created_at, updated_at)
			VALUES ('challenge-1', 'ARCHITECTURE', 'Cache', 's', '[]', '[]', '[]', '[]', 'ADVANCED', 'caching', 'session-1', 'AI', 'now', 'now');
		INSERT INTO rubric_versions (id, challenge_id, version, base_version, created_at) VALUES ('rv-1', 'challenge-1', 1, 'v1', 'now');
		INSERT INTO rubric_criteria (id, rubric_version_id, position, key, name, description, weight, source, mandatory, levels)
			VALUES ('crit-1', 'rv-1', 1, 'k', 'n', 'd', 100, 'BASE', 1, '[]');
		INSERT INTO session_exercises (id, session_id, challenge_id, message_id, created_at) VALUES ('link-1', 'session-1', 'challenge-1', 'message-1', 'now');
		INSERT INTO challenge_attempts (id, challenge_id, rubric_version_id, session_exercise_id, state, started_at, deadline)
			VALUES ('attempt-1', 'challenge-1', 'rv-1', 'link-1', 'FINISHED', 'now', 'later');
		INSERT INTO submissions (id, attempt_id, content_type, content, checksum, submitted_at) VALUES ('sub-1', 'attempt-1', 'x', 'c', 'sum', 'now');
		INSERT INTO evaluations (id, attempt_id, kind, model, score, summary, strengths, gaps, recommendations, created_at)
			VALUES ('eval-1', 'attempt-1', 'DIAGRAM', 'm', 3, 's', '[]', '[]', '[]', 'now');
		INSERT INTO criterion_results (evaluation_id, criterion_id, level, justification, evidence, insufficient_evidence)
			VALUES ('eval-1', 'crit-1', 0, 'j', 'e', 0);
		INSERT INTO attempt_hints (id, attempt_id, position, content, created_at) VALUES ('hint-1', 'attempt-1', 1, 'h', 'now');
		INSERT INTO defense_turns (id, attempt_id, position, criterion_id, question, asked_at) VALUES ('turn-1', 'attempt-1', 1, 'crit-1', 'q', 'now');
		INSERT INTO exercise_offers (id, session_id, message_id, created_at) VALUES ('offer-1', 'session-2', 'message-2', 'now');
	`)
	require.NoError(t, err)

	// When resetting local data
	require.NoError(t, NewResetter(db).Reset(context.Background()))

	// Then every Architecture Lab table is empty
	for _, table := range []string{
		"challenges", "rubric_versions", "rubric_criteria", "session_exercises", "challenge_attempts",
		"submissions", "evaluations", "criterion_results", "attempt_hints", "defense_turns", "exercise_offers",
	} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count))
		assert.Equalf(t, 0, count, "expected table %s to be empty", table)
	}
}

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

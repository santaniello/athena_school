package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

func setupEvidenceFixture(t *testing.T) (*sql.DB, *EvidenceRepository, *ConceptStateRepository) {
	t.Helper()
	db, concepts := setupConceptFixture(t)
	_, err := db.Exec(`
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES
			('user-1', 'session-1', 'user', 'q', CURRENT_TIMESTAMP),
			('reply-1', 'session-1', 'assistant', 'a', CURRENT_TIMESTAMP),
			('user-2', 'session-2', 'user', 'q', CURRENT_TIMESTAMP),
			('reply-2', 'session-2', 'assistant', 'a', CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
	return db, NewEvidenceRepository(db), concepts
}

func evidence(sessionID, conceptName string, age time.Duration) domainstudy.Evidence {
	userID, replyID := "user-1", "reply-1"
	if sessionID == "session-2" {
		userID, replyID = "user-2", "reply-2"
	}
	return domainstudy.Evidence{
		SessionID:      sessionID,
		Concept:        conceptName,
		Assessment:     domainstudy.AssessmentPartiallyCorrect,
		Description:    fmt.Sprintf("proof %s", age),
		Assisted:       true,
		UserMessageID:  userID,
		ReplyMessageID: replyID,
		CreatedAt:      conceptBase.Add(-age),
	}
}

func evidenceDescriptions(t *testing.T, repo *EvidenceRepository, sessionID string) []string {
	t.Helper()
	list, err := repo.ListBySession(context.Background(), sessionID)
	require.NoError(t, err)
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.Description)
	}
	return out
}

func TestEvidenceRepository_AddThenList_roundTripsEveryField(t *testing.T) {
	// Given a concept and a stored proof
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	want := evidence("session-1", "mutex", 0)
	require.NoError(t, repo.Add(ctx, want))

	// When listing the session's proofs
	got, err := repo.ListBySession(ctx, "session-1")

	// Then the row comes back unchanged
	require.NoError(t, err)
	assert.Equal(t, []domainstudy.Evidence{want}, got)
}

func TestEvidenceRepository_ListBySession_isScopedAndOldestFirst(t *testing.T) {
	// Given proofs saved out of order, in two sessions
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, concepts.Upsert(ctx, concept("session-2", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", 2*time.Hour)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", time.Hour)))
	require.NoError(t, repo.Add(ctx, evidence("session-2", "mutex", 0)))

	// When listing session-1
	// Then only its proofs come back, oldest first
	assert.Equal(t, []string{"proof 2h0m0s", "proof 1h0m0s", "proof 0s"}, evidenceDescriptions(t, repo, "session-1"))
	assert.Len(t, evidenceDescriptions(t, repo, "session-2"), 1)
	assert.Empty(t, evidenceDescriptions(t, repo, "no-such-session"))
}

func TestEvidenceRepository_Add_rejectsAProofWithoutItsConceptRow(t *testing.T) {
	// Given no concept row
	_, repo, _ := setupEvidenceFixture(t)

	// When adding a proof for it
	err := repo.Add(context.Background(), evidence("session-1", "ghost", 0))

	// Then the foreign key refuses it
	require.Error(t, err)
	assert.Empty(t, evidenceDescriptions(t, repo, "session-1"))
}

func TestEvidenceRepository_Add_rejectsAProofWithoutItsMessages(t *testing.T) {
	// Given a concept row and a proof naming a message that does not exist
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))

	for name, mutate := range map[string]func(*domainstudy.Evidence){
		"user message":  func(e *domainstudy.Evidence) { e.UserMessageID = "missing" },
		"reply message": func(e *domainstudy.Evidence) { e.ReplyMessageID = "missing" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := evidence("session-1", "mutex", 0)
			mutate(&bad)

			// When adding it
			err := repo.Add(ctx, bad)

			// Then the foreign key refuses it
			require.Error(t, err)
		})
	}
}

func TestEvidenceRepository_Add_rejectsAnUnknownAssessment(t *testing.T) {
	// Given a concept row and a proof with an assessment outside the three
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	bad := evidence("session-1", "mutex", 0)
	bad.Assessment = domainstudy.Assessment("GOOD")

	// When adding it
	// Then the database refuses it
	require.Error(t, repo.Add(ctx, bad))
}

func TestEvidenceRepository_TrimOldest_keepsTheMostRecentOfOneConcept(t *testing.T) {
	// Given four proofs of one concept and one of another, in the same session
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "channel", domainstudy.StatusIntroduced, 0)))
	for _, age := range []time.Duration{4 * time.Hour, 3 * time.Hour, 2 * time.Hour, time.Hour} {
		require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", age)))
	}
	other := evidence("session-1", "channel", 9*time.Hour)
	require.NoError(t, repo.Add(ctx, other))

	// When trimming the first concept to two proofs
	require.NoError(t, repo.TrimOldest(ctx, "session-1", "mutex", 2))

	// Then its two most recent remain and the other concept is untouched
	assert.Equal(t, []string{other.Description, "proof 2h0m0s", "proof 1h0m0s"}, evidenceDescriptions(t, repo, "session-1"))
}

func TestEvidenceRepository_TrimOldest_underTheLimit_deletesNothing(t *testing.T) {
	// Given two proofs
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", time.Hour)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", 0)))

	// When trimming to a limit above the count
	require.NoError(t, repo.TrimOldest(ctx, "session-1", "mutex", 5))

	// Then both remain
	assert.Len(t, evidenceDescriptions(t, repo, "session-1"), 2)
}

func TestEvidence_followsItsConcept_whenTheConceptIsEvicted(t *testing.T) {
	// Given proofs of two concepts
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "old", domainstudy.StatusDemonstrated, time.Hour)))
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "current", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "old", time.Hour)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "current", 0)))

	// When the concept limit evicts the old one
	require.NoError(t, concepts.DeleteOldest(ctx, "session-1", "current"))

	// Then its proofs go with it
	assert.Equal(t, []string{"proof 0s"}, evidenceDescriptions(t, repo, "session-1"))
}

func TestEvidence_staysAfterAConceptUpsert(t *testing.T) {
	// Given a proof of a concept
	_, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, time.Hour)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", 0)))

	// When the concept row is updated in place
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusDemonstrated, 0)))

	// Then the proof is still there (an update must never cascade)
	assert.Len(t, evidenceDescriptions(t, repo, "session-1"), 1)
}

func TestSessionDelete_removesItsEvidence(t *testing.T) {
	// Given proofs in two sessions
	db, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, concepts.Upsert(ctx, concept("session-2", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-2", "mutex", 0)))

	// When deleting session-1
	require.NoError(t, NewSessionRepository(db).Delete(ctx, "session-1"))

	// Then its proofs go with it, and the other session keeps its own
	assert.Empty(t, evidenceDescriptions(t, repo, "session-1"))
	assert.Len(t, evidenceDescriptions(t, repo, "session-2"), 1)
}

func TestReset_clearsEvidence(t *testing.T) {
	// Given a stored proof
	db, repo, concepts := setupEvidenceFixture(t)
	ctx := context.Background()
	require.NoError(t, concepts.Upsert(ctx, concept("session-1", "mutex", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Add(ctx, evidence("session-1", "mutex", 0)))

	// When resetting local data
	require.NoError(t, NewResetter(db).Reset(ctx))

	// Then the table is empty
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM concept_evidence`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestOpen_createsConceptEvidenceTable_onADatabaseThatPredatesIt(t *testing.T) {
	// Given a database opened before the concept_evidence migration existed
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	_, err = first.Exec(`DROP TABLE concept_evidence`)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the table exists again
	var tableName string
	require.NoError(t, db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'concept_evidence'`,
	).Scan(&tableName))
}

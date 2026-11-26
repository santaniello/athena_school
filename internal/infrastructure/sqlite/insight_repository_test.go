package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

var insightEpoch = time.Date(2026, 12, 1, 9, 0, 0, 123456789, time.UTC)

func newTestInsightRepository(t *testing.T) (*InsightRepository, *sql.DB) {
	t.Helper()
	_, db := newTestChallengeRepository(t) // seeds folder-a and session-1
	insertTestSession(t, db, "session-2", "queues")
	return NewInsightRepository(db), db
}

func newTestInsight(analysis string) domainprogress.Insight {
	return domainprogress.Insight{
		Analysis: []domainprogress.Statement{
			{Text: analysis, Citations: []domainprogress.Citation{{Kind: domainprogress.CitationQuiz, ID: "q1", Label: "Quiz 1"}}},
			{Text: "Second sentence.", Citations: []domainprogress.Citation{}},
		},
		Recommendations: []domainprogress.Statement{
			{Text: "Retake it.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationChallenge, ID: "c1", Label: "Design a cache"}}},
		},
		Model: "anthropic/x", Language: "pt", Fingerprint: "abc123", CreatedAt: insightEpoch,
	}
}

func TestInsightRepository_SaveThenGet_roundTripsEveryField(t *testing.T) {
	// Given an insight of a topic
	repo, _ := newTestInsightRepository(t)
	ctx := context.Background()
	original := newTestInsight("First sentence.")

	// When storing it and reading it back
	require.NoError(t, repo.Save(ctx, "session-1", original))
	got, err := repo.Get(ctx, "session-1")

	// Then every field is preserved
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestInsightRepository_Save_replacesThePreviousInsightOfTheTopic(t *testing.T) {
	// Given a stored insight and another topic with its own
	repo, db := newTestInsightRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "session-1", newTestInsight("Old.")))
	require.NoError(t, repo.Save(ctx, "session-2", newTestInsight("Other topic.")))

	// When the first topic is saved again
	require.NoError(t, repo.Save(ctx, "session-1", newTestInsight("New.")))

	// Then only that one changed and there is still one row per topic
	got, err := repo.Get(ctx, "session-1")
	require.NoError(t, err)
	assert.Equal(t, "New.", got.Analysis[0].Text)
	other, err := repo.Get(ctx, "session-2")
	require.NoError(t, err)
	assert.Equal(t, "Other topic.", other.Analysis[0].Text)
	var rows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM topic_insights`).Scan(&rows))
	assert.Equal(t, 2, rows)
}

func TestInsightRepository_Get_isNotFoundForATopicWithoutInsight(t *testing.T) {
	// Given no stored insight
	repo, _ := newTestInsightRepository(t)

	// When reading one
	_, err := repo.Get(context.Background(), "session-1")

	// Then it is ErrInsightNotFound
	require.ErrorIs(t, err, domainprogress.ErrInsightNotFound)
}

func TestInsightRepository_Save_failsForAnUnknownSession(t *testing.T) {
	// Given a session that does not exist
	repo, _ := newTestInsightRepository(t)

	// When saving an insight for it
	err := repo.Save(context.Background(), "ghost", newTestInsight("X."))

	// Then the foreign key refuses it
	require.Error(t, err)
}

func TestInsightRepository_isDeletedWithItsSession(t *testing.T) {
	// Given a stored insight
	repo, db := newTestInsightRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "session-2", newTestInsight("X.")))

	// When the session is deleted
	_, err := db.Exec(`DELETE FROM sessions WHERE id = 'session-2'`)
	require.NoError(t, err)

	// Then the insight goes with it
	_, err = repo.Get(ctx, "session-2")
	require.ErrorIs(t, err, domainprogress.ErrInsightNotFound)
}

func TestInsightRepository_Get_failsOnACorruptPayload(t *testing.T) {
	// Given a row whose payload is not JSON
	repo, db := newTestInsightRepository(t)
	_, err := db.Exec(`INSERT INTO topic_insights (session_id, analysis, payload, model, language, fingerprint, created_at)
		VALUES ('session-1', 'a', 'not json', 'm', 'en', 'f', '2026-12-01T09:00:00Z')`)
	require.NoError(t, err)

	// When reading it
	_, err = repo.Get(context.Background(), "session-1")

	// Then the failure is visible
	require.Error(t, err)
	assert.NotErrorIs(t, err, domainprogress.ErrInsightNotFound)
}

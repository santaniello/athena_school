package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

func newTestFolderInsightRepository(t *testing.T) (*FolderInsightRepository, *sql.DB) {
	t.Helper()
	_, db := newTestChallengeRepository(t) // seeds folder-a and session-1
	mustExec(t, db, `INSERT INTO folders (id, name, created_at) VALUES ('folder-b', 'B', CURRENT_TIMESTAMP)`)
	return NewFolderInsightRepository(db), db
}

func newTestFolderInsight(analysis string) domainprogress.Insight {
	return domainprogress.Insight{
		Analysis: []domainprogress.Statement{
			{Text: analysis, Citations: []domainprogress.Citation{{Kind: domainprogress.CitationTopic, ID: "session-1", Label: "Cache"}}},
			{Text: "Second sentence.", Citations: []domainprogress.Citation{}},
		},
		Recommendations: []domainprogress.Statement{
			{Text: "Review queues.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationTopic, ID: "session-2", Label: "Queues"}}},
		},
		Model: "anthropic/x", Language: "pt", Fingerprint: "abc123", CreatedAt: insightEpoch,
	}
}

func TestFolderInsightRepository_SaveThenGet_roundTripsEveryField(t *testing.T) {
	// Given an insight of a folder
	repo, _ := newTestFolderInsightRepository(t)
	ctx := context.Background()
	original := newTestFolderInsight("First sentence.")

	// When storing it and reading it back
	require.NoError(t, repo.Save(ctx, "folder-a", original))
	got, err := repo.Get(ctx, "folder-a")

	// Then every field is preserved
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestFolderInsightRepository_Save_replacesThePreviousInsightOfTheFolder(t *testing.T) {
	// Given a stored insight and another folder with its own
	repo, db := newTestFolderInsightRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "folder-a", newTestFolderInsight("Old.")))
	require.NoError(t, repo.Save(ctx, "folder-b", newTestFolderInsight("Other folder.")))

	// When the first folder is saved again
	require.NoError(t, repo.Save(ctx, "folder-a", newTestFolderInsight("New.")))

	// Then only that one changed and there is still one row per folder
	got, err := repo.Get(ctx, "folder-a")
	require.NoError(t, err)
	assert.Equal(t, "New.", got.Analysis[0].Text)
	other, err := repo.Get(ctx, "folder-b")
	require.NoError(t, err)
	assert.Equal(t, "Other folder.", other.Analysis[0].Text)
	var rows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM folder_insights`).Scan(&rows))
	assert.Equal(t, 2, rows)
}

func TestFolderInsightRepository_Get_isNotFoundForAFolderWithoutInsight(t *testing.T) {
	repo, _ := newTestFolderInsightRepository(t)

	_, err := repo.Get(context.Background(), "folder-a")

	require.ErrorIs(t, err, domainprogress.ErrInsightNotFound)
}

func TestFolderInsightRepository_Save_failsForAnUnknownFolder(t *testing.T) {
	repo, _ := newTestFolderInsightRepository(t)

	err := repo.Save(context.Background(), "ghost", newTestFolderInsight("X."))

	require.Error(t, err)
}

func TestFolderInsightRepository_isDeletedWithItsFolder(t *testing.T) {
	// Given a stored insight
	repo, db := newTestFolderInsightRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "folder-b", newTestFolderInsight("X.")))

	// When the folder is deleted
	mustExec(t, db, `DELETE FROM folders WHERE id = 'folder-b'`)

	// Then the insight goes with it
	_, err := repo.Get(ctx, "folder-b")
	require.ErrorIs(t, err, domainprogress.ErrInsightNotFound)
}

func TestFolderInsightRepository_Get_failsOnACorruptPayload(t *testing.T) {
	repo, db := newTestFolderInsightRepository(t)
	mustExec(t, db, `INSERT INTO folder_insights (folder_id, analysis, payload, model, language, fingerprint, created_at)
		VALUES ('folder-a', 'a', 'not json', 'm', 'en', 'f', '2026-12-01T09:00:00Z')`)

	_, err := repo.Get(context.Background(), "folder-a")

	require.Error(t, err)
	assert.NotErrorIs(t, err, domainprogress.ErrInsightNotFound)
}

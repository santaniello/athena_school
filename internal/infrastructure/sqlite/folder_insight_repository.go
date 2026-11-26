package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// FolderInsightRepository is the SQLite-backed implementation of
// progress.FolderInsightRepository: the latest Athena Insights of each folder,
// one row per folder, stored like the ones of a topic.
type FolderInsightRepository struct {
	db *sql.DB
}

// NewFolderInsightRepository creates a FolderInsightRepository backed by db. db
// must already have its migrations applied (see Open).
func NewFolderInsightRepository(db *sql.DB) *FolderInsightRepository {
	return &FolderInsightRepository{db: db}
}

// Save stores the insight of the folder, replacing the previous one. The
// folder must exist.
func (r *FolderInsightRepository) Save(ctx context.Context, folderID string, insight domainprogress.Insight) error {
	payload, err := encodeInsightPayload(insight)
	if err != nil {
		return err
	}
	_, err = execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO folder_insights (folder_id, analysis, payload, model, language, fingerprint, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(folder_id) DO UPDATE SET analysis = excluded.analysis, payload = excluded.payload,
		   model = excluded.model, language = excluded.language, fingerprint = excluded.fingerprint,
		   created_at = excluded.created_at`,
		folderID, analysisText(insight.Analysis), payload, insight.Model, insight.Language,
		insight.Fingerprint, formatChallengeTime(insight.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving folder insight: %w", err)
	}
	return nil
}

// Get returns the insight of the folder, or domainprogress.ErrInsightNotFound.
func (r *FolderInsightRepository) Get(ctx context.Context, folderID string) (domainprogress.Insight, error) {
	var (
		insight          domainprogress.Insight
		payload, created string
	)
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT payload, model, language, fingerprint, created_at FROM folder_insights WHERE folder_id = ?`, folderID,
	).Scan(&payload, &insight.Model, &insight.Language, &insight.Fingerprint, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return domainprogress.Insight{}, domainprogress.ErrInsightNotFound
	}
	if err != nil {
		return domainprogress.Insight{}, fmt.Errorf("sqlite: reading folder insight: %w", err)
	}
	if err := decodeInsightPayload(payload, created, &insight); err != nil {
		return domainprogress.Insight{}, fmt.Errorf("sqlite: reading folder insight of %q: %w", folderID, err)
	}
	return insight, nil
}

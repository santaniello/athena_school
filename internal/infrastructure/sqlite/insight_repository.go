package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// InsightRepository is the SQLite-backed implementation of
// progress.InsightRepository: the latest Athena Insights of each topic, one row
// per session.
type InsightRepository struct {
	db *sql.DB
}

// NewInsightRepository creates an InsightRepository backed by db. db must
// already have its migrations applied (see Open).
func NewInsightRepository(db *sql.DB) *InsightRepository {
	return &InsightRepository{db: db}
}

type insightPayload struct {
	Analysis        []insightStatementJSON `json:"analysis"`
	Recommendations []insightStatementJSON `json:"recommendations"`
}

type insightStatementJSON struct {
	Text      string                `json:"text"`
	Citations []insightCitationJSON `json:"citations"`
}

type insightCitationJSON struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Save stores the insight of the session, replacing the previous one. The
// session must exist.
func (r *InsightRepository) Save(ctx context.Context, sessionID string, insight domainprogress.Insight) error {
	payload, err := encodeInsightPayload(insight)
	if err != nil {
		return err
	}
	_, err = execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO topic_insights (session_id, analysis, payload, model, language, fingerprint, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET analysis = excluded.analysis, payload = excluded.payload,
		   model = excluded.model, language = excluded.language, fingerprint = excluded.fingerprint,
		   created_at = excluded.created_at`,
		sessionID, analysisText(insight.Analysis), payload, insight.Model, insight.Language,
		insight.Fingerprint, formatChallengeTime(insight.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving insight: %w", err)
	}
	return nil
}

// Get returns the insight of the session, or domainprogress.ErrInsightNotFound.
func (r *InsightRepository) Get(ctx context.Context, sessionID string) (domainprogress.Insight, error) {
	var (
		insight          domainprogress.Insight
		payload, created string
	)
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT payload, model, language, fingerprint, created_at FROM topic_insights WHERE session_id = ?`, sessionID,
	).Scan(&payload, &insight.Model, &insight.Language, &insight.Fingerprint, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return domainprogress.Insight{}, domainprogress.ErrInsightNotFound
	}
	if err != nil {
		return domainprogress.Insight{}, fmt.Errorf("sqlite: reading insight: %w", err)
	}
	if err := decodeInsightPayload(payload, created, &insight); err != nil {
		return domainprogress.Insight{}, fmt.Errorf("sqlite: reading insight of %q: %w", sessionID, err)
	}
	return insight, nil
}

func encodeInsightPayload(insight domainprogress.Insight) (string, error) {
	payload, err := json.Marshal(insightPayload{
		Analysis:        encodeStatements(insight.Analysis),
		Recommendations: encodeStatements(insight.Recommendations),
	})
	if err != nil {
		return "", fmt.Errorf("sqlite: encoding insight: %w", err)
	}
	return string(payload), nil
}

// decodeInsightPayload fills the statements and the creation time of the
// insight from the stored payload and timestamp.
func decodeInsightPayload(payload, created string, insight *domainprogress.Insight) error {
	var decoded insightPayload
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return fmt.Errorf("decoding payload: %w", err)
	}
	insight.Analysis = decodeStatements(decoded.Analysis)
	insight.Recommendations = decodeStatements(decoded.Recommendations)
	var err error
	insight.CreatedAt, err = parseChallengeTime(created)
	return err
}

func analysisText(statements []domainprogress.Statement) string {
	texts := make([]string, len(statements))
	for i, statement := range statements {
		texts[i] = statement.Text
	}
	return strings.Join(texts, " ")
}

func encodeStatements(statements []domainprogress.Statement) []insightStatementJSON {
	encoded := make([]insightStatementJSON, len(statements))
	for i, statement := range statements {
		citations := make([]insightCitationJSON, len(statement.Citations))
		for j, citation := range statement.Citations {
			citations[j] = insightCitationJSON{Kind: string(citation.Kind), ID: citation.ID, Label: citation.Label}
		}
		encoded[i] = insightStatementJSON{Text: statement.Text, Citations: citations}
	}
	return encoded
}

// decodeStatements maps the JSON back; a statement without citations has an
// empty, never nil, list, as the domain guarantees.
func decodeStatements(encoded []insightStatementJSON) []domainprogress.Statement {
	statements := make([]domainprogress.Statement, len(encoded))
	for i, statement := range encoded {
		citations := make([]domainprogress.Citation, len(statement.Citations))
		for j, citation := range statement.Citations {
			citations[j] = domainprogress.Citation{Kind: domainprogress.CitationKind(citation.Kind), ID: citation.ID, Label: citation.Label}
		}
		statements[i] = domainprogress.Statement{Text: statement.Text, Citations: citations}
	}
	return statements
}

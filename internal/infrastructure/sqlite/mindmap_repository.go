package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/mindmap"
)

// MindMapRepository is the SQLite-backed implementation of mindmap.Repository.
type MindMapRepository struct {
	db *sql.DB
}

// NewMindMapRepository creates a MindMapRepository backed by db. db must
// already have its migrations applied (see Open).
func NewMindMapRepository(db *sql.DB) *MindMapRepository {
	return &MindMapRepository{db: db}
}

// Create stores the mind map. The session must exist: the map goes with it.
func (r *MindMapRepository) Create(ctx context.Context, m mindmap.MindMap) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO mind_maps (id, session_id, session_title, title, tree, diagram_xml, message_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.SessionID, m.SessionTitle, m.Title, m.Tree, m.DiagramXML, nullableText(m.MessageID),
		formatChallengeTime(m.CreatedAt), formatChallengeTime(m.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: creating mind map: %w", err)
	}
	return nil
}

// Get returns the mind map with its tree and diagram, or mindmap.ErrMindMapNotFound.
func (r *MindMapRepository) Get(ctx context.Context, id string) (mindmap.MindMap, error) {
	var (
		m                  mindmap.MindMap
		messageID          sql.NullString
		createdAt, updated string
	)
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT id, session_id, session_title, title, tree, diagram_xml, message_id, created_at, updated_at
		 FROM mind_maps WHERE id = ?`, id,
	).Scan(&m.ID, &m.SessionID, &m.SessionTitle, &m.Title, &m.Tree, &m.DiagramXML, &messageID, &createdAt, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return mindmap.MindMap{}, mindmap.ErrMindMapNotFound
	}
	if err != nil {
		return mindmap.MindMap{}, fmt.Errorf("sqlite: reading mind map: %w", err)
	}
	m.MessageID = messageID.String
	if m.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return mindmap.MindMap{}, err
	}
	if m.UpdatedAt, err = parseChallengeTime(updated); err != nil {
		return mindmap.MindMap{}, err
	}
	return m, nil
}

const mindMapSummarySelect = `SELECT id, session_id, session_title, title, created_at, updated_at, message_id FROM mind_maps `

// List returns every mind map, newest first, without its tree and diagram.
func (r *MindMapRepository) List(ctx context.Context) ([]mindmap.Summary, error) {
	return r.querySummaries(ctx, `ORDER BY created_at DESC, id DESC`)
}

// FindByMessage returns the mind map the offer under the chat message produced,
// or mindmap.ErrMindMapNotFound. A map from the menu has no message, so it
// never matches.
func (r *MindMapRepository) FindByMessage(ctx context.Context, messageID string) (mindmap.Summary, error) {
	summaries, err := r.querySummaries(ctx, `WHERE message_id = ? ORDER BY created_at ASC, id ASC LIMIT 1`, messageID)
	if err != nil {
		return mindmap.Summary{}, err
	}
	if len(summaries) == 0 {
		return mindmap.Summary{}, mindmap.ErrMindMapNotFound
	}
	return summaries[0], nil
}

// ListBySession returns the mind maps the session's offers produced, oldest
// first. A map from the menu is not one of them.
func (r *MindMapRepository) ListBySession(ctx context.Context, sessionID string) ([]mindmap.Summary, error) {
	return r.querySummaries(ctx, `WHERE session_id = ? AND message_id IS NOT NULL ORDER BY created_at ASC, id ASC`, sessionID)
}

func (r *MindMapRepository) querySummaries(ctx context.Context, tail string, args ...any) ([]mindmap.Summary, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, mindMapSummarySelect+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing mind maps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var summaries []mindmap.Summary
	for rows.Next() {
		var (
			summary            mindmap.Summary
			createdAt, updated string
			messageID          sql.NullString
		)
		if err := rows.Scan(&summary.ID, &summary.SessionID, &summary.SessionTitle, &summary.Title,
			&createdAt, &updated, &messageID); err != nil {
			return nil, fmt.Errorf("sqlite: scanning mind map summary: %w", err)
		}
		summary.MessageID = messageID.String
		if summary.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
			return nil, err
		}
		if summary.UpdatedAt, err = parseChallengeTime(updated); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: reading mind maps: %w", err)
	}
	return summaries, nil
}

// Update replaces the session title snapshot, title, tree, diagram and update
// time of the mind map. Its session, message and creation time stay. It
// returns mindmap.ErrMindMapNotFound for an unknown ID.
func (r *MindMapRepository) Update(ctx context.Context, m mindmap.MindMap) error {
	result, err := execer(ctx, r.db).ExecContext(ctx,
		`UPDATE mind_maps SET session_title = ?, title = ?, tree = ?, diagram_xml = ?, updated_at = ? WHERE id = ?`,
		m.SessionTitle, m.Title, m.Tree, m.DiagramXML, formatChallengeTime(m.UpdatedAt), m.ID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: updating mind map: %w", err)
	}
	return requireRowAffected(result, mindmap.ErrMindMapNotFound)
}

// Delete removes the mind map, or returns mindmap.ErrMindMapNotFound.
func (r *MindMapRepository) Delete(ctx context.Context, id string) error {
	result, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM mind_maps WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting mind map: %w", err)
	}
	return requireRowAffected(result, mindmap.ErrMindMapNotFound)
}

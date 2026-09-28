package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// IngestedFileRepository is the SQLite-backed implementation of
// knowledge.IngestedFileRepository.
type IngestedFileRepository struct {
	db *sql.DB
}

// NewIngestedFileRepository creates an IngestedFileRepository backed by
// db. db must already have its migrations applied (see Open).
func NewIngestedFileRepository(db *sql.DB) *IngestedFileRepository {
	return &IngestedFileRepository{db: db}
}

// ListBySession returns sessionID's ingested files, keyed by SourcePath —
// one query per import.
func (r *IngestedFileRepository) ListBySession(ctx context.Context, sessionID string) (map[string]knowledge.IngestedFile, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id
		 FROM ingested_files WHERE session_id = ?`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing ingested files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	files := map[string]knowledge.IngestedFile{}
	for rows.Next() {
		var file knowledge.IngestedFile
		if err := rows.Scan(
			&file.SessionID, &file.SourcePath, &file.Path, &file.MTimeUnixNano, &file.EmbeddingModel, &file.ChunkCount, &file.ItemID,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scanning ingested file: %w", err)
		}
		files[file.SourcePath] = file
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating ingested files: %w", err)
	}
	return files, nil
}

// Upsert inserts file, or replaces the existing row for
// (file.SessionID, file.SourcePath).
func (r *IngestedFileRepository) Upsert(ctx context.Context, file knowledge.IngestedFile) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO ingested_files (session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(session_id, source_path) DO UPDATE SET
		   file_path = excluded.file_path,
		   mtime_unix_nano = excluded.mtime_unix_nano,
		   embedding_model = excluded.embedding_model,
		   chunk_count = excluded.chunk_count,
		   item_id = excluded.item_id,
		   ingested_at = excluded.ingested_at`,
		file.SessionID, file.SourcePath, file.Path, file.MTimeUnixNano, file.EmbeddingModel, file.ChunkCount, file.ItemID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upserting ingested file %s: %w", file.SourcePath, err)
	}
	return nil
}

// ListSourcesBySession returns sessionID's imported documents for the
// Sources panel, oldest-imported first. ingested_files is joined to
// knowledge_items on item_id, so a record whose item was deleted (e.g. by
// RemoveSource) is silently excluded rather than surfaced as a broken row.
func (r *IngestedFileRepository) ListSourcesBySession(ctx context.Context, sessionID string) ([]knowledge.SessionSource, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT ingested_files.item_id, knowledge_items.concept, ingested_files.file_path,
		        ingested_files.chunk_count, ingested_files.ingested_at
		 FROM ingested_files
		 JOIN knowledge_items ON knowledge_items.id = ingested_files.item_id
		 WHERE ingested_files.session_id = ?
		 ORDER BY ingested_files.ingested_at ASC`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing session sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sources := []knowledge.SessionSource{}
	for rows.Next() {
		var source knowledge.SessionSource
		if err := rows.Scan(&source.ItemID, &source.Title, &source.Path, &source.ChunkCount, &source.IngestedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning session source: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating session sources: %w", err)
	}
	return sources, nil
}

// DeleteByItemID removes the ingested-file record owned by itemID within
// sessionID. It is a no-op, not an error, when no such record exists.
func (r *IngestedFileRepository) DeleteByItemID(ctx context.Context, sessionID, itemID string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`DELETE FROM ingested_files WHERE session_id = ? AND item_id = ?`, sessionID, itemID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: deleting ingested file for item %s: %w", itemID, err)
	}
	return nil
}

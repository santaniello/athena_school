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

// ListAll returns every ingested file, keyed by path — one query per import.
func (r *IngestedFileRepository) ListAll(ctx context.Context) (map[string]knowledge.IngestedFile, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT file_path, mtime, embedding_model, chunk_count, item_id FROM ingested_files`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing ingested files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	files := map[string]knowledge.IngestedFile{}
	for rows.Next() {
		var file knowledge.IngestedFile
		if err := rows.Scan(&file.Path, &file.MTime, &file.EmbeddingModel, &file.ChunkCount, &file.ItemID); err != nil {
			return nil, fmt.Errorf("sqlite: scanning ingested file: %w", err)
		}
		files[file.Path] = file
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating ingested files: %w", err)
	}
	return files, nil
}

// Upsert inserts file, or replaces the existing row for file.Path.
func (r *IngestedFileRepository) Upsert(ctx context.Context, file knowledge.IngestedFile) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO ingested_files (file_path, mtime, embedding_model, chunk_count, item_id, ingested_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(file_path) DO UPDATE SET
		   mtime = excluded.mtime,
		   embedding_model = excluded.embedding_model,
		   chunk_count = excluded.chunk_count,
		   item_id = excluded.item_id,
		   ingested_at = excluded.ingested_at`,
		file.Path, file.MTime, file.EmbeddingModel, file.ChunkCount, file.ItemID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upserting ingested file %s: %w", file.Path, err)
	}
	return nil
}

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// ChunkRepository is the SQLite-backed implementation of
// knowledge.ChunkRepository.
type ChunkRepository struct {
	db *sql.DB
}

// NewChunkRepository creates a ChunkRepository backed by db. db must
// already have its migrations applied (see Open).
func NewChunkRepository(db *sql.DB) *ChunkRepository {
	return &ChunkRepository{db: db}
}

const chunkColumns = `id, source, topic, status, item_id, file_path, heading, content, embedding, embedding_model, item_updated_at, created_at`

// SaveAll inserts every chunk. Callers are responsible for deleting any
// previous chunks for the same file/item first (see DeleteByFilePath) —
// SaveAll never overwrites, it only inserts.
func (r *ChunkRepository) SaveAll(ctx context.Context, chunks []knowledge.Chunk) error {
	for _, chunk := range chunks {
		_, err := execer(ctx, r.db).ExecContext(ctx,
			`INSERT INTO knowledge_chunks (`+chunkColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chunk.ID, chunk.Source, chunk.Topic, chunk.Status, chunk.ItemID, chunk.FilePath,
			chunk.Heading, chunk.Content, encodeEmbedding(chunk.Embedding), chunk.EmbeddingModel,
			toNullTime(chunk.ItemUpdatedAt), chunk.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("sqlite: saving knowledge chunk %s: %w", chunk.ID, err)
		}
	}
	return nil
}

// ListAll returns every chunk, oldest first.
func (r *ChunkRepository) ListAll(ctx context.Context) ([]knowledge.Chunk, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+chunkColumns+` FROM knowledge_chunks ORDER BY created_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing knowledge chunks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	chunks := []knowledge.Chunk{}
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning knowledge chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating knowledge chunks: %w", err)
	}
	return chunks, nil
}

// DeleteByFilePath removes every chunk previously produced by path. It is
// a no-op, not an error, when no chunk matches.
func (r *ChunkRepository) DeleteByFilePath(ctx context.Context, path string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE file_path = ?`, path)
	if err != nil {
		return fmt.Errorf("sqlite: deleting knowledge chunks by file path: %w", err)
	}
	return nil
}

// DeleteByItemID removes every chunk owned by itemID. It is a no-op, not
// an error, when no chunk matches.
func (r *ChunkRepository) DeleteByItemID(ctx context.Context, itemID string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE item_id = ?`, itemID)
	if err != nil {
		return fmt.Errorf("sqlite: deleting knowledge chunks by item id: %w", err)
	}
	return nil
}

func scanChunk(scanner rowScanner) (knowledge.Chunk, error) {
	var chunk knowledge.Chunk
	var embedding []byte
	var itemUpdatedAt sql.NullTime
	err := scanner.Scan(
		&chunk.ID, &chunk.Source, &chunk.Topic, &chunk.Status, &chunk.ItemID, &chunk.FilePath,
		&chunk.Heading, &chunk.Content, &embedding, &chunk.EmbeddingModel,
		&itemUpdatedAt, &chunk.CreatedAt,
	)
	if err != nil {
		return knowledge.Chunk{}, err
	}

	chunk.Embedding, err = decodeEmbedding(embedding)
	if err != nil {
		return knowledge.Chunk{}, fmt.Errorf("decoding embedding for chunk %s: %w", chunk.ID, err)
	}
	chunk.ItemUpdatedAt = fromNullTime(itemUpdatedAt)
	return chunk, nil
}

func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func fromNullTime(nt sql.NullTime) time.Time {
	if !nt.Valid {
		return time.Time{}
	}
	return nt.Time
}

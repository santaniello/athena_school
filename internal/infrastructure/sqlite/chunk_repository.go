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

// DeleteByFilePath removes every chunk previously produced by path and
// returns the IDs removed. It is a no-op, not an error, when no chunk
// matches.
func (r *ChunkRepository) DeleteByFilePath(ctx context.Context, path string) ([]string, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`DELETE FROM knowledge_chunks WHERE file_path = ? RETURNING id`, path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: deleting knowledge chunks by file path: %w", err)
	}
	ids, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("sqlite: reading ids deleted by file path: %w", err)
	}
	return ids, nil
}

// DeleteByItemID removes every chunk owned by itemID and returns the IDs
// removed. It is a no-op, not an error, when no chunk matches.
func (r *ChunkRepository) DeleteByItemID(ctx context.Context, itemID string) ([]string, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`DELETE FROM knowledge_chunks WHERE item_id = ? RETURNING id`, itemID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: deleting knowledge chunks by item id: %w", err)
	}
	ids, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("sqlite: reading ids deleted by item id: %w", err)
	}
	return ids, nil
}

// UpdateMetadataByItemID overwrites topic/status on every chunk owned by
// itemID and returns the updated rows. It is a no-op, not an error, when no
// chunk matches.
func (r *ChunkRepository) UpdateMetadataByItemID(ctx context.Context, itemID, topic, status string) ([]knowledge.Chunk, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`UPDATE knowledge_chunks SET topic = ?, status = ? WHERE item_id = ? RETURNING `+chunkColumns,
		topic, status, itemID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: updating knowledge chunk metadata by item id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	chunks := []knowledge.Chunk{}
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning updated knowledge chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating updated knowledge chunks: %w", err)
	}
	return chunks, nil
}

// chunkLoadCurrentQuery backs ListCurrent: a LEFT JOIN so a chunk whose
// owning knowledge_items row is missing still comes back (with NULL item_*
// columns) and can be reported as a ChunkLoadIssue rather than silently
// vanishing from the result set.
const chunkLoadCurrentQuery = `
	SELECT c.id, c.source, c.topic, c.status, c.item_id, c.file_path, c.heading, c.content,
	       c.embedding, c.embedding_model, c.item_updated_at, c.created_at,
	       i.id, i.topic, i.status, i.source, i.updated_at
	FROM knowledge_chunks c
	LEFT JOIN knowledge_items i ON i.id = c.item_id
	WHERE c.embedding_model = ?
	ORDER BY c.created_at ASC, c.id ASC`

// ListCurrent returns only chunks safe to index. See
// knowledge.ChunkRepository.ListCurrent for the exact freshness rules;
// scanCurrentChunkRow applies them per row.
func (r *ChunkRepository) ListCurrent(ctx context.Context, embeddingModel string) (knowledge.ChunkLoadResult, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, chunkLoadCurrentQuery, embeddingModel)
	if err != nil {
		return knowledge.ChunkLoadResult{}, fmt.Errorf("sqlite: listing current knowledge chunks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := knowledge.ChunkLoadResult{Chunks: []knowledge.Chunk{}, Issues: []knowledge.ChunkLoadIssue{}}
	for rows.Next() {
		chunk, issue, err := scanCurrentChunkRow(rows)
		if err != nil {
			return knowledge.ChunkLoadResult{}, fmt.Errorf("sqlite: scanning current knowledge chunk: %w", err)
		}
		if issue != nil {
			result.Issues = append(result.Issues, *issue)
			continue
		}
		result.Chunks = append(result.Chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return knowledge.ChunkLoadResult{}, fmt.Errorf("sqlite: iterating current knowledge chunks: %w", err)
	}
	return result, nil
}

// scanCurrentChunkRow decodes one chunkLoadCurrentQuery row. A non-nil
// error means a genuine scan/decode-level failure the caller should treat
// as database-wide; a non-nil *knowledge.ChunkLoadIssue means the row was
// read fine but excluded for a documented reason; otherwise the chunk is
// safe to index.
func scanCurrentChunkRow(rows *sql.Rows) (knowledge.Chunk, *knowledge.ChunkLoadIssue, error) {
	var chunk knowledge.Chunk
	var embedding []byte
	var itemUpdatedAt sql.NullTime
	var itemID, itemTopic, itemStatus, itemSource sql.NullString
	var itemCurrentUpdatedAt sql.NullTime

	err := rows.Scan(
		&chunk.ID, &chunk.Source, &chunk.Topic, &chunk.Status, &chunk.ItemID, &chunk.FilePath,
		&chunk.Heading, &chunk.Content, &embedding, &chunk.EmbeddingModel,
		&itemUpdatedAt, &chunk.CreatedAt,
		&itemID, &itemTopic, &itemStatus, &itemSource, &itemCurrentUpdatedAt,
	)
	if err != nil {
		return knowledge.Chunk{}, nil, err
	}
	chunk.ItemUpdatedAt = fromNullTime(itemUpdatedAt)

	decoded, decodeErr := decodeEmbedding(embedding)
	if decodeErr != nil {
		return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ChunkIssueMalformedEmbedding), nil
	}
	chunk.Embedding = decoded

	if !itemID.Valid {
		return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ChunkIssueMissingItem), nil
	}
	if chunk.Source != itemSource.String {
		return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ChunkIssueSourceMismatch), nil
	}
	if chunk.Topic != itemTopic.String {
		return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ChunkIssueTopicMismatch), nil
	}
	if chunk.Status != itemStatus.String {
		return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ChunkIssueStatusMismatch), nil
	}
	// imported_doc freshness is governed by ingested_files (mtime + model),
	// never by item_updated_at — every other source requires it to match
	// the Item's current UpdatedAt.
	if chunk.Source != knowledge.SourceImportedDoc {
		if chunk.ItemUpdatedAt.IsZero() || !chunk.ItemUpdatedAt.Equal(itemCurrentUpdatedAt.Time) {
			return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ChunkIssueStaleItem), nil
		}
	}

	if validateErr := knowledge.ValidateChunk(chunk); validateErr != nil {
		return knowledge.Chunk{}, chunkLoadIssue(chunk, knowledge.ReasonForValidationError(validateErr)), nil
	}

	return chunk, nil, nil
}

func chunkLoadIssue(chunk knowledge.Chunk, reason string) *knowledge.ChunkLoadIssue {
	return &knowledge.ChunkLoadIssue{
		ChunkID:  chunk.ID,
		ItemID:   chunk.ItemID,
		Source:   chunk.Source,
		FilePath: chunk.FilePath,
		Reason:   reason,
	}
}

// scanIDs reads a single "id" column from every row of a RETURNING query
// result and closes rows before returning.
func scanIDs(rows *sql.Rows) ([]string, error) {
	defer func() { _ = rows.Close() }()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
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

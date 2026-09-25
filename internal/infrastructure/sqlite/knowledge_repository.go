package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// KnowledgeRepository is the SQLite-backed implementation of
// knowledge.Repository.
type KnowledgeRepository struct {
	db *sql.DB
}

// NewKnowledgeRepository creates a KnowledgeRepository backed by db. db
// must already have its migrations applied (see Open).
func NewKnowledgeRepository(db *sql.DB) *KnowledgeRepository {
	return &KnowledgeRepository{db: db}
}

const knowledgeItemColumns = `id, session_id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at`

// knowledgeItemSelectColumns reads the three JSON-array-as-TEXT columns
// through COALESCE so a NULL value (e.g. a pre-existing row from before
// these columns existed) decodes as "" rather than failing the Scan into
// a plain string. unmarshalStringList treats "" as an empty list.
const knowledgeItemSelectColumns = `id, session_id, topic, concept, definition, COALESCE(properties, ''), COALESCE(trade_offs, ''), COALESCE(related_concepts, ''), source, status, created_at, updated_at`

// Save inserts a new knowledge item.
func (r *KnowledgeRepository) Save(ctx context.Context, item knowledge.Item) error {
	properties, tradeOffs, relatedConcepts, err := marshalItemLists(item)
	if err != nil {
		return err
	}
	_, err = execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO knowledge_items (`+knowledgeItemColumns+`, normalized_concept) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.SessionID, item.Topic, item.Concept, item.Definition,
		properties, tradeOffs, relatedConcepts,
		item.Source, item.Status, item.CreatedAt, item.UpdatedAt,
		knowledge.NormalizeConcept(item.Concept),
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving knowledge item: %w", err)
	}
	return nil
}

// marshalItemLists encodes item's three JSON-array-as-TEXT columns
// together so Save and Update share one error-handling path.
func marshalItemLists(item knowledge.Item) (properties, tradeOffs, relatedConcepts string, err error) {
	properties, err = marshalStringList(item.Properties)
	if err != nil {
		return "", "", "", fmt.Errorf("sqlite: encoding properties for item %s: %w", item.ID, err)
	}
	tradeOffs, err = marshalStringList(item.TradeOffs)
	if err != nil {
		return "", "", "", fmt.Errorf("sqlite: encoding trade_offs for item %s: %w", item.ID, err)
	}
	relatedConcepts, err = marshalStringList(item.RelatedConcepts)
	if err != nil {
		return "", "", "", fmt.Errorf("sqlite: encoding related_concepts for item %s: %w", item.ID, err)
	}
	return properties, tradeOffs, relatedConcepts, nil
}

// GetByID returns the item with the given id, or knowledge.ErrItemNotFound
// if it does not exist.
func (r *KnowledgeRepository) GetByID(ctx context.Context, id string) (knowledge.Item, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+knowledgeItemSelectColumns+` FROM knowledge_items WHERE id = ?`, id,
	)
	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return knowledge.Item{}, knowledge.ErrItemNotFound
	}
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: getting knowledge item: %w", err)
	}
	return item, nil
}

// Update persists every field of item, or returns knowledge.ErrItemNotFound
// if it does not exist.
func (r *KnowledgeRepository) Update(ctx context.Context, item knowledge.Item) error {
	properties, tradeOffs, relatedConcepts, err := marshalItemLists(item)
	if err != nil {
		return err
	}
	result, err := execer(ctx, r.db).ExecContext(ctx,
		`UPDATE knowledge_items SET topic = ?, concept = ?, definition = ?, properties = ?, trade_offs = ?, related_concepts = ?, source = ?, status = ?, created_at = ?, updated_at = ?, normalized_concept = ? WHERE id = ?`,
		item.Topic, item.Concept, item.Definition,
		properties, tradeOffs, relatedConcepts,
		item.Source, item.Status, item.CreatedAt, item.UpdatedAt,
		knowledge.NormalizeConcept(item.Concept), item.ID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: updating knowledge item: %w", err)
	}
	return requireRowAffected(result, knowledge.ErrItemNotFound)
}

// Delete permanently removes the item with the given id, or returns
// knowledge.ErrItemNotFound if it does not exist.
func (r *KnowledgeRepository) Delete(ctx context.Context, id string) error {
	result, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM knowledge_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting knowledge item: %w", err)
	}
	return requireRowAffected(result, knowledge.ErrItemNotFound)
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(scanner rowScanner) (knowledge.Item, error) {
	var item knowledge.Item
	var properties, tradeOffs, relatedConcepts string
	err := scanner.Scan(
		&item.ID, &item.SessionID, &item.Topic, &item.Concept, &item.Definition,
		&properties, &tradeOffs, &relatedConcepts,
		&item.Source, &item.Status, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return knowledge.Item{}, err
	}

	item.Properties, err = unmarshalStringList(properties)
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: decoding properties for item %s: %w", item.ID, err)
	}
	item.TradeOffs, err = unmarshalStringList(tradeOffs)
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: decoding trade_offs for item %s: %w", item.ID, err)
	}
	item.RelatedConcepts, err = unmarshalStringList(relatedConcepts)
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: decoding related_concepts for item %s: %w", item.ID, err)
	}
	return item, nil
}
